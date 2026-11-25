// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define connection orchestration and JSON request dispatching.
//
// Key Components:
//   - InnerTubeError / APIError: Structured InnerTube execution exception types
//   - doInnerTube: Encapsulates authentication injection, visitor IDs, context merging, and POST dispatching
//   - setHeaders: injects browser headers to bypass simple bot checks
//
// Dependencies:
//   - bytes
//   - context
//   - encoding/json
//   - fmt
//   - io
//   - net/http
//   - strings
//   - time
//
// Error Types:
//   - InnerTubeError: returned on HTTP status failure or backend response failure
//
package ytm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	writeMinGapDefault = 500 * time.Millisecond
	refusalRetryDelay  = 250 * time.Millisecond
	refusalPauseRun    = 6
	refusalPauseBase   = 5 * time.Second
	refusalPauseMaxPow = 3 // 5s << 3 = 40s
)

// APIError represents the structured error response from InnerTube API.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// InnerTubeError wraps HTTP status errors and parses APIError when available.
type InnerTubeError struct {
	ResponseCode int
	Body         string
	Parsed       *APIError
}

func (e *InnerTubeError) Error() string {
	if e.Parsed != nil {
		return fmt.Sprintf("InnerTube error (HTTP %d, Code %d): %s - %s", e.ResponseCode, e.Parsed.Code, e.Parsed.Status, e.Parsed.Message)
	}
	return fmt.Sprintf("InnerTube error (HTTP %d): %s", e.ResponseCode, e.Body)
}

// writeResponse captures the common status and actions fields returned by
// InnerTube mutation endpoints.
type writeResponse struct {
	Status  string            `json:"status"`
	Actions []json.RawMessage `json:"actions,omitempty"`
}

func (r writeResponse) checkStatus() error {
	if r.Status != "" && !strings.Contains(r.Status, "SUCCEEDED") {
		return fmt.Errorf("%w: %s", ErrEditRejected, r.Status)
	}
	return nil
}

func (r writeResponse) asksAboutDuplicates() bool {
	if r.Status == "" || strings.Contains(r.Status, "SUCCEEDED") {
		return false
	}
	for _, act := range r.Actions {
		if bytes.Contains(act, []byte("DEDUPE_OPTION_SKIP")) {
			return true
		}
	}
	return false
}

func isWritePath(path string) bool {
	return strings.HasPrefix(path, "like/") ||
		path == "playlist/create" ||
		path == "playlist/delete" ||
		path == "browse/edit_playlist" ||
		path == "feedback" ||
		strings.HasPrefix(path, "subscription/")
}

