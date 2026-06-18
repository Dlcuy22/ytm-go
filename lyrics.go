// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement lyrics loading.
//
// Key Components:
//   - GetSongLyrics: requests and decodes track lyrics from browse endpoint using lyricsBrowseId
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

/*
GetSongLyrics retrieves the plaintext lyrics content for the specified lyrics browse ID.

    params:
          ctx: execution context
          lyricsID: lyrics browse ID
    returns:
          string: plaintext lyrics
          error: network or parsing error
*/
func (c *Client) GetSongLyrics(ctx context.Context, lyricsID string) (string, error) {
	if lyricsID == "" {
		return "", fmt.Errorf("empty lyrics ID")
	}

	var resp struct {
		Contents struct {
			SectionListRenderer *struct {
				Contents []struct {
					MusicDescriptionShelfRenderer *struct {
						Description TextRuns `json:"description"`
					} `json:"musicDescriptionShelfRenderer"`
				} `json:"contents"`
			} `json:"sectionListRenderer"`
		} `json:"contents"`
	}

	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": lyricsID,
	}, false, &resp)
	if err != nil {
		return "", err
	}

	defer func() { recover() }()
	if resp.Contents.SectionListRenderer != nil && len(resp.Contents.SectionListRenderer.Contents) > 0 {
		shelf := resp.Contents.SectionListRenderer.Contents[0].MusicDescriptionShelfRenderer
		if shelf != nil {
			return shelf.Description.FirstText(), nil
		}
	}

	return "", fmt.Errorf("lyrics description shelf not found")
}
