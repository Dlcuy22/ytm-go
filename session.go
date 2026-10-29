// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//
//	Decide whether a response was produced for a signed-in viewer.
//
// Key Components:
//   - authState: the verdict a response body carries
//   - classifyAuthState: reads the two markers a body can hold
//   - Client.SignedIn: the verdict of the most recent response
//   - discardHandler: the no-op logger used when none was installed
//
// Dependencies:
//   - bytes
//   - context
//   - log/slog
//   - regexp
//
// Error Types:
//   - None
package ytm

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
)

// authState is what a response body says about the session.
type authState int

const (
	// authStateUnknown means the body carried no viewer marker at all. It is the
	// state of endpoints that never echo the session, such as visitor_id.
	authStateUnknown authState = iota
	authStateSignedIn
	authStateSignedOut
)

// loggedInMarker matches the GFEEDBACK tracking parameter. The key and value are
// separate JSON fields, so the pattern tolerates whitespace between them.
var loggedInMarker = regexp.MustCompile(`"logged_in"\s*,\s*"value"\s*:\s*"(1|0)"`)

// classifyAuthState reads the viewer markers out of a raw response body.
//
// The tracking parameter wins when present because it states the answer
// directly. Only a body without it falls back to looking for a sign-in endpoint,
// which is what a signed-out page offers instead of account data.
func classifyAuthState(body []byte) authState {
	if m := loggedInMarker.FindSubmatch(body); m != nil {
		if len(m[1]) == 1 && m[1][0] == '1' {
			return authStateSignedIn
		}
		return authStateSignedOut
	}
	if bytes.Contains(body, []byte("signInEndpoint")) {
		return authStateSignedOut
	}
	return authStateUnknown
}

// authObserver is implemented by response types that keep the verdict, so the
// dispatcher can hand it over without decoding the body twice.
type authObserver interface {
	setAuthState(authState)
}

// SignedIn reports whether the most recent response was produced for a signed-in
// account.
//
// It answers true when nothing is known yet. An endpoint that does not echo the
// viewer state must not be mistaken for a dead session, so only a body that
// positively reports logged_in=0, or that offers a sign-in endpoint, flips this
// to false.
func (c *Client) SignedIn() bool {
	return c == nil || c.lastAuthState != authStateSignedOut
}

// AuthKnown reports whether any response so far carried a viewer marker. It
// separates "signed out" from "no opinion yet", which SignedIn alone cannot.
func (c *Client) AuthKnown() bool {
	return c != nil && c.lastAuthState != authStateUnknown
}

// requireSignedIn is the guard the library readers use after a successful call.
// It turns the silent empty library into an error the caller can act on.
func (c *Client) requireSignedIn() error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}
	if !c.SignedIn() {
		return ErrSessionExpired
	}
	return nil
}

// discardHandler is a slog handler that drops every record. It lets the library
// log unconditionally while staying silent unless the caller opted in.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (d discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return d }
func (d discardHandler) WithGroup(string) slog.Handler           { return d }

// logPanicWith records a recovered parse panic through the given logger, falling
// back to the package logger. It is the form the free parse helpers use, since
// they have no Client to read a logger from.
func logPanicWith(l *slog.Logger, op string, recovered any) {
	if l == nil {
		l = packageLogger
	}
	if l == nil {
		return
	}
	l.Error("ytm: recovered parse panic", "op", op, "panic", recovered)
}
