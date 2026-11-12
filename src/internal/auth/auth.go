// Package auth puts Amazon Cognito sign-in in front of the Experiment API. It is the same code on Floci and on AWS:
// the API Lambda serves POST /auth/login and POST /auth/refresh (user name and password in, tokens out) and refuses
// every other call that does not carry a valid access token. A token is valid when Cognito's GetUser accepts it, so
// signature, expiry and revocation are Cognito's decision, and the Lambda needs only the Cognito API endpoint (no JWKS).
package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// validFor is how long a token that Cognito accepted is trusted without asking again.
	validFor = time.Minute
	// maxBody bounds the sign-in request bodies.
	maxBody = 4 << 10
)

// Config is the Cognito app client and the API endpoint of the identity provider.
type Config struct {
	// Endpoint is the Cognito API URL: https://cognito-idp.<region>.amazonaws.com/ on AWS, Floci's URL locally.
	Endpoint string
	ClientID string
	// Issuer is the user pool's issuer URL (the token claim "iss"). GetUser accepts an access token from any user pool
	// of the region, so the claims are compared with the configured pool and app client.
	Issuer string
	// HTTP is the client for Cognito calls (default: 5 s timeout).
	HTTP *http.Client
	// Now is the clock (default time.Now); tests set it.
	Now func() time.Time
}

// Authenticator signs users in and checks access tokens.
type Authenticator struct {
	cfg Config

	mu    sync.Mutex
	valid map[[32]byte]time.Time // sha256(token) -> trusted until
}

// New returns an Authenticator, or an error when the configuration is incomplete (the API then must not start).
func New(cfg Config) (*Authenticator, error) {
	if cfg.Endpoint == "" || cfg.ClientID == "" || cfg.Issuer == "" {
		return nil, errors.New("auth: the Cognito endpoint, client id and issuer are required")
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: 5 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Authenticator{cfg: cfg, valid: map[[32]byte]time.Time{}}, nil
}

// Wrap serves the sign-in endpoints itself and passes every other request to next only with a valid bearer token.
func (a *Authenticator) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/auth/login":
			a.login(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/auth/refresh":
			a.refresh(w, r)
		default:
			switch a.check(r) {
			case allowed:
				next.ServeHTTP(w, r)
			case unavailable:
				// Not a 401: the web app signs the user out on a 401, and an outage says nothing about the token.
				reject(w, http.StatusServiceUnavailable, "identity provider unavailable")
			default:
				reject(w, http.StatusUnauthorized, "sign in required")
			}
		}
	})
}

type tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int    `json:"expiresIn"`
}

func (a *Authenticator) login(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if !decode(w, r, &in) || in.Username == "" || in.Password == "" {
		reject(w, http.StatusBadRequest, "username and password are required")
		return
	}
	a.initiateAuth(w, r.Context(), "USER_PASSWORD_AUTH", map[string]string{"USERNAME": in.Username, "PASSWORD": in.Password})
}

func (a *Authenticator) refresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decode(w, r, &in) || in.RefreshToken == "" {
		reject(w, http.StatusBadRequest, "refreshToken is required")
		return
	}
	a.initiateAuth(w, r.Context(), "REFRESH_TOKEN_AUTH", map[string]string{"REFRESH_TOKEN": in.RefreshToken})
}

func (a *Authenticator) initiateAuth(w http.ResponseWriter, ctx context.Context, flow string, params map[string]string) {
	var out struct {
		AuthenticationResult *struct {
			AccessToken  string
			RefreshToken string
			ExpiresIn    int
		}
		ChallengeName string
	}
	status, fault, err := a.call(ctx, "InitiateAuth", map[string]any{
		"AuthFlow": flow, "ClientId": a.cfg.ClientID, "AuthParameters": params,
	}, &out)
	switch {
	case err != nil:
		reject(w, http.StatusBadGateway, "identity provider unavailable")
	case status != http.StatusOK && fault.refused():
		// Only a refusal of the credentials or the refresh token is a 401: the web app signs the user out on it.
		reject(w, http.StatusUnauthorized, fault.Message)
	case status != http.StatusOK && fault.throttled():
		reject(w, http.StatusTooManyRequests, "too many attempts, try again later")
	case status != http.StatusOK:
		reject(w, http.StatusBadGateway, "identity provider unavailable")
	case out.AuthenticationResult == nil:
		reject(w, http.StatusUnauthorized, "sign-in needs a further step ("+out.ChallengeName+")")
	default:
		res := out.AuthenticationResult
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokens{AccessToken: res.AccessToken, RefreshToken: res.RefreshToken, ExpiresIn: res.ExpiresIn})
	}
}

type verdict int

const (
	denied verdict = iota
	allowed
	unavailable // Cognito could not answer; the token is neither accepted nor refused
)

// check decides whether the request carries an access token that Cognito accepts.
func (a *Authenticator) check(r *http.Request) verdict {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return denied
	}
	key := sha256.Sum256([]byte(token))
	now := a.cfg.Now()
	a.mu.Lock()
	until, hit := a.valid[key]
	a.mu.Unlock()
	if hit && now.Before(until) {
		return allowed
	}
	status, f, err := a.call(r.Context(), "GetUser", map[string]string{"AccessToken": token}, nil)
	switch {
	case err != nil || (status != http.StatusOK && !f.refused()):
		return unavailable
	case status != http.StatusOK || !a.ours(token):
		return denied
	}
	a.mu.Lock()
	for k, t := range a.valid { // drop expired entries so the map stays small
		if !now.Before(t) {
			delete(a.valid, k)
		}
	}
	a.valid[key] = now.Add(validFor)
	a.mu.Unlock()
	return allowed
}

// ours reports whether an access token (already accepted by Cognito) was issued by this user pool to this app client.
// The claims are read without a signature check because Cognito has just verified the token.
func (a *Authenticator) ours(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var c struct {
		Iss      string `json:"iss"`
		ClientID string `json:"client_id"`
		Use      string `json:"token_use"`
	}
	if json.Unmarshal(payload, &c) != nil {
		return false
	}
	return c.Iss == a.cfg.Issuer && c.ClientID == a.cfg.ClientID && c.Use == "access"
}

// fault is the error Cognito answered with.
type fault struct{ Type, Message string }

// refused reports a wrong user name or password, an unknown user, or an invalid or expired refresh token.
func (f fault) refused() bool {
	switch f.Type {
	case "NotAuthorizedException", "UserNotFoundException", "PasswordResetRequiredException", "UserNotConfirmedException":
		return true
	}
	return false
}

func (f fault) throttled() bool {
	return f.Type == "TooManyRequestsException" || f.Type == "LimitExceededException"
}

// call posts one Cognito JSON-protocol request. It returns the HTTP status and, for a non-200 answer, Cognito's error.
func (a *Authenticator) call(ctx context.Context, target string, body any, out any) (int, fault, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fault{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return 0, fault{}, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService."+target)
	res, err := a.cfg.HTTP.Do(req)
	if err != nil {
		return 0, fault{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, fault{}, err
	}
	if res.StatusCode != http.StatusOK {
		var e struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Message == "" {
			e.Message = "sign-in refused"
		}
		return res.StatusCode, fault{Type: e.Type, Message: e.Message}, nil
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return 0, fault{}, fmt.Errorf("auth: decode %s: %w", target, err)
		}
	}
	return res.StatusCode, fault{}, nil
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(v) == nil
}

func reject(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="dagobert"`)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"code": http.StatusText(status), "message": msg})
}
