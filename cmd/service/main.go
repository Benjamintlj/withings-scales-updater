package main

import (
	"log"
	"net/http"

	"github.com/Benjamintlj/withings-scales-updater/authorization"
)

func main() {
	http.HandleFunc("/", authorization.Handler)

	log.Println("starting ...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
