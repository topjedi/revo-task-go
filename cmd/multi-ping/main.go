package main

import (
	"fmt"
	"log"
	"net/http"
	"revo-task-go/cmd/multi-ping/app"
)

func main() {
	http.HandleFunc("/sites/", app.Handler)
	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hello world")
}
