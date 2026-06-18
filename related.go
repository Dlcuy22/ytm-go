// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement related content resolving for tracks.
//
// Key Components:
//   - RelatedGroup: collection of items related to a track
//   - GetSongRelated: requests and parses recommended playlists, albums, and tracks using relatedBrowseId
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

// RelatedGroup represents a section of related media items.
type RelatedGroup struct {
	Title       string      `json:"title"`
	Items       []MediaItem `json:"items,omitempty"`
	Description string      `json:"description,omitempty"`
}

/*
GetSongRelated queries and parses related tracks and albums for the specified track.

    params:
          ctx: execution context
          songID: YouTube track ID
    returns:
          []RelatedGroup: related groups of media items
          error: network or parsing error
*/
func (c *Client) GetSongRelated(ctx context.Context, songID string) ([]RelatedGroup, error) {
	songID = CleanSongID(songID)

	song, err := c.LoadSong(ctx, songID)
	if err != nil {
		return nil, err
	}

	if song.RelatedBrowseID == "" {
		return nil, fmt.Errorf("song has no related_browse_id")
	}

	var resp struct {
		Contents struct {
			SectionListRenderer *struct {
				Contents []YoutubeiShelf `json:"contents"`
			} `json:"sectionListRenderer"`
		} `json:"contents"`
	}

	err = c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": song.RelatedBrowseID,
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	var groups []RelatedGroup
	if resp.Contents.SectionListRenderer != nil {
		for _, group := range resp.Contents.SectionListRenderer.Contents {
			groups = append(groups, RelatedGroup{
				Title:       group.Title(),
				Items:       group.GetMediaItems(c.hl),
				Description: group.Description(),
			})
		}
	}

	return groups, nil
}
