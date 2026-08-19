package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()
	au := &authorization{
		serviceUrl:     "http://localhost:8080",
		clientId:       "NONE",
		customerSecret: "SECRET",
		client:         &http.Client{},
	}

	au.LoginHandler(response, request)

	assertStatusCode(t, http.StatusFound, response.Code)
	const wantLocation = "https://account.withings.com/oauth2_user/authorize2?response_type=code&client_id=NONE&scope=user.info,user.metrics,user.activity&redirect_uri=http://localhost:8080/get_token&state=ignore"
	assertRedirectUrl(t, wantLocation, response.Header().Get("Location"))
}

func TestTokenHandler(t *testing.T) {

	tests := []struct {
		name             string
		code             string
		accessToken      string
		refreshToken     string
		responseStatus   int
		wantStatus       int
		wantAccessToken  string
		wantRefreshToken string
	}{
		{
			name:             "Success",
			code:             "f9fb4ad0aed32ddd79771cf855d53dd0aec9fa8c",
			accessToken:      "11312321323123",
			refreshToken:     "123123123123123",
			responseStatus:   http.StatusOK,
			wantStatus:       http.StatusOK,
			wantAccessToken:  "11312321323123",
			wantRefreshToken: "123123123123123",
		},
		{
			name:             "No code",
			code:             "",
			accessToken:      "",
			refreshToken:     "",
			responseStatus:   http.StatusTeapot,
			wantStatus:       http.StatusBadRequest,
			wantAccessToken:  "",
			wantRefreshToken: "",
		},
		{
			name:             "Missing access token",
			code:             "f9fb4ad0aed32ddd79771cf855d53dd0aec9fa8c",
			accessToken:      "",
			refreshToken:     "123123123123123",
			responseStatus:   http.StatusOK,
			wantStatus:       http.StatusBadGateway,
			wantAccessToken:  "",
			wantRefreshToken: "",
		},
		{
			name:             "Missing refresh token",
			code:             "f9fb4ad0aed32ddd79771cf855d53dd0aec9fa8c",
			accessToken:      "123123",
			refreshToken:     "",
			responseStatus:   http.StatusOK,
			wantStatus:       http.StatusBadGateway,
			wantAccessToken:  "",
			wantRefreshToken: "",
		},
		{
			name:             "Get a bad status code",
			code:             "f9fb4ad0aed32ddd79771cf855d53dd0aec9fa8c",
			accessToken:      "11312321323123",
			refreshToken:     "123123123123123",
			responseStatus:   http.StatusInternalServerError,
			wantStatus:       http.StatusBadGateway	,
			wantAccessToken:  "",
			wantRefreshToken: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/token", nil)
			q := request.URL.Query()
			q.Add("code", test.code)
			q.Add("state", "NONE")
			request.URL.RawQuery = q.Encode()

			response := httptest.NewRecorder()

			server := httptest.NewServer(
				http.HandlerFunc(func(
					w http.ResponseWriter,
					r *http.Request,
				) {
					w.WriteHeader(test.responseStatus)
					err := json.NewEncoder(w).Encode(requestTokenPayload{
						Body: requestTokenPayloadBody{
							AccessToken:  test.accessToken,
							RefreshToken: test.refreshToken,
						},
					})
					if err != nil {
						t.Fatal(err)
					}
				}),
			)
			defer server.Close()
			au := &authorization{
				serviceUrl:      "http://localhost:8080",
				clientId:        "NONE",
				customerSecret:  "SECRET",
				client:          &http.Client{Timeout: 1 * time.Second},
				requestTokenUrl: server.URL,
			}

			au.TokenHandler(response, request)

			assertStatusCode(t, test.wantStatus, response.Code)

			if au.accessToken != test.wantAccessToken {
				t.Errorf("expected accessToken %q but got %q", au.accessToken, test.wantAccessToken)
			}

			if au.refreshToken != test.wantRefreshToken	 {
				t.Errorf("expected refreshToken %q but got %q", au.refreshToken, test.wantRefreshToken)
			}
		})
	}
}

func assertStatusCode(t testing.TB, want, got int) {
	t.Helper()

	if want != got {
		t.Errorf("expected status code %v, but got %v", want, got)
	}
}

func assertRedirectUrl(t testing.TB, want, got string) {
	t.Helper()

	if want != got {
		t.Errorf("expected location to be %q, but got %q", want, got)
	}
}
