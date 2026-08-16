package authorization

import (
	"fmt"
	"net/http"
)

const (
	headerLocation = "Location"
)

const (
	authorizationUrl = "https://account.withings.com/oauth2_user/authorize2?response_type=code&client_id=%v&scope=%v&redirect_uri=%v&state=%v"

	scope = "user.info,user.metrics,user.activity"
	authRedirectPath = "/get_token"
	state = "NONE" // todo: this should be generated at random
)

type authorization struct {
	serviceUrl string
	clientId string
}

func New(serviceUrl, clientId string) *authorization {
	return &authorization{
		serviceUrl: serviceUrl,
		clientId: clientId,
	}
}

func (a *authorization) LoginHandler(w http.ResponseWriter, r *http.Request) {
	authRedirectUri := fmt.Sprint(a.serviceUrl, authRedirectPath)
	withlingsRedirectUrl := fmt.Sprintf(authorizationUrl, a.clientId, scope, authRedirectUri, state)
	w.Header().Add(headerLocation, withlingsRedirectUrl)

	w.WriteHeader(http.StatusFound)
}