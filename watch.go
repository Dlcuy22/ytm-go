// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement recording playback progress (marking songs as watched).
//
// Key Components:
//   - MarkSongAsWatched: reports song playback start to YouTube tracking servers
//   - generateCpn: generates a random tracking correlation identifier
//
// Dependencies:
//   - context
//   - fmt
//   - math/rand
//   - net/http
//   - strings
//   - time
//
// Error Types:
//   - ErrLoginRequired: returned if the action is attempted without credentials
//
package ytm

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type PlaybackTrackingResponse struct {
	PlaybackTracking struct {
		VideostatsPlaybackURL struct {
			BaseURL string `json:"baseUrl"`
		} `json:"videostatsPlaybackUrl"`
	} `json:"playbackTracking"`
}

const cpnAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"

func generateCpn() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var sb strings.Builder
	for i := 0; i < 16; i++ {
		sb.WriteByte(cpnAlphabet[r.Intn(len(cpnAlphabet))])
	}
	return sb.String()
}

/*
MarkSongAsWatched reports song playback to YouTube stats tracking backend.

    params:
          ctx: execution context
          songID: track identifier to mark as watched
    returns:
          error: network or validation error
*/
func (c *Client) MarkSongAsWatched(ctx context.Context, songID string) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}

	var tracking PlaybackTrackingResponse
	err := c.doInnerTube(ctx, "player", GetContextWebRemix(c.hl), map[string]any{
		"videoId": songID,
	}, true, &tracking)
	if err != nil {
		// Fallback to ANDROID_MUSIC context if WEB_REMIX fails
		err = c.doInnerTube(ctx, "player", GetContextAndroidMusic(c.hl), map[string]any{
			"videoId": songID,
		}, true, &tracking)
		if err != nil {
			return err
		}
	}

	playbackURL := tracking.PlaybackTracking.VideostatsPlaybackURL.BaseURL
	if !strings.Contains(playbackURL, "s.youtube.com") {
		return fmt.Errorf("invalid playback tracking URL: %s", playbackURL)
	}

	playbackURL = strings.Replace(playbackURL, "s.youtube.com", "music.youtube.com", 1)

	req, err := http.NewRequestWithContext(ctx, "GET", playbackURL, nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Set("ver", "2")
	q.Set("c", "WEB_REMIX")
	q.Set("cpn", generateCpn())
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", YtmUserAgent)
	if c.auth != nil {
		req.Header.Set("Cookie", c.auth.Cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("playback tracking request failed with status: %d", resp.StatusCode)
	}

	return nil
}
