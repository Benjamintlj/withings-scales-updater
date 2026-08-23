package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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
		measureUrl:      "https://wbsapi.withings.net/measure",
		tokenMu:         sync.RWMutex{},
	}

	http.HandleFunc("/login", auth.LoginHandler)
	http.HandleFunc("/get_token", auth.TokenHandler)
	http.HandleFunc("GET /weight", auth.GetWeight)

	serverFailureChan := make(chan error, 1)
	go func() {
		serverFailureChan <- http.ListenAndServe(fmt.Sprintf(":%v", *port), nil)
	}()

	log.Fatal(<-serverFailureChan)
}

type ErrScalesUpdater string

func (e ErrScalesUpdater) Error() string {
	return string(e)
}

const (
	ErrFailedToCreateRequest = ErrScalesUpdater("Failed to create request")
	ErrRequestFailed         = ErrScalesUpdater("Request failed")
	ErrDecodePayload         = ErrScalesUpdater("Failed to decode payload")
)

type authorization struct {
	serviceUrl      string
	clientId        string
	customerSecret  string
	client          *http.Client
	tokenMu         sync.RWMutex
	accessToken     string
	refreshToken    string
	requestTokenUrl string
	measureUrl      string
	tokenExpiryDate time.Time
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

type responseTokenPayloadBody struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type responseTokenPayload struct {
	Body responseTokenPayloadBody `json:"body"`
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

	var responsePayload responseTokenPayload
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

	a.tokenMu.Lock()
	a.accessToken = responsePayload.Body.AccessToken
	a.refreshToken = responsePayload.Body.RefreshToken
	a.tokenExpiryDate = time.Now().UTC().Add(time.Second * time.Duration(responsePayload.Body.ExpiresIn))
	a.tokenMu.Unlock()
}

type getWeightResponse struct {
	Status int                   `json:"status"`
	Body   getWeightResponseBody `json:"body"`
}

type getWeightResponseBody struct {
	MeasureGroups []measureGroup `json:"measuregrps"`
}

type measureGroup struct {
	Measures []weightMeasure `json:"measures"`
}

type weightMeasure struct {
	Value float64 `json:"value"`
	Unit  int     `json:"unit"`
	Type  int     `json:"type"`
}

type metrics struct {
	Weight float64 `json:"weight"`
}

type weightResult struct {
	Measurements []metrics `json:"measurements"`
}

func (a *authorization) GetWeight(w http.ResponseWriter, r *http.Request) {
	err := a.RefreshTokens()
	if err != nil {
		slog.Error("failed to refresh tokens", "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	form := url.Values{}
	form.Set("action", "getmeas")
	form.Set("meastype", "1")
	form.Set("category", "1")
	form.Set("lastupdate", fmt.Sprint(time.Now().Add(-time.Hour*26).Unix()))
	form.Set("offset", "1")

	request, err := http.NewRequest(http.MethodPost, a.measureUrl, strings.NewReader(form.Encode()))
	if err != nil {
		slog.Error("failed to create body", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", a.getBearerToken())

	response, err := a.client.Do(request)
	if err != nil {
		slog.Error("request failed", "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	var responseBody getWeightResponse
	err = json.NewDecoder(response.Body).Decode(&responseBody)
	if err != nil {
		slog.Error("failed to decode response", "error", err.Error())
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	if responseBody.Status != 0 {
		slog.Warn("received a non-0 status", "status", responseBody.Status)
	}

	if len(responseBody.Body.MeasureGroups) == 0 {
		slog.Warn("no measure groups")
		w.WriteHeader(http.StatusOK)
		return
	}

	resultBody := weightResult{
		Measurements: []metrics{},
	}

	for _, measureGroup := range responseBody.Body.MeasureGroups {
		metric := metrics{}

		for _, measure := range measureGroup.Measures {
			if measure.Type == 1 {
				metric.Weight = measure.Value * math.Pow10(measure.Unit)
			}
		}
		resultBody.Measurements = append(resultBody.Measurements, metric)
	}

	slog.Info("got measurements", "length", len(resultBody.Measurements))
	json.NewEncoder(w).Encode(resultBody)
	w.WriteHeader(http.StatusOK)
}

func (a *authorization) RefreshTokens() error {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	if a.tokenExpiryDate.Add(time.Minute).After(time.Now().UTC()) {
		slog.Info("No need to refresh tokens", "expiryDate", a.tokenExpiryDate)
		return nil
	}

	form := url.Values{}
	form.Add("action", "requesttoken")
	form.Add("client_id", a.clientId)
	form.Add("client_secret", a.customerSecret)
	form.Add("grant_type", "refresh_token")
	form.Add("refresh_token", a.refreshToken)

	request, err := http.NewRequest(http.MethodPost, a.requestTokenUrl, strings.NewReader(form.Encode()))
	if err != nil {
		slog.Error("failed to create request", "error", err.Error())
		return ErrFailedToCreateRequest
	}

	response, err := a.client.Do(request)
	if err != nil {
		slog.Error("request failed", "error", err)
		return ErrRequestFailed
	}

	var responsePayload responseTokenPayload
	err = json.NewDecoder(response.Body).Decode(&responsePayload)
	if err != nil {
		slog.Error("failed to decode response payload", "error", err.Error())
		return ErrDecodePayload
	}

	a.accessToken = responsePayload.Body.AccessToken
	a.refreshToken = responsePayload.Body.RefreshToken
	a.tokenExpiryDate = time.Now().UTC().Add(time.Second * time.Duration(responsePayload.Body.ExpiresIn))
	return nil
}

func (a *authorization) getBearerToken() string {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	return fmt.Sprint("Bearer ", a.accessToken)
}
