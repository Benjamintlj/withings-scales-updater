package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
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
			wantStatus:       http.StatusBadGateway,
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
					err := json.NewEncoder(w).Encode(responseTokenPayload{
						Body: responseTokenPayloadBody{
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

			if au.refreshToken != test.wantRefreshToken {
				t.Errorf("expected refreshToken %q but got %q", au.refreshToken, test.wantRefreshToken)
			}
		})
	}
}

func TestGetWeight(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/weight", nil)
		response := httptest.NewRecorder()

		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				err := json.NewEncoder(w).Encode(getWeightResponse{
					Status: 0,
					Body: getWeightResponseBody{
						[]measureGroup{
							{[]weightMeasure{{Value: 81000, Unit: -3, Type: 1}}},
							{[]weightMeasure{{Value: 82000, Unit: -3, Type: 1}}},
						},
					},
				})

				if err != nil {
					t.Fatalf("failed to encode mock response %q", err.Error())
				}
			}))

		defer server.Close()
		au := &authorization{
			tokenExpiryDate: time.Now().UTC(),
			accessToken:     "token",
			refreshToken:    "askdf",
			tokenMu:         sync.RWMutex{},
			client:          &http.Client{Timeout: 1 * time.Second},
			measureUrl:      server.URL,
		}

		au.GetWeight(response, request)

		assertStatusCode(t, http.StatusOK, response.Code)

		var responsePayload weightResult
		err := json.NewDecoder(response.Body).Decode(&responsePayload)
		if err != nil {
			t.Fatalf("Failed to decode response %q", err.Error())
		}

		wantResponse := weightResult{
			Measurements: []metrics{
				{Weight: 81},
				{Weight: 82},
			},
		}
		if !reflect.DeepEqual(responsePayload, wantResponse) {
			t.Errorf("Wanted payload %v, but got %v", wantResponse, responsePayload)
		}
	})

	t.Run("Bad response", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/weight", nil)
		response := httptest.NewRecorder()

		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
		)
		defer server.Close()
		au := &authorization{
			tokenExpiryDate: time.Now().UTC(),
			accessToken:     "token",
			refreshToken:    "askdf",
			tokenMu:         sync.RWMutex{},
			client:          &http.Client{Timeout: 1 * time.Second},
			measureUrl:      server.URL,
		}

		au.GetWeight(response, request)

		assertStatusCode(t, http.StatusBadGateway, response.Code)
	})
}

func TestRefreshTokens(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		wantAccessToken := "want access token"
		wantRefreshToken := "want refresh token"
		wantExpiry := 10800

		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				err := json.NewEncoder(w).Encode(responseTokenPayload{
					Body: responseTokenPayloadBody{
						AccessToken:  wantAccessToken,
						RefreshToken: wantRefreshToken,
						ExpiresIn:    wantExpiry,
					},
				})

				if err != nil {
					t.Fatalf("failed to encode mock response %q", err.Error())
				}
			}))
		defer server.Close()
		au := &authorization{
			tokenExpiryDate: time.Now().UTC().Add(time.Minute * -2),
			accessToken:     "token",
			refreshToken:    "askdf",
			tokenMu:         sync.RWMutex{},
			client:          &http.Client{Timeout: 1 * time.Second},
			requestTokenUrl: server.URL,
		}

		err := au.RefreshTokens()

		if err != nil {
			t.Errorf("Expected no error %q", err.Error())
		}
		if au.refreshToken != wantRefreshToken {
			t.Errorf("Expected refresh token %q but got %q", wantRefreshToken, au.refreshToken)
		}
		if au.accessToken != wantAccessToken {
			t.Errorf("Expected refresh token %q but got %q", wantAccessToken, au.accessToken)
		}
		if au.tokenExpiryDate.Before(time.Now().UTC()) {
			t.Errorf("Expected expiry date to be in the future but was not")
		}
	})

	t.Run("Don't update token if it has not expired", func(t *testing.T) {
		wantAccessToken := "want access token"
		wantRefreshToken := "want refresh token"
		wantExpiry := time.Now().UTC().Add(time.Minute * 5)

		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)

				err := json.NewEncoder(w).Encode(responseTokenPayload{
					Body: responseTokenPayloadBody{
						AccessToken:  "asdfasdf",
						RefreshToken: "asddfsa",
						ExpiresIn:    10800,
					},
				})

				if err != nil {
					t.Fatalf("failed to encode mock response %q", err.Error())
				}
			}))
		defer server.Close()
		au := &authorization{
			tokenExpiryDate: wantExpiry,
			accessToken:     wantAccessToken,
			refreshToken:    wantRefreshToken,
			tokenMu:         sync.RWMutex{},
			client:          &http.Client{Timeout: 1 * time.Second},
			requestTokenUrl: server.URL,
		}

		err := au.RefreshTokens()

		if err != nil {
			t.Errorf("Expected no error %q", err.Error())
		}
		if au.refreshToken != wantRefreshToken {
			t.Errorf("Expected refresh token %q but got %q", wantRefreshToken, au.refreshToken)
		}
		if au.accessToken != wantAccessToken {
			t.Errorf("Expected refresh token %q but got %q", wantAccessToken, au.accessToken)
		}
		if au.tokenExpiryDate.Before(time.Now().UTC()) {
			t.Errorf("Expected expiry date to be in the future but was not")
		}
	})

	t.Run("Bad server response", func(t *testing.T) {
		server := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
		defer server.Close()
		au := &authorization{
			tokenExpiryDate: time.Now().UTC().Add(time.Minute * -2),
			accessToken:     "asdf",
			refreshToken:    "asdfasdf",
			tokenMu:         sync.RWMutex{},
			client:          &http.Client{Timeout: 1 * time.Second},
			requestTokenUrl: server.URL,
		}

		err := au.RefreshTokens()

		if errors.Is(err, ErrRequestFailed) {
			t.Errorf("Expected no error %q", err.Error())
		}
	})
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
