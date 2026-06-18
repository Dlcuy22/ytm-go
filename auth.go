// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define AuthState and SAPISIDHASH authentication generation logic.
//
// Key Components:
//   - AuthState: Credentials container
//   - GenerateSAPISIDHash: SAPISID hashing algorithm
//   - NewAuthFromCookies / NewAuthFromCookieString: Constructors for AuthState
//
// Dependencies:
//   - crypto/sha1
//   - encoding/hex
//   - net/http
//
// Error Types:
//   - None
//
package ytm

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AuthState manages authenticated session credentials.
type AuthState struct {
	Cookie        string    `json:"cookie"`
	Authorization string    `json:"authorization"` // Current SAPISIDHASH
	ChannelID     string    `json:"channel_id,omitempty"`
	SAPISID       string    `json:"sapisid,omitempty"`
	LastUpdated   time.Time `json:"last_updated"`
}

/*
GenerateSAPISIDHash calculates a valid SAPISIDHASH header value.

    params:
          sapisid: the SAPISID cookie value
    returns:
          string: formatted SAPISIDHASH authorization header
*/
func GenerateSAPISIDHash(sapisid string) string {
	ts := time.Now().Unix()
	input := fmt.Sprintf("%d %s https://music.youtube.com", ts, sapisid)
	h := sha1.New()
	h.Write([]byte(input))
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("SAPISIDHASH %d_%s", ts, hash)
}

/*
NewAuthFromCookies parses cookie slice to build an AuthState.

    params:
          cookies: list of http.Cookies
    returns:
          *AuthState: authenticated session state, or error if SAPISID is missing
*/
func NewAuthFromCookies(cookies []*http.Cookie) (*AuthState, error) {
	var sapisid string
	var cookieParts []string
	for _, c := range cookies {
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", c.Name, c.Value))
		if c.Name == "__Secure-3PAPISID" || c.Name == "SAPISID" {
			sapisid = c.Value
		}
	}
	if sapisid == "" {
		return nil, fmt.Errorf("SAPISID or __Secure-3PAPISID cookie not found")
	}
	cookieStr := strings.Join(cookieParts, "; ")
	return &AuthState{
		Cookie:        cookieStr,
		SAPISID:       sapisid,
		Authorization: GenerateSAPISIDHash(sapisid),
		LastUpdated:   time.Now(),
	}, nil
}

/*
NewAuthFromCookieString parses raw cookie header string to build an AuthState.

    params:
          cookieStr: raw cookie header content
    returns:
          *AuthState: authenticated session state, or error if SAPISID is missing
*/
func NewAuthFromCookieString(cookieStr string) (*AuthState, error) {
	var sapisid string
	parts := strings.Split(cookieStr, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		subparts := strings.SplitN(part, "=", 2)
		if len(subparts) == 2 {
			name := strings.TrimSpace(subparts[0])
			val := strings.TrimSpace(subparts[1])
			if name == "__Secure-3PAPISID" || name == "SAPISID" {
				sapisid = val
			}
		}
	}
	if sapisid == "" {
		return nil, fmt.Errorf("SAPISID or __Secure-3PAPISID cookie not found in cookie string")
	}
	return &AuthState{
		Cookie:        cookieStr,
		SAPISID:       sapisid,
		Authorization: GenerateSAPISIDHash(sapisid),
		LastUpdated:   time.Now(),
	}, nil
}

/*
GetAuthorizationHeader returns a fresh authorization token if SAPISID is present, or the cached fallback.

    returns:
          string: authorization header token
*/
func (a *AuthState) GetAuthorizationHeader() string {
	if a.SAPISID == "" {
		return a.Authorization
	}
	return GenerateSAPISIDHash(a.SAPISID)
}