func (c *Client) waitIfPaused(ctx context.Context) error {
	for {
		c.backoffMu.Lock()
		until := c.pausedUntil
		c.backoffMu.Unlock()
		wait := until.Sub(c.now())
		if wait <= 0 {
			return nil
		}
		if err := c.sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func (c *Client) recordRefusal() int {
	c.backoffMu.Lock()
	defer c.backoffMu.Unlock()
	c.refusals++
	n := c.refusals
	if n >= refusalPauseRun {
		shift := n - refusalPauseRun
		if shift > refusalPauseMaxPow {
			shift = refusalPauseMaxPow
		}
		until := c.now().Add(refusalPauseBase << shift)
		if until.After(c.pausedUntil) {
			c.pausedUntil = until
		}
	}
	return n
}

func (c *Client) resetRefusals() {
	c.backoffMu.Lock()
	c.refusals = 0
	c.pausedUntil = time.Time{}
	c.backoffMu.Unlock()
}

/*
doInnerTube sends a JSON POST request containing the merged client context.

    params:
          ctx: execution context
          path: URL path suffix (e.g. search, browse)
          clientCtx: client platform impersonation parameters
          extra: map containing request parameters (like videoId, browseId)
          authed: requires authorized credentials
          respBody: pointer to unmarshal destination struct
    returns:
          error: error if request or unmarshal failed
*/
func (c *Client) doInnerTube(ctx context.Context, path string, clientCtx ClientContext, extra map[string]any, authed bool, respBody any) error {
	if authed && c.auth == nil {
		return ErrLoginRequired
	}

	if path != "visitor_id" {
		if err := c.ensureVisitorID(ctx); err != nil {
			return err
		}
	}

	if c.visitorID != "" {
		clientCtx.VisitorData = c.visitorID
	}

	reqContext := map[string]any{
		"client": clientCtx,
		"user":   map[string]any{},
	}

	reqMap := map[string]any{
		"context": reqContext,
	}
	for k, v := range extra {
		reqMap[k] = v
	}

	apiURL := c.apiURL
	if clientCtx.ClientName == "WEB" {
		apiURL = c.nonMusicURL
	}

	url := apiURL + path

	var reqBodyBytes []byte
	if len(reqMap) > 0 {
		var err error
		reqBodyBytes, err = json.Marshal(reqMap)
		if err != nil {
			return err
		}
	}

	if authed {
		if err := c.waitIfPaused(ctx); err != nil {
			return err
		}
		if isWritePath(path) {
			c.writeMu.Lock()
			if !c.lastWriteAt.IsZero() && c.writeMinGap > 0 {
				if wait := c.writeMinGap - c.now().Sub(c.lastWriteAt); wait > 0 {
					if err := c.sleep(ctx, wait); err != nil {
						c.writeMu.Unlock()
						return err
					}
				}
			}
			defer func() {
				c.lastWriteAt = c.now()
				c.writeMu.Unlock()
			}()
		}
	}

	const maxAttempts = 2
	var bodyBytes []byte
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var err error
		bodyBytes, err = c.sendInnerTubeOnce(ctx, url, clientCtx, reqBodyBytes, authed)
		if err == nil {
			if authed {
				c.resetRefusals()
			}
			break
		}
		if !authed {
			return err
		}
		itErr, ok := err.(*InnerTubeError)
		if !ok || itErr.ResponseCode != http.StatusForbidden {
			return err
		}
		refusals := c.recordRefusal()
		if attempt == maxAttempts-1 {
			if refusals >= refusalPauseRun {
				return fmt.Errorf("%w: %w", ErrRateLimited, err)
			}
			return err
		}
		if sleepErr := c.sleep(ctx, refusalRetryDelay); sleepErr != nil {
			return sleepErr
		}
	}

	// The viewer state is only in the body, so it is recorded here, while the raw
	// bytes are still available.
	if state := classifyAuthState(bodyBytes); state != authStateUnknown {
		c.lastAuthState = state
		if obs, ok := respBody.(authObserver); ok {
			obs.setAuthState(state)
		}
	}

	return json.Unmarshal(bodyBytes, respBody)
}

func (c *Client) sendInnerTubeOnce(ctx context.Context, url string, clientCtx ClientContext, reqBodyBytes []byte, authed bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, clientCtx, authed)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error *APIError `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &apiErr)

		err := &InnerTubeError{
			ResponseCode: resp.StatusCode,
			Body:         string(bodyBytes),
			Parsed:       apiErr.Error,
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: %w", ErrSessionExpired, err)
		}
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

/*
setHeaders sets required request headers for InnerTube request.

    params:
          req: request pointer
          clientCtx: client configuration
          authed: inject user cookies and auth headers
*/
func (c *Client) setHeaders(req *http.Request, clientCtx ClientContext, authed bool) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	// The client identity headers default to the desktop web client, which is
	// what every catalogue endpoint uses. A context that names its own numeric
	// client id overrides them, which playback profiles such as VISIONOS need
	// to stay internally consistent.
	clientID := "67"
	clientVersion := "1.20251111.09.00"
	if clientCtx.ClientID != "" {
		clientID = clientCtx.ClientID
	}
	if clientCtx.ClientVersion != "" {
		clientVersion = clientCtx.ClientVersion
	}
	req.Header.Set("X-YouTube-Client-Name", clientID)
	req.Header.Set("X-YouTube-Client-Version", clientVersion)

	// One Google account can own several channels, and the server picks the
	// channel from this index. A hardcoded "0" makes every brand channel behave
	// like the first one.
	authUser := "0"
	if c.auth != nil && c.auth.AuthUser != "" {
		authUser = c.auth.AuthUser
	}
	req.Header.Set("X-Goog-AuthUser", authUser)
	if c.visitorID != "" {
		req.Header.Set("X-Goog-EOM-Visitor-Id", c.visitorID)
	}

	origin := "https://music.youtube.com"
	if clientCtx.ClientName == "WEB" {
		origin = "https://www.youtube.com"
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Origin", origin)

	// The user agent follows the impersonated client when it declares one, so a
	// mobile profile is not paired with a desktop UA. The default stays the
	// desktop string the catalogue endpoints were tuned against.
	userAgent := YtmUserAgent
	if clientCtx.UserAgent != "" {
		userAgent = clientCtx.UserAgent
	}
	req.Header.Set("User-Agent", userAgent)

	if authed && c.auth != nil {
		req.Header.Set("Cookie", c.auth.Cookie)
		req.Header.Set("Authorization", c.auth.GetAuthorizationHeader())
	}
}
