// Package oauthhelper provide an OAuth2 authentication and token management helper
package oauthhelper

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

const (
	// loopbackHost is the address the temporary authorization server listens on.
	// Port 0 lets the OS pick any free port.
	loopbackHost = "127.0.0.1:0"
	// stateLength is the number of random bytes used for the OAuth2 state value.
	stateLength = 16
	// serverReadHeaderTimeout protects the temporary authorization server.
	serverReadHeaderTimeout = 10 * time.Second
)

// Sentinel errors returned by the loopback authorization flow.
var (
	// ErrAuthorizationDenied is returned when the user denies the authorization request.
	ErrAuthorizationDenied = errors.New("authorization request was denied")
	// ErrStateMismatch is returned when the state returned by the callback doesn't match.
	ErrStateMismatch = errors.New("oauth2 state mismatch")
	// ErrMissingCode is returned when the authorization callback doesn't contain a code.
	ErrMissingCode = errors.New("no authorization code in callback")
)

// AuthenticateFunc defines the signature of the authentication function used
type AuthenticateFunc func(url string) (code string, err error)

// Auth defines the authentication parameters
type Auth struct {
	// Token holds the token that should be used for authentication (optional).
	// If the token is nil the loopback authorization flow is started and after
	// authorization this token will be set. Store (and restore prior use) this
	// token to avoid further authorization calls.
	Token *oauth2.Token
	// ClientID  from https://console.developers.google.com/project/<your-project-id>/apiui/credential
	ClientID     string
	ClientSecret string
	// Authenticate is an optional callback that, when set, overrides the default
	// loopback flow. It receives the authorization URL and must return the
	// authorization code. Leave it nil to capture the code automatically through a
	// temporary loopback web server (recommended).
	Authenticate AuthenticateFunc
	// OpenURL is an optional callback used to present the authorization URL to the
	// user (for example by opening a browser). When nil the URL is printed to the
	// standard output. It is only used by the default loopback flow.
	OpenURL func(url string) error
}

// codeResult carries the outcome of the loopback authorization callback.
type codeResult struct {
	code string
	err  error
}

// NewHTTPClient instantiates a new authentication client
func (auth *Auth) NewHTTPClient(ctx context.Context, scopes ...string) (*http.Client, error) {
	// If no scope has been specified, it shall only be the drive API one
	if len(scopes) == 0 {
		scopes = []string{"https://www.googleapis.com/auth/drive"}
	}

	config := &oauth2.Config{
		Scopes: scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
	}

	if auth.Token == nil {
		var err error

		auth.Token, err = auth.getTokenFromWeb(ctx, config)
		if err != nil {
			return nil, err
		}
	}

	return config.Client(ctx, auth.Token), nil
}

func (auth *Auth) getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	// When no custom Authenticate callback is provided, capture the authorization
	// code automatically through a temporary loopback server. Google deprecated the
	// out-of-band "urn:ietf:wg:oauth:2.0:oob" redirect that was previously used.
	if auth.Authenticate == nil {
		return auth.getTokenViaLocalServer(ctx, config)
	}

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	code, err := auth.Authenticate(authURL)
	if err != nil {
		return nil, fmt.Errorf("authenticate error: %w", err)
	}

	tok, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return tok, nil
}

// getTokenViaLocalServer runs the OAuth2 "loopback" flow: it starts a temporary
// HTTP server on a loopback address, uses it as the redirect URI, presents the
// authorization URL to the user and waits for Google to redirect back with the
// authorization code.
func (auth *Auth) getTokenViaLocalServer(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", loopbackHost)
	if err != nil {
		return nil, fmt.Errorf("couldn't start local authorization server: %w", err)
	}

	config.RedirectURL = fmt.Sprintf("http://%s/", listener.Addr().String())

	state, err := randomState()
	if err != nil {
		_ = listener.Close()

		return nil, err
	}

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	if err = auth.presentURL(authURL); err != nil {
		_ = listener.Close()

		return nil, err
	}

	code, err := waitForCode(ctx, listener, state)
	if err != nil {
		return nil, err
	}

	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}

	return token, nil
}

// presentURL shows the authorization URL to the user.
func (auth *Auth) presentURL(authURL string) error {
	if auth.OpenURL != nil {
		if err := auth.OpenURL(authURL); err != nil {
			return fmt.Errorf("couldn't open authorization URL: %w", err)
		}

		return nil
	}

	if _, err := fmt.Fprintf(
		os.Stdout,
		"Open the following URL in your browser to authorize access:\n\n%s\n\n",
		authURL,
	); err != nil {
		return fmt.Errorf("couldn't display authorization URL: %w", err)
	}

	return nil
}

// waitForCode serves a single OAuth2 callback on the listener and returns the
// captured authorization code.
func waitForCode(ctx context.Context, listener net.Listener, state string) (string, error) {
	results := make(chan codeResult, 1)

	server := &http.Server{
		Handler:           callbackHandler(state, results),
		ReadHeaderTimeout: serverReadHeaderTimeout,
	}

	defer func() { _ = server.Close() }()

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			results <- codeResult{err: fmt.Errorf("local authorization server failed: %w", err)}
		}
	}()

	select {
	case res := <-results:
		return res.code, res.err
	case <-ctx.Done():
		return "", fmt.Errorf("authorization canceled: %w", ctx.Err())
	}
}

// callbackHandler builds the HTTP handler that captures the authorization code.
func callbackHandler(state string, results chan<- codeResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()

		switch {
		case query.Get("error") != "":
			writeBrowserMessage(writer, "Authorization failed. You can close this window.")
			results <- codeResult{err: fmt.Errorf("%w: %s", ErrAuthorizationDenied, query.Get("error"))}
		case query.Get("state") != state:
			writeBrowserMessage(writer, "Invalid authorization state. You can close this window.")
			results <- codeResult{err: ErrStateMismatch}
		case query.Get("code") == "":
			writeBrowserMessage(writer, "Missing authorization code. You can close this window.")
			results <- codeResult{err: ErrMissingCode}
		default:
			writeBrowserMessage(writer, "Authorization successful. You can close this window and return to the application.")
			results <- codeResult{code: query.Get("code")}
		}
	})

	return mux
}

// writeBrowserMessage writes a short message back to the user's browser.
func writeBrowserMessage(writer http.ResponseWriter, message string) {
	_, _ = io.WriteString(writer, message+"\n")
}

// randomState returns a random, URL-safe OAuth2 state value.
func randomState() (string, error) {
	buf := make([]byte, stateLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("couldn't generate oauth2 state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// LoadTokenFromFile loads an OAuth2 token from a JSON file
func LoadTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return nil, fmt.Errorf("couldn't open token file: %w", err)
	}

	var token oauth2.Token
	err = json.NewDecoder(f).Decode(&token)

	if errClose := f.Close(); errClose != nil {
		return nil, fmt.Errorf("couldn't close token file: %w", errClose)
	}

	return &token, err
}

// StoreTokenToFile stores an OAuth2 token to a JSON file
func StoreTokenToFile(file string, token *oauth2.Token) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("couldn't open token file: %w", err)
	}

	err = json.NewEncoder(f).Encode(token)

	if errClose := f.Close(); errClose != nil {
		return fmt.Errorf("couldn't close token file: %w", errClose)
	}

	return err
}

// GetTokenBase64 returns the Base64 representation of JSON token
func GetTokenBase64(token *oauth2.Token) (string, error) {
	jb, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(jb), nil
}
