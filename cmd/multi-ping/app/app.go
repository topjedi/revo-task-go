package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"revo-task-go/pkg/config"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type testItem struct {
	maxThreads    int32
	resultThreads int32
	timeSpent     time.Duration
}

func Handler(w http.ResponseWriter, r *http.Request) {
	response := make(map[string]int32)
	mu := &sync.Mutex{}
	//start := time.Now()
	//fmt.Fprintf(w, "r.URL.Query().Get(\"search\") = %q\n", r.URL.Query().Get("search"))
	//strSearchReq, _ := url.QueryUnescape(r.URL.Query().Get("search"))
	strSearchReq := r.URL.Query().Get("search")
	timeoutInt, err := strconv.Atoi(config.GetEnv("TIMEOUT", "5"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("Error to parse env TIMEOUT. Error: %v\n", err.Error())
		return
	}
	timeout := time.Duration(int64(timeoutInt))
	strThreads := config.GetEnv("THREADS", "5,10,20")
	arStrThreads := strings.Split(strThreads, ",")
	arTreads := make([]int, len(arStrThreads))
	for i, _ := range arStrThreads {
		arTreads[i], err = strconv.Atoi(arStrThreads[i])
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Printf("Error to parse env THREADS. Error: %v\n", err.Error())
			return
		}
	}
	requestCount, err := strconv.Atoi(config.GetEnv("REQUEST_COUNT", "100"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("Error to parse env REQUEST_COUNT. Error: %v\n", err.Error())
		return
	}
	cl := &http.Client{}
	resp, err := cl.Get(fmt.Sprintf(baseYandexURL, url.QueryEscape(strSearchReq)))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Printf("error yandex %v", err.Error())
		return
	}
	bufBody, _ := io.ReadAll(resp.Body)
	hosts := parseYandexResponse(bufBody)
	if hosts.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Printf(hosts.Error.Error())
		return
	}

	wg := &sync.WaitGroup{}
	for i, item := range hosts.Items {
		wg.Add(1)
		go func(wg *sync.WaitGroup, item responseItem, i int) {
			defer wg.Done()
			//result := ""
			var successWorkers int32
			var time time.Duration
			//result+= fmt.Sprintf(fmt.Sprintf("[%v] %v      %v\n", i, item.Host, item.Url))
			results := make([]testItem, len(arTreads))
			for i, count := range arTreads {
				successWorkers, time = TestHost(item.Url, count, requestCount, timeout)
				results[i].maxThreads = int32(count)
				results[i].resultThreads = successWorkers
				results[i].timeSpent = time
				//result+= fmt.Sprintf("max %v Success workers: %v Duration %v\n",count, successWorkers,time)
				if successWorkers == 0 {
					break
				}
			}
			finalItem := 0
			for i, item := range results {
				if i > 0 {
					if item.timeSpent < results[i-1].timeSpent && item.resultThreads >= results[i-1].maxThreads {
						finalItem = i
					}
				}
			}
			mu.Lock()
			response[item.Host] = results[finalItem].resultThreads
			mu.Unlock()
			//fmt.Fprint(w,result)
			//fmt.Fprintf(w,"Final item %v ma workers %v\n",finalItem,results[finalItem].resultThreads)
		}(wg, item, i)
	}
	wg.Wait()
	b, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(b)
	//fmt.Fprintf(w,"Results: %v\n",response)
	//allTime := time.Since(start)
	//fmt.Printf("request all %v\n", allTime)
	return
}

func TestHost(url string, maxThreads int, requestCount int, timeout time.Duration) (int32, time.Duration) {
	start := time.Now()
	var end time.Duration
	var successWorkers int32
	cl := &http.Client{
		Timeout: time.Second * timeout,
	}
	ch := make(chan bool)
	ctx, finish := context.WithCancel(context.Background())
	go func(ctx context.Context, out chan<- bool) {
		wg := &sync.WaitGroup{}
		for i := 0; i < maxThreads; i++ {
			wg.Add(1)
			go func(out chan<- bool, wg *sync.WaitGroup, workerNum int) {
				i := 0
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						atomic.AddInt32(&successWorkers, 1)
						//fmt.Printf("Good Worker [%v]    done %v success\n", workerNum, i)
						return
					default:
						respItem, err := cl.Get(url)
						if err == nil && respItem.StatusCode == http.StatusOK {
							i++
							out <- true
							//fmt.Fprintf(w,"Status %v\n", respItem.StatusCode)
						} else {
							//if err != nil{
							//	if err, ok := err.(net.Error); ok && err.Timeout() {
							//		fmt.Printf("Timeout error\n")
							//	}else {
							//		fmt.Printf("Error request %v\n", err.Error())
							//	}
							//} else {
							//	fmt.Printf("Status %v\n", respItem.Status)
							//}

							//fmt.Printf("Worker [%v]    done %v success\n", workerNum, i)
							out <- false
							return
						}
					}
				}
			}(out, wg, i)
		}
		wg.Wait()
		close(out)
	}(ctx, ch)

	var success, fail, needToEnd int
	needToEnd = requestCount
	for result := range ch {
		if result == true {
			success++
			if success >= needToEnd {
				//Останавливаем таймер по достижении нужного количества ответов, запросы в ожидании не учитываем
				end = time.Since(start)
				finish()
			}
		} else {
			fail++
		}
	}
	//if success < needToEnd{
	//	fmt.Printf("FAILED ALL treads\n")
	//}else{
	//	fmt.Printf("Success workers: %v\n",successWorkers)
	//}
	//fmt.Printf("Success %v\n FAIL %v\n", success,fail)
	//0 потоков быть не может, поэтому минимум 1
	if successWorkers == 0 {
		successWorkers = 1
	}
	return successWorkers, end
}
