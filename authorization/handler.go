package authorization

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

const (
	headerLocation = "Location"
)

const (
	authorizationUrl       = "https://account.withings.com/oauth2_user/authorize2?response_type=code&client_id=%v&scope=%v&redirect_uri=%v&state=%v"
	accessTokenExchangeUrl = "https://wbsapi.withings.net/v2/oauth2"

	scope            = "user.info,user.metrics,user.activity"
	authRedirectPath = "/get_token"
	state            = "NONE" // todo: this should be generated at random
)

type authorization struct {
	serviceUrl          string
	clientId            string
	customerSecret      string
	accountWithlingsUrl string
	client              *http.Client
}

func New(serviceUrl, clientId, customerSecret, accountWithlingsUrl string, client *http.Client) *authorization {
	return &authorization{
		serviceUrl:          serviceUrl,
		clientId:            clientId,
		customerSecret:      customerSecret,
		accountWithlingsUrl: accountWithlingsUrl,
		client:              client,
	}
}

func (a *authorization) LoginHandler(w http.ResponseWriter, r *http.Request) {
	authRedirectUri := fmt.Sprint(a.serviceUrl, authRedirectPath)
	withlingsRedirectUrl := fmt.Sprintf(authorizationUrl, a.clientId, scope, authRedirectUri, state)
	w.Header().Add(headerLocation, withlingsRedirectUrl)

	w.WriteHeader(http.StatusFound)
}

type TokenPayload struct {
	GrantType    string `json:"grant_type"`
	ClientId     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectUri  string `json:"redirect_uri"`
}

type AccessTokens struct {
	Body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	} `json:"body"`
}

func (a *authorization) TokenHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	// state := r.URL.Query().Get("state") // todo we will ignore this for now

	form := url.Values{}
	form.Set("action", "requesttoken")
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientId)
	form.Set("client_secret", a.customerSecret)
	form.Set("code", code)
	form.Set("redirect_uri", fmt.Sprint(a.serviceUrl, authRedirectPath))

	request, err := http.NewRequest(http.MethodPost, accessTokenExchangeUrl, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		slog.Error("failed to create body", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response, err := a.client.Do(request)
	if err != nil {
		slog.Error("request failed", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var responsePayload AccessTokens
	err = json.NewDecoder(response.Body).Decode(&responsePayload)
	fmt.Println(responsePayload)
}
