package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const issuer = "https://idp.example/pool"

// jwt builds an unsigned token with the given claims (Cognito, not this package, verifies signatures).
func jwt(claims string) string {
	return "h." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".s"
}

var goodToken = jwt(`{"iss":"` + issuer + `","client_id":"client","token_use":"access"}`)

// fakeCognito answers InitiateAuth and GetUser like the Cognito JSON API.
func fakeCognito(t *testing.T, getUserCalls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "AWSCognitoIdentityProviderService.") {
		case "InitiateAuth":
			p, _ := body["AuthParameters"].(map[string]any)
			if p["USERNAME"] == "throttled" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"TooManyRequestsException","message":"Rate exceeded"}`))
				return
			}
			if body["AuthFlow"] == "REFRESH_TOKEN_AUTH" && p["REFRESH_TOKEN"] != "refresh" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"NotAuthorizedException","message":"Invalid Refresh Token"}`))
				return
			}
			if body["AuthFlow"] == "USER_PASSWORD_AUTH" && (p["USERNAME"] != "admin" || p["PASSWORD"] != "password") {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"NotAuthorizedException","message":"Incorrect username or password"}`))
				return
			}
			_, _ = w.Write([]byte(`{"AuthenticationResult":{"AccessToken":"` + goodToken + `","RefreshToken":"refresh","ExpiresIn":3600}}`))
		case "GetUser":
			getUserCalls.Add(1)
			if body["AccessToken"] == "outage" {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"__type":"InternalErrorException","message":"boom"}`))
				return
			}
			if s, _ := body["AccessToken"].(string); s == "bad" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"NotAuthorizedException","message":"Invalid Access Token"}`))
				return
			}
			if s, _ := body["AccessToken"].(string); s == "" || s == "bad" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"__type":"NotAuthorizedException","message":"Invalid Access Token"}`))
				return
			}
			_, _ = w.Write([]byte(`{"Username":"admin"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
}

func newAPI(t *testing.T, now func() time.Time) (http.Handler, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	idp := fakeCognito(t, calls)
	t.Cleanup(idp.Close)
	a, err := New(Config{Endpoint: idp.URL, ClientID: "client", Issuer: issuer, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return a.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("api")) })), calls
}

func do(h http.Handler, method, path, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLoginReturnsTokens(t *testing.T) {
	h, _ := newAPI(t, nil)
	rec := do(h, "POST", "/auth/login", `{"username":"admin","password":"password"}`, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"accessToken":"`+goodToken+`"`) || !strings.Contains(rec.Body.String(), `"refreshToken":"refresh"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestLoginWithWrongPasswordIsUnauthorized(t *testing.T) {
	h, _ := newAPI(t, nil)
	rec := do(h, "POST", "/auth/login", `{"username":"admin","password":"nope"}`, "")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Incorrect username or password") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestRefreshReturnsAccessToken(t *testing.T) {
	h, _ := newAPI(t, nil)
	rec := do(h, "POST", "/auth/refresh", `{"refreshToken":"refresh"}`, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"accessToken":"`+goodToken+`"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestRefreshWithAnInvalidTokenIsUnauthorized(t *testing.T) {
	h, _ := newAPI(t, nil)
	if rec := do(h, "POST", "/auth/refresh", `{"refreshToken":"stale"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestIdentityProviderOutageIsNotAnAuthenticationFailure(t *testing.T) {
	h, _ := newAPI(t, nil)
	if rec := do(h, "POST", "/x", `{}`, "outage"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
}

func TestThrottlingIsNotAnAuthenticationFailure(t *testing.T) {
	h, _ := newAPI(t, nil)
	if rec := do(h, "POST", "/auth/login", `{"username":"throttled","password":"x"}`, ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
}

func TestAPIRefusesCallsWithoutAValidToken(t *testing.T) {
	h, _ := newAPI(t, nil)
	for name, bearer := range map[string]string{"no token": "", "wrong token": "bad"} {
		rec := do(h, "POST", "/tradingcopilot.v1.ExperimentService/ListExperiments", `{}`, bearer)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, rec.Code)
		}
	}
}

func TestAPIAcceptsAValidTokenAndAsksCognitoOncePerMinute(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	h, calls := newAPI(t, func() time.Time { return now })
	for i := 0; i < 3; i++ {
		if rec := do(h, "POST", "/x", `{}`, goodToken); rec.Code != http.StatusOK || rec.Body.String() != "api" {
			t.Fatalf("got %d %s", rec.Code, rec.Body)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("GetUser called %d times, want 1", calls.Load())
	}
	now = now.Add(validFor + time.Second)
	do(h, "POST", "/x", `{}`, goodToken)
	if calls.Load() != 2 {
		t.Fatalf("GetUser called %d times after the trust window, want 2", calls.Load())
	}
}

func TestAPIRefusesTokensOfAnotherPoolOrClient(t *testing.T) {
	h, _ := newAPI(t, nil)
	for name, token := range map[string]string{
		"other pool":   jwt(`{"iss":"https://idp.example/other","client_id":"client","token_use":"access"}`),
		"other client": jwt(`{"iss":"` + issuer + `","client_id":"other","token_use":"access"}`),
		"id token":     jwt(`{"iss":"` + issuer + `","aud":"client","token_use":"id"}`),
		"not a jwt":    "good",
	} {
		if rec := do(h, "POST", "/x", `{}`, token); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, rec.Code)
		}
	}
}

func TestNewRequiresConfiguration(t *testing.T) {
	if _, err := New(Config{ClientID: "c", Issuer: "i"}); err == nil {
		t.Fatal("want an error without an endpoint")
	}
	if _, err := New(Config{Endpoint: "http://x", Issuer: "i"}); err == nil {
		t.Fatal("want an error without a client id")
	}
	if _, err := New(Config{Endpoint: "http://x", ClientID: "c"}); err == nil {
		t.Fatal("want an error without an issuer")
	}
}
