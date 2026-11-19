// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement mix queue loading from the watch-next endpoint.
//
// Key Components:
//   - GetMix: loads an "RD…" mix playlist and its continuation pages
//   - mixVideoID: recovers the seed video a share id carries
//   - playlistPanelSongs: flattens a queue panel into songs
//
// Dependencies:
//   - context
//   - fmt
//   - strings
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"fmt"
	"strings"
)

// IsMixID reports whether a playlist id names a mix, the "RD…" form the browse
// endpoint refuses and only the watch-next queue resolves.
func IsMixID(playlistID string) bool {
	return strings.HasPrefix(CleanPlaylistID(strings.TrimSpace(playlistID)), "RD")
}

/*
GetMix loads the tracks behind a YouTube mix playlist, the "RD…" id a mix share
URL carries (for example RDLoK17z6xDwI). The browse endpoint rejects these with
"This playlist type is unviewable.", so the queue comes from the watch-next
endpoint instead.

The first call sends the playlist id and, when the id encodes one, the seed
video, so the queue starts where the share URL would. A continuation call sends
only the token.

    params:
          ctx: execution context
          playlistID: mix playlist id, with or without a share URL prefix
          continuation: opt-in next page, as returned in Playlist.Continuation
    returns:
          *Playlist: the mix tracks, typed as radio
          error: network, parsing, or empty-queue error
*/
func (c *Client) GetMix(ctx context.Context, playlistID string, continuation *BuiltInContinuation) (*Playlist, error) {
	playlistID = CleanPlaylistID(strings.TrimSpace(playlistID))
	if !IsMixID(playlistID) {
		return nil, fmt.Errorf("%q is not a mix id", playlistID)
	}

	if continuation != nil && continuation.Token != "" {
		return c.mixPage(ctx, playlistID, continuation.Token, map[string]any{
			"continuation": continuation.Token,
		})
	}

	bodyParams := map[string]any{
		"playlistId":  playlistID,
		"isAudioOnly": true,
	}
	if videoID := mixVideoID(playlistID); videoID != "" {
		bodyParams["videoId"] = videoID
	}

	return c.mixPage(ctx, playlistID, "", bodyParams)
}

// mixPage sends one watch-next request and turns its queue panel into a
// playlist page. A page the endpoint did not answer with a queue is an error,
// because an empty playlist would hide a rejected id behind a valid response.
func (c *Client) mixPage(ctx context.Context, playlistID, token string, bodyParams map[string]any) (*Playlist, error) {
	panel, err := c.loadQueuePanel(ctx, token, bodyParams)
	if err != nil {
		return nil, err
	}
	if panel == nil {
		if token != "" {
			return nil, fmt.Errorf("no mix queue page for %s", playlistID)
		}

		return nil, fmt.Errorf("no mix queue for %s", playlistID)
	}

	playlist := &Playlist{
		ID:    playlistID,
		Items: playlistPanelSongs(panel.Contents),
		Type:  PlaylistTypeRadio,
	}
	if len(panel.Continuations) > 0 {
		if next := panel.Continuations[0].GetToken(); next != "" {
			playlist.Continuation = &BuiltInContinuation{
				Token:  next,
				Type:   ContinuationMix,
				ItemID: playlistID,
			}
		}
	}

	return playlist, nil
}

// loadQueuePanel dispatches a next request and returns the queue panel it
// answered with, or nil when the response carried none.
func (c *Client) loadQueuePanel(ctx context.Context, token string, bodyParams map[string]any) (*PlaylistPanelRenderer, error) {
	if token != "" {
		var resp YoutubeiNextContinuationResponse
		if err := c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), bodyParams, c.auth != nil, &resp); err != nil {
			return nil, err
		}

		return &resp.ContinuationContents.PlaylistPanelContinuation, nil
	}

	var resp YoutubeiNextResponse
	if err := c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), bodyParams, c.auth != nil, &resp); err != nil {
		return nil, err
	}

	tabs := resp.Contents.SingleColumnMusicWatchNextResultsRenderer.TabbedRenderer.WatchNextTabbedResultsRenderer.Tabs
	if len(tabs) == 0 || tabs[0].TabRenderer.Content == nil {
		return nil, nil
	}
	queue := tabs[0].TabRenderer.Content.MusicQueueRenderer
	if queue == nil || queue.Content == nil {
		return nil, nil
	}

	return &queue.Content.PlaylistPanelRenderer, nil
}

// playlistPanelSongs flattens a queue panel, skipping entries with no video id
// so a header or a divider cannot become an unplayable row.
func playlistPanelSongs(items []ResponseRadioItem) []Song {
	songs := make([]Song, 0, len(items))
	for _, item := range items {
		renderer := item.GetRenderer()
		if renderer == nil || renderer.VideoID == "" {
			continue
		}
		explicit := false
		for _, badge := range renderer.Badges {
			if badge.IsExplicit() {
				explicit = true
				break
			}
		}
		var duration int64
		if renderer.LengthText != nil {
			duration = parseDurationMs(renderer.LengthText.FirstText())
		}
		songs = append(songs, Song{
			ID:         CleanSongID(renderer.VideoID),
			Name:       renderer.Title.FirstText(),
			DurationMs: duration,
			Artists:    renderer.GetArtists(),
			Album:      renderer.GetAlbum(),
			Thumbnail:  NewThumbnailProvider(renderer.Thumbnail.List()),
			IsExplicit: explicit,
		})
	}

	return songs
}

// mixVideoID returns the seed video a mix id encodes, or "". A mix built around
// a video is "RD" followed by the eleven-character video id, which is the form
// a YouTube mix share URL carries; other mix ids (RDAMVM…, RDCLAK…, RDAT…) name
// no single seed.
func mixVideoID(playlistID string) string {
	if len(playlistID) != len("RD")+11 || !strings.HasPrefix(playlistID, "RD") {
		return ""
	}
	for _, r := range playlistID[len("RD"):] {
		valid := r == '-' || r == '_' ||
			r >= '0' && r <= '9' ||
			r >= 'A' && r <= 'Z' ||
			r >= 'a' && r <= 'z'
		if !valid {
			return ""
		}
	}

	return playlistID[len("RD"):]
}
