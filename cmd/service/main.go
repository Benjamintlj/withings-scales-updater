package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/Benjamintlj/withings-scales-updater/authorization"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	slog.Info("starting")

	port := flag.String("port", "8080", "port the service will run on")
	serviceUrl := flag.String("service-url", "https://raspberrypi5.tailbe1abe.ts.net", "the url the service is exposed on")
	flag.Parse()

	clientId := os.Getenv("CLIENT_ID")

	au := authorization.New(*serviceUrl, clientId)

	http.HandleFunc("/login", au.LoginHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", *port), nil))
}
