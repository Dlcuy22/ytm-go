// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement song liking modification and check.
//
// Key Components:
//   - SongLikedStatus: enum for track preference (neutral, liked, disliked)
//   - SetSongLiked: updates liking state on a track
//   - GetSongLiked: checks preference status from /next player overlays
//
// Dependencies:
//   - context
//   - fmt
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"fmt"
)

// SongLikedStatus represents liking preference state.
type SongLikedStatus string

const (
	SongLikedStatusNeutral  SongLikedStatus = "NEUTRAL"
	SongLikedStatusLiked    SongLikedStatus = "LIKED"
	SongLikedStatusDisliked SongLikedStatus = "DISLIKED"
)

/*
SetSongLiked updates the liked preference status of a track.

    params:
          ctx: execution context
          songID: YouTube track ID
          liked: preference target (NEUTRAL, LIKED, DISLIKED)
    returns:
          error: error if request or unmarshal failed
*/
func (c *Client) SetSongLiked(ctx context.Context, songID string, liked SongLikedStatus) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}

	path := ""
	switch liked {
	case SongLikedStatusNeutral:
		path = "like/removelike"
	case SongLikedStatusLiked:
		path = "like/like"
	case SongLikedStatusDisliked:
		path = "like/dislike"
	default:
		return fmt.Errorf("invalid liked status: %s", liked)
	}

	var resp struct{}
	return c.doInnerTube(ctx, path, GetContextWebRemix(c.hl), map[string]any{
		"target": map[string]any{
			"videoId": songID,
		},
	}, true, &resp)
}

/*
GetSongLiked checks the liked preference status of a track.

    params:
          ctx: execution context
          songID: YouTube track ID
    returns:
          SongLikedStatus: preference state
          error: network or parsing error
*/
func (c *Client) GetSongLiked(ctx context.Context, songID string) (SongLikedStatus, error) {
	if err := requireAuth(c.auth); err != nil {
		return SongLikedStatusNeutral, err
	}

	var resp struct {
		PlayerOverlays *struct {
			PlayerOverlayRenderer *struct {
				Actions []struct {
					LikeButtonRenderer *struct {
						LikeStatus   string `json:"likeStatus"`
						LikesAllowed bool   `json:"likesAllowed"`
					} `json:"likeButtonRenderer"`
				} `json:"actions"`
			} `json:"playerOverlayRenderer"`
		} `json:"playerOverlays"`
	}

	err := c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), map[string]any{
		"videoId": songID,
	}, true, &resp)
	if err != nil {
		return SongLikedStatusNeutral, err
	}

	defer func() { recover() }()
	if resp.PlayerOverlays != nil && resp.PlayerOverlays.PlayerOverlayRenderer != nil {
		for _, action := range resp.PlayerOverlays.PlayerOverlayRenderer.Actions {
			if action.LikeButtonRenderer != nil {
				status := action.LikeButtonRenderer.LikeStatus
				switch status {
				case "LIKE":
					return SongLikedStatusLiked, nil
				case "DISLIKE":
					return SongLikedStatusDisliked, nil
				case "INDIFFERENT":
					return SongLikedStatusNeutral, nil
				default:
					return SongLikedStatusNeutral, fmt.Errorf("unknown like status: %s", status)
				}
			}
		}
	}

	return SongLikedStatusNeutral, fmt.Errorf("likeButtonRenderer not found in response")
}
