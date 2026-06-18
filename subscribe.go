// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement artist subscription status modification and checker.
//
// Key Components:
//   - SetSubscribedToArtist: Subscribes or unsubscribes to an artist channel
//   - GetSubscribedToArtist: Checks if the user is subscribed to an artist
//
// Dependencies:
//   - context
//   - errors
//
// Error Types:
//   - ErrLoginRequired: returned if the action is attempted without credentials
//
package ytm

import (
	"context"
	"errors"
)

/*
SetSubscribedToArtist updates the subscription status to an artist channel.

    params:
          ctx: execution context
          artistID: channel ID of the artist
          subscribed: true to subscribe, false to unsubscribe
          subscribeChannelID: optional specific subscription channel ID
    returns:
          error: error if request or unmarshal failed
*/
func (c *Client) SetSubscribedToArtist(ctx context.Context, artistID string, subscribed bool, subscribeChannelID string) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}

	path := "subscription/subscribe"
	if !subscribed {
		path = "subscription/unsubscribe"
	}

	targetChannelID := subscribeChannelID
	if targetChannelID == "" {
		targetChannelID = artistID
	}

	var resp struct{}
	return c.doInnerTube(ctx, path, GetContextWebRemix(c.hl), map[string]any{
		"channelIds": []string{targetChannelID},
	}, true, &resp)
}

/*
GetSubscribedToArtist checks if currently subscribed to the artist.

    params:
          ctx: execution context
          artistID: channel ID of the artist
    returns:
          bool: true if subscribed, false otherwise
          error: network or parsing error
*/
func (c *Client) GetSubscribedToArtist(ctx context.Context, artistID string) (bool, error) {
	if err := requireAuth(c.auth); err != nil {
		return false, err
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextMobile(c.hl), map[string]any{
		"browseId": artistID,
	}, true, &resp)
	if err != nil {
		return false, err
	}

	headerRenderer := resp.Header.GetRenderer()
	if headerRenderer != nil && headerRenderer.SubscriptionButton != nil {
		return headerRenderer.SubscriptionButton.SubscribeButtonRenderer.Subscribed, nil
	}

	return false, errors.New("subscription button not found in artist header")
}
