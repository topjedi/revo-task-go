package main

import (
	"log"
	"net/http"
	"revo-task-go/cmd/multi-ping/app"
)

func main() {
	http.HandleFunc("/sites/", app.Handler)
	log.Fatal(http.ListenAndServe(":80", nil))
}
