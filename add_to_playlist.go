// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Fetch account playlists available as targets when adding a song.
//
// Key Components:
//   - AddToPlaylistTarget: playlist metadata shown in YouTube Music's "Save to playlist" sheet
//   - GetAddToPlaylistTargets: calls /playlist/get_add_to_playlist for a track
//
// Dependencies:
//   - context
//   - fmt
//   - strings
//
// Error Types:
//   - ErrLoginRequired: returned when called without credentials
//
package ytm

import (
	"context"
	"fmt"
	"strings"
)

// AddToPlaylistTarget represents an account playlist that can receive a song.
type AddToPlaylistTarget struct {
	PlaylistID string
	Title      string
	Subtitle   string // e.g. "9 tracks" from shortBylineText
	Thumbnail  string // highest-resolution URL from thumbnailRenderer
}

type addToPlaylistText struct {
	SimpleText string `json:"simpleText"`
	Runs       []struct {
		Text string `json:"text"`
	} `json:"runs"`
}

func (t addToPlaylistText) String() string {
	if t.SimpleText != "" {
		return t.SimpleText
	}
	if len(t.Runs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, r := range t.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

type addToPlaylistResponse struct {
	Contents []struct {
		AddToPlaylistRenderer *struct {
			Playlists []struct {
				PlaylistAddToOptionRenderer *struct {
					PlaylistID        string            `json:"playlistId"`
					Title             addToPlaylistText `json:"title"`
					ShortBylineText   addToPlaylistText `json:"shortBylineText"`
					ThumbnailRenderer *struct {
						MusicThumbnailRenderer *struct {
							Thumbnail *struct {
								Thumbnails []struct {
									URL string `json:"url"`
								} `json:"thumbnails"`
							} `json:"thumbnail"`
						} `json:"musicThumbnailRenderer"`
					} `json:"thumbnailRenderer"`
				} `json:"playlistAddToOptionRenderer"`
			} `json:"playlists"`
		} `json:"addToPlaylistRenderer"`
	} `json:"contents"`
}

// GetAddToPlaylistTargets returns the playlists (including "LM" for Liked Music)
// that YouTube Music offers in its "Save to playlist" sheet for videoID.
func (c *Client) GetAddToPlaylistTargets(ctx context.Context, videoID string) ([]AddToPlaylistTarget, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, err
	}
	cleanID := CleanSongID(strings.TrimSpace(videoID))
	if cleanID == "" {
		return nil, fmt.Errorf("ytm: video id is required")
	}

	var resp addToPlaylistResponse
	if err := c.doInnerTube(ctx, "playlist/get_add_to_playlist", GetContextWebRemix(c.hl), map[string]any{
		"videoIds": []string{cleanID},
	}, true, &resp); err != nil {
		return nil, err
	}

	var targets []AddToPlaylistTarget
	for _, item := range resp.Contents {
		if item.AddToPlaylistRenderer == nil {
			continue
		}
		for _, entry := range item.AddToPlaylistRenderer.Playlists {
			opt := entry.PlaylistAddToOptionRenderer
			if opt == nil {
				continue
			}
			id := strings.TrimSpace(opt.PlaylistID)
			if id == "" {
				continue
			}
			thumb := ""
			if opt.ThumbnailRenderer != nil &&
				opt.ThumbnailRenderer.MusicThumbnailRenderer != nil &&
				opt.ThumbnailRenderer.MusicThumbnailRenderer.Thumbnail != nil {
				thumbs := opt.ThumbnailRenderer.MusicThumbnailRenderer.Thumbnail.Thumbnails
				if len(thumbs) > 0 {
					thumb = thumbs[len(thumbs)-1].URL
				}
			}
			targets = append(targets, AddToPlaylistTarget{
				PlaylistID: id,
				Title:      strings.TrimSpace(opt.Title.String()),
				Subtitle:   strings.TrimSpace(opt.ShortBylineText.String()),
				Thumbnail:  thumb,
			})
		}
	}
	return targets, nil
}
