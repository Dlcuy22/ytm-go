// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define the main Client struct, defaults, and configuration setters.
//
// Key Components:
//   - Client: Central orchestrator containing the HTTP client, API routes, authentication context, and item caches
//   - NewClient / NewClientWithHTTP: Client factory constructors
//   - SetAuth / SetVisitorID: configuration modifiers
//
// Dependencies:
//   - net/http
//   - time
//
// Error Types:
//   - ErrLoginRequired: returned when an authenticated action is attempted without credentials
//   - ErrContentNotAvailable: returned when paid or premium streaming tracks are encountered
//   - ErrStreamNotFound: returned when no stream urls are found
//
package ytm

import (
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
	DefaultAPIURL         = "https://music.youtube.com/youtubei/v1/"
	DefaultNonMusicAPIURL = "https://www.youtube.com/youtubei/v1/"
)

var (
	ErrLoginRequired       = errors.New("login required")
	ErrContentNotAvailable = errors.New("content not available")
	ErrStreamNotFound      = errors.New("stream not found")

	// ErrSessionExpired means credentials were supplied but the server treated
	// the request as signed out. It is a different failure from ErrLoginRequired:
	// the caller did configure a session, the session is just no longer valid.
	//
	// The distinction matters because a dead session is not an HTTP error. The
	// server answers 200 with a signed-out page, so this error is raised by
	// inspecting the response body, not the status code.
	ErrSessionExpired = errors.New("session expired")

	// packageLogger is the fallback for the parse helpers that have no client to
	// read a logger from. It is nil until SetLogger is called.
	packageLogger *slog.Logger
)

// SetLogger installs a package-level logger for helpers that cannot reach a
// Client. Callers that do have a client should prefer Client.SetLogger.
func SetLogger(l *slog.Logger) {
	packageLogger = l
}

// Client manages connection parameters to YouTube Music.
type Client struct {
	httpClient  *http.Client
	apiURL      string
	nonMusicURL string
	visitorID   string
	auth        *AuthState
	cache       *ItemCache
	hl          string
	logger      *slog.Logger

	// lastAuthState is what the most recent response body said about the
	// session. It is only meaningful for endpoints that echo the viewer state,
	// and it stays unknown until such a response arrives.
	lastAuthState authState
}

// SetLogger installs a logger for this client. A nil logger is ignored, so a
// caller can pass a value that may be nil without checking.
func (c *Client) SetLogger(l *slog.Logger) {
	if l != nil {
		c.logger = l
	}
}

// log returns the client logger, or the package logger when the client has
// none. It never returns nil, so call sites can log unconditionally.
func (c *Client) log() *slog.Logger {
	if c != nil && c.logger != nil {
		return c.logger
	}
	if packageLogger != nil {
		return packageLogger
	}
	return slog.New(discardHandler{})
}

// logPanic records a recovered parse panic. The call is a no-op unless a logger
// was installed, because a library must not write to stderr on its own.
func (c *Client) logPanic(op string, recovered any) {
	logPanicWith(c.log(), op, recovered)
}

/*
NewClient creates a Client instance with sensible default timeouts.

    returns:
          *Client: configured client
*/
func NewClient() *Client {
	return NewClientWithHTTP(&http.Client{
		Timeout: 30 * time.Second,
	})
}

/*
NewClientWithHTTP initializes a Client with a custom http.Client.

    params:
          httpClient: custom http client
    returns:
          *Client: configured client
*/
func NewClientWithHTTP(httpClient *http.Client) *Client {
	c := &Client{
		httpClient:  httpClient,
		apiURL:      DefaultAPIURL,
		nonMusicURL: DefaultNonMusicAPIURL,
		hl:          DefaultHL,
		cache:       NewItemCache(),
	}
	return c
}

/*
SetAuth configures the authentication state on the client.

    params:
          auth: auth credential container
*/
func (c *Client) SetAuth(auth *AuthState) {
	c.auth = auth
}

/*
SetVisitorID overrides the client's visitor ID token.

    params:
          id: visitor ID token string
*/
func (c *Client) SetVisitorID(id string) {
	c.visitorID = id
}

/*
SetHL overrides the client's locale language setting.

    params:
          hl: language tag (e.g. id-ID)
*/
func (c *Client) SetHL(hl string) {
	c.hl = hl
}
