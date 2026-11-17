package oauthhelper

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

var errUnexpected = errors.New("unexpected")

// fakeOAuthServer is a minimal OAuth2 token endpoint that enforces PKCE: each
// code it issues is bound to a code_challenge, and the token request must carry
// the matching code_verifier.
type fakeOAuthServer struct {
	server *httptest.Server

	mu         sync.Mutex
	challenges map[string]string // code -> S256 challenge
	verifiers  []string          // code_verifier values received
}

func newFakeOAuthServer(t *testing.T) *fakeOAuthServer {
	t.Helper()

	fake := &fakeOAuthServer{challenges: map[string]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", fake.handleToken)
	fake.server = httptest.NewServer(mux)
	t.Cleanup(fake.server.Close)

	return fake
}

func (f *fakeOAuthServer) endpoint() *oauth2.Endpoint {
	return &oauth2.Endpoint{
		AuthURL:   f.server.URL + "/auth",
		TokenURL:  f.server.URL + "/token",
		AuthStyle: oauth2.AuthStyleInParams,
	}
}

// issueCode registers a code as if the user had consented to an authorization
// request carrying this challenge.
func (f *fakeOAuthServer) issueCode(code, challenge string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.challenges[code] = challenge
}

func (f *fakeOAuthServer) receivedVerifiers() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.verifiers...)
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (f *fakeOAuthServer) handleToken(writer http.ResponseWriter, request *http.Request) {
	if err := request.ParseForm(); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)

		return
	}

	code := request.PostForm.Get("code")
	verifier := request.PostForm.Get("code_verifier")

	f.mu.Lock()
	f.verifiers = append(f.verifiers, verifier)
	challenge, known := f.challenges[code]
	f.mu.Unlock()

	if request.PostForm.Get("grant_type") != "authorization_code" || !known || verifier == "" ||
		s256(verifier) != challenge {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_grant"}`))

		return
	}

	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"access_token":  "access-" + code,
		"refresh_token": "refresh-" + code,
		"token_type":    "Bearer",
		"expires_in":    3600,
	})
}

// callback simulates the browser redirect to the loopback server.
func callback(redirectURI, state, code string) error {
	target := redirectURI + "?" + url.Values{"state": {state}, "code": {code}}.Encode()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	return resp.Body.Close()
}

type parsedAuthURL struct {
	state, redirectURI, challenge, method string
}

func parseAuthURL(t *testing.T, raw string) parsedAuthURL {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid authorization URL %q: %v", raw, err)
	}

	query := parsed.Query()

	return parsedAuthURL{
		state:       query.Get("state"),
		redirectURI: query.Get("redirect_uri"),
		challenge:   query.Get("code_challenge"),
		method:      query.Get("code_challenge_method"),
	}
}

func checkChallenge(t *testing.T, authURL parsedAuthURL) {
	t.Helper()

	if authURL.challenge == "" {
		t.Fatal("authorization URL has no code_challenge")
	}

	if authURL.method != "S256" {
		t.Fatalf("code_challenge_method = %q, want S256", authURL.method)
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	return ctx
}

func TestLoopbackFlowUsesPKCE(t *testing.T) {
	fake := newFakeOAuthServer(t)
	tokenFile := filepath.Join(t.TempDir(), "token.json")

	var challenge string

	auth := &Auth{
		ClientID:  "client",
		TokenFile: tokenFile,
		endpoint:  fake.endpoint(),
		OpenURL: func(raw string) error {
			authURL := parseAuthURL(t, raw)
			checkChallenge(t, authURL)
			challenge = authURL.challenge
			fake.issueCode("good-code", authURL.challenge)

			go func() { _ = callback(authURL.redirectURI, authURL.state, "good-code") }()

			return nil
		},
	}

	if _, err := auth.NewHTTPClient(testContext(t)); err != nil {
		t.Fatalf("flow failed: %v", err)
	}

	if auth.Token == nil || auth.Token.AccessToken != "access-good-code" {
		t.Fatalf("unexpected token: %+v", auth.Token)
	}

	verifiers := fake.receivedVerifiers()
	if len(verifiers) != 1 || s256(verifiers[0]) != challenge {
		t.Fatalf("token request verifiers %v don't match challenge %q", verifiers, challenge)
	}

	stored, err := LoadTokenFromFile(tokenFile)
	if err != nil || stored.AccessToken != "access-good-code" {
		t.Fatalf("token not stored: %+v, %v", stored, err)
	}
}

func TestLoopbackFlowRejectsInjectedCode(t *testing.T) {
	fake := newFakeOAuthServer(t)
	tokenFile := filepath.Join(t.TempDir(), "token.json")

	auth := &Auth{
		ClientID:  "client",
		TokenFile: tokenFile,
		endpoint:  fake.endpoint(),
		OpenURL: func(raw string) error {
			// The attacker reads the logged URL (state + redirect URI), runs their own
			// consent flow with their own verifier, and injects the resulting code.
			authURL := parseAuthURL(t, raw)
			checkChallenge(t, authURL)
			fake.issueCode("attacker-code", s256(oauth2.GenerateVerifier()))

			go func() { _ = callback(authURL.redirectURI, authURL.state, "attacker-code") }()

			return nil
		},
	}

	_, err := auth.NewHTTPClient(testContext(t))
	if err == nil {
		t.Fatal("a code issued for another challenge was accepted")
	}

	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) || retrieveErr.ErrorCode != "invalid_grant" {
		t.Fatalf("unexpected error: %v", err)
	}

	if auth.Token != nil {
		t.Fatalf("token set despite the failure: %+v", auth.Token)
	}

	if _, statErr := os.Stat(tokenFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("token file written despite the failure: %v", statErr)
	}
}

func TestAuthenticateCallbackUsesPKCE(t *testing.T) {
	fake := newFakeOAuthServer(t)

	var challenge string

	auth := &Auth{
		ClientID: "client",
		endpoint: fake.endpoint(),
		Authenticate: func(raw string) (string, error) {
			authURL := parseAuthURL(t, raw)
			if authURL.challenge == "" || authURL.method != "S256" {
				return "", fmt.Errorf("%w: no S256 challenge in %q", errUnexpected, raw)
			}

			challenge = authURL.challenge
			fake.issueCode("cb-code", authURL.challenge)

			return "cb-code", nil
		},
	}

	if _, err := auth.NewHTTPClient(testContext(t)); err != nil {
		t.Fatalf("flow failed: %v", err)
	}

	verifiers := fake.receivedVerifiers()
	if len(verifiers) != 1 || s256(verifiers[0]) != challenge {
		t.Fatalf("token request verifiers %v don't match challenge %q", verifiers, challenge)
	}
}

func TestAuthenticateCallbackRejectsForeignCode(t *testing.T) {
	fake := newFakeOAuthServer(t)

	auth := &Auth{
		ClientID: "client",
		endpoint: fake.endpoint(),
		Authenticate: func(string) (string, error) {
			fake.issueCode("foreign-code", s256(oauth2.GenerateVerifier()))

			return "foreign-code", nil
		},
	}

	if _, err := auth.NewHTTPClient(testContext(t)); err == nil {
		t.Fatal("a code issued for another challenge was accepted")
	}

	if auth.Token != nil {
		t.Fatalf("token set despite the failure: %+v", auth.Token)
	}
}
