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
	"net/http"
	"time"
)

const (
	DefaultAPIURL         = "https://music.youtube.com/youtubei/v1/"
	DefaultNonMusicAPIURL = "https://www.youtube.com/youtubei/v1/"
)

var (
	ErrLoginRequired        = errors.New("login required")
	ErrContentNotAvailable  = errors.New("content not available")
	ErrStreamNotFound       = errors.New("stream not found")
)

// Client manages connection parameters to YouTube Music.
type Client struct {
	httpClient  *http.Client
	apiURL      string
	nonMusicURL string
	visitorID   string
	auth        *AuthState
	cache       *ItemCache
	hl          string
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
