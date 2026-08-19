package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	slog.Info("starting")

	port := flag.String("port", "8080", "port the service will run on")
	serviceUrl := flag.String("service-url", "https://raspberrypi5.tailbe1abe.ts.net", "the url the service is exposed on")
	flag.Parse()

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	auth := authorization{
		serviceUrl:      *serviceUrl,
		clientId:        os.Getenv("CLIENT_ID"),
		customerSecret:  os.Getenv("CUSTOMER_SECRET"),
		client:          client,
		requestTokenUrl: "https://wbsapi.withings.net/v2/oauth2",
	}

	http.HandleFunc("/login", auth.LoginHandler)
	http.HandleFunc("/get_token", auth.TokenHandler)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", *port), nil))
}

type authorization struct {
	serviceUrl      string
	clientId        string
	customerSecret  string
	client          *http.Client
	accessToken     string
	refreshToken    string
	requestTokenUrl string
}

func (a *authorization) LoginHandler(w http.ResponseWriter, r *http.Request) {
	authRedirectUri := fmt.Sprint(a.serviceUrl, "/get_token")
	withlingsRedirectUrl := fmt.Sprintf(
		"https://account.withings.com/oauth2_user/authorize2?response_type=code&client_id=%v&scope=%v&redirect_uri=%v&state=%v",
		a.clientId,
		"user.info,user.metrics,user.activity",
		authRedirectUri,
		"ignore")

	w.Header().Add("Location", withlingsRedirectUrl)

	w.WriteHeader(http.StatusFound)
}

type requestTokenPayloadBody struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type requestTokenPayload struct {
	Body requestTokenPayloadBody `json:"body"`
}

func (a *authorization) TokenHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("callback received")

	code := r.URL.Query().Get("code")
	if code == "" {
		slog.Error("code was empty")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// state := r.URL.Query().Get("state") // todo we will ignore this for now

	form := url.Values{}
	form.Set("action", "requesttoken")
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientId)
	form.Set("client_secret", a.customerSecret)
	form.Set("code", code)
	form.Set("redirect_uri", fmt.Sprint(a.serviceUrl, "/get_token"))

	request, err := http.NewRequest(http.MethodPost, a.requestTokenUrl, strings.NewReader(form.Encode()))
	if err != nil {
		slog.Error("failed to create body", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := a.client.Do(request)
	var urlErr *url.Error
	if errors.Is(err, urlErr) && urlErr.Timeout() {
		slog.Error("request timed out", "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	} else if err != nil {
		slog.Error("request failed", "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	if response.StatusCode >= 300 {
		slog.Error("recieved non-200 status code", "status code", response.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	var responsePayload requestTokenPayload
	err = json.NewDecoder(response.Body).Decode(&responsePayload)
	if err != nil {
		slog.Error("failed to decode response", "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	if responsePayload.Body.AccessToken == "" {
		slog.Error("returned access token was empty")
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	if responsePayload.Body.RefreshToken == "" {
		slog.Error("returned access token was empty")
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	a.accessToken = responsePayload.Body.AccessToken
	a.refreshToken = responsePayload.Body.RefreshToken
}
