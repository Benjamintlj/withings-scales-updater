package authorization_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Benjamintlj/withings-scales-updater/authorization"
)

func TestLoginHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	response := httptest.NewRecorder()
	au := authorization.New("http://localhost:8080", "NONE")

	au.LoginHandler(response, request)

	assertStatusCode(t, http.StatusFound, response.Code)
	const wantLocation = "https://account.withings.com/oauth2_user/authorize2?response_type=code&client_id=NONE&scope=user.info,user.metrics,user.activity&redirect_uri=http://localhost:8080/get_token&state=NONE"
	assertRedirectUrl(t, wantLocation, response.Header().Get("Location"))
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
