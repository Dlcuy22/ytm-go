// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement song information fetching.
//
// Key Components:
//   - LoadSong: retrieves details for a track by sending requests to /next and /player endpoints
//   - parseSongResponse: parses /next queue items to extract lyrics and related browse IDs
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
	"log/slog"
)

/*
tabBrowseID returns the browse ID of the first tab whose page type matches.

Tabs are not a fixed list: a music track gets Up next, Lyrics, Comments and
Related, while a podcast gets Up next, Details, Comments and Related. Indexing
them positionally hands back the wrong page, which is why the page type is what
this looks at.

    params:
          tabs: watch-next tabs
          pageType: MUSIC_PAGE_TYPE_* identifier to match
    returns:
          string: the matching tab's browse ID, or empty when no tab matches
*/
func tabBrowseID(tabs []Tab, pageType string) string {
	for _, tab := range tabs {
		if tab.TabRenderer.Endpoint == nil {
			continue
		}
		endpoint := tab.TabRenderer.Endpoint.BrowseEndpoint
		if endpoint.PageType() == pageType && endpoint.BrowseID != "" {
			return endpoint.BrowseID
		}
	}
	return ""
}

/*
LoadSong retrieves metadata details for the specified song ID.

    params:
          ctx: execution context
          songID: YouTube track ID
    returns:
          *Song: track metadata details
          error: network or parsing error
*/
func (c *Client) LoadSong(ctx context.Context, songID string) (*Song, error) {
	songID = CleanSongID(songID)

	var nextResp YoutubeiNextResponse
	err := c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), map[string]any{
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"videoId":                       songID,
	}, false, &nextResp)

	if err == nil {
		if song := parseSongResponse(c.log(), songID, &nextResp); song != nil {
			c.cache.StoreSong(song)
			return song, nil
		}
	}

	var playerResp struct {
		VideoDetails *struct {
			Title     string `json:"title"`
			ChannelID string `json:"channelId"`
		} `json:"videoDetails"`
	}
	err = c.doInnerTube(ctx, "player", GetContextWebRemix(c.hl), map[string]any{
		"videoId": songID,
	}, false, &playerResp)
	if err != nil {
		return nil, err
	}

	if playerResp.VideoDetails == nil {
		return nil, fmt.Errorf("videoDetails missing from player response")
	}

	song := &Song{
		ID:   songID,
		Name: playerResp.VideoDetails.Title,
		Artists: []Artist{
			{ID: playerResp.VideoDetails.ChannelID},
		},
	}

	c.cache.StoreSong(song)
	return song, nil
}

func parseSongResponse(log *slog.Logger, songID string, nextResp *YoutubeiNextResponse) *Song {
	defer func() {
		if r := recover(); r != nil {
			logPanicWith(log, "parseSongResponse", r)
		}
	}()

	tabs := nextResp.Contents.SingleColumnMusicWatchNextResultsRenderer.TabbedRenderer.WatchNextTabbedResultsRenderer.Tabs
	if len(tabs) == 0 {
		return nil
	}

	lyricsBrowseID := tabBrowseID(tabs, "MUSIC_PAGE_TYPE_TRACK_LYRICS")
	relatedBrowseID := tabBrowseID(tabs, "MUSIC_PAGE_TYPE_TRACK_RELATED")

	tab0 := tabs[0]
	if tab0.TabRenderer.Content == nil || tab0.TabRenderer.Content.MusicQueueRenderer == nil || tab0.TabRenderer.Content.MusicQueueRenderer.Content == nil {
		return nil
	}

	contents := tab0.TabRenderer.Content.MusicQueueRenderer.Content.PlaylistPanelRenderer.Contents
	if len(contents) == 0 {
		return nil
	}

	video := contents[0].GetRenderer()
	if video == nil {
		return nil
	}

	title := video.Title.FirstText()
	isExplicit := false
	for _, b := range video.Badges {
		if b.IsExplicit() {
			isExplicit = true
			break
		}
	}

	artists := video.GetArtists()
	var duration int64
	if video.LengthText != nil {
		duration = parseDurationMs(video.LengthText.FirstText())
	}

	return &Song{
		ID:              songID,
		Name:            title,
		Artists:         artists,
		IsExplicit:      isExplicit,
		DurationMs:      duration,
		LyricsBrowseID:  lyricsBrowseID,
		RelatedBrowseID: relatedBrowseID,
		Thumbnail:       NewThumbnailProvider(video.Thumbnail.Thumbnail.Thumbnails),
		Album:           video.GetAlbum(),
	}
}
