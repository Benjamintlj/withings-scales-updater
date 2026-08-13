package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", testingHandler)

	log.Println("starting ...")
	log.Fatal(http.ListenAndServe(":443", nil))
}

func testingHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("endpoint hit")
	w.WriteHeader(http.StatusOK)
}
