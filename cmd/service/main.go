package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Benjamintlj/withings-scales-updater/authorization"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	slog.Info("starting")

	port := flag.String("port", "8080", "port the service will run on")
	serviceUrl := flag.String("service-url", "https://raspberrypi5.tailbe1abe.ts.net", "the url the service is exposed on")
	accountWithlingsUrl := flag.String("account-withlings-url", "https://account.withings.com", "withlings account url")
	flag.Parse()

	clientId := os.Getenv("CLIENT_ID")
	customerSecret := os.Getenv("CUSTOMER_SECRET")

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	au := authorization.New(*serviceUrl, clientId, customerSecret, *accountWithlingsUrl, client)

	http.HandleFunc("/login", au.LoginHandler)
	http.HandleFunc("/get_token", au.TokenHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", *port), nil))
}
