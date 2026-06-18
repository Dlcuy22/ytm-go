// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Handle YouTube Music visitor ID acquisition and lifecycle.
//
// Key Components:
//   - GetVisitorID: Fetches a fresh visitor ID from InnerTube
//   - ensureVisitorID: Lazy loader ensuring visitor ID is present before dispatching requests
//
// Dependencies:
//   - context
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
)

type visitorIDResponse struct {
	ResponseContext struct {
		VisitorData string `json:"visitorData"`
	} `json:"responseContext"`
}

/*
GetVisitorID requests a new visitor ID from the visitor_id endpoint.

    params:
          ctx: execution context
    returns:
          string: visitor ID token
          error: network or parsing error
*/
func (c *Client) GetVisitorID(ctx context.Context) (string, error) {
	var resp visitorIDResponse
	// GetVisitorID uses doInnerTube but forces no visitor ID injection during the bootstrap call
	err := c.doInnerTube(ctx, "visitor_id", GetContextWebRemix(c.hl), nil, false, &resp)
	if err != nil {
		return "", err
	}
	return resp.ResponseContext.VisitorData, nil
}

/*
ensureVisitorID checks and retrieves a visitor ID if not already cached.

    params:
          ctx: execution context
    returns:
          error: error if fetching fails
*/
func (c *Client) ensureVisitorID(ctx context.Context) error {
	if c.visitorID != "" {
		return nil
	}
	id, err := c.GetVisitorID(ctx)
	if err != nil {
		return err
	}
	c.visitorID = id
	return nil
}
