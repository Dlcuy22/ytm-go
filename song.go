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
)

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
		if song := parseSongResponse(songID, &nextResp); song != nil {
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

func parseSongResponse(songID string, nextResp *YoutubeiNextResponse) *Song {
	defer func() { recover() }()

	tabs := nextResp.Contents.SingleColumnMusicWatchNextResultsRenderer.TabbedRenderer.WatchNextTabbedResultsRenderer.Tabs
	if len(tabs) == 0 {
		return nil
	}

	var lyricsBrowseID string
	if len(tabs) > 1 && tabs[1].TabRenderer.Endpoint != nil && tabs[1].TabRenderer.Endpoint.BrowseEndpoint.BrowseID != "" {
		lyricsBrowseID = tabs[1].TabRenderer.Endpoint.BrowseEndpoint.BrowseID
	}

	var relatedBrowseID string
	if len(tabs) > 2 && tabs[2].TabRenderer.Endpoint != nil && tabs[2].TabRenderer.Endpoint.BrowseEndpoint.BrowseID != "" {
		relatedBrowseID = tabs[2].TabRenderer.Endpoint.BrowseEndpoint.BrowseID
	}

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
