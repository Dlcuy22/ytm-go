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

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return err
	}

	c.setHeaders(req, clientCtx, authed)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		var apiErr struct {
			Error *APIError `json:"error"`
		}
		_ = json.Unmarshal(bodyBytes, &apiErr)

		return &InnerTubeError{
			ResponseCode: resp.StatusCode,
			Body:         string(bodyBytes),
			Parsed:       apiErr.Error,
		}
	}

	return json.NewDecoder(resp.Body).Decode(respBody)
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

	// Use desktop/web client name and version headers to avoid InnerTube restrictions on mobile requests
	req.Header.Set("X-YouTube-Client-Name", "67")
	req.Header.Set("X-YouTube-Client-Version", "1.20251111.09.00")

	req.Header.Set("X-Goog-AuthUser", "0")
	if c.visitorID != "" {
		req.Header.Set("X-Goog-EOM-Visitor-Id", c.visitorID)
	}

	origin := "https://music.youtube.com"
	if clientCtx.ClientName == "WEB" {
		origin = "https://www.youtube.com"
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Origin", origin)

	// Use desktop User-Agent to avoid bot-detection and streaming playback restrictions
	req.Header.Set("User-Agent", YtmUserAgent)

	if authed && c.auth != nil {
		req.Header.Set("Cookie", c.auth.Cookie)
		req.Header.Set("Authorization", c.auth.GetAuthorizationHeader())
	}
}