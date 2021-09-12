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
			var successWorkers int32
			var time time.Duration
			results := make([]testItem, len(arTreads))
			for i, count := range arTreads {
				successWorkers, time = TestHost(item.Url, count, requestCount, timeout)
				results[i].maxThreads = int32(count)
				results[i].resultThreads = successWorkers
				results[i].timeSpent = time
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
	return
}

//Тестовая сессия для хоста
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
						return
					default:
						respItem, err := cl.Get(url)
						if err == nil && respItem.StatusCode == http.StatusOK {
							i++
							out <- true
						} else {
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
	//0 потоков быть не может, поэтому минимум 1
	if successWorkers == 0 {
		successWorkers = 1
	}
	return successWorkers, end
}
