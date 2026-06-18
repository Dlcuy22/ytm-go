// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement playlist details loading and parsing.
//
// Key Components:
//   - LoadPlaylist: retrieves metadata and tracks for a playlist, album, or radio
//   - parsePlaylistResponse: unmarshals the browse response into a Playlist struct
//   - formatBrowseID: formats a playlist ID prefix ensuring VL is prepended when necessary
//
// Dependencies:
//   - context
//   - strconv
//   - strings
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"strconv"
	"strings"
)

/*
LoadPlaylist retrieves metadata and track list contents for the specified playlist ID.

    params:
          ctx: execution context
          playlistID: playlist browse ID
          continuation: opt-in pagination token
          browseParams: opt-in filter params
          playlistURL: opt-in canonical URL
          useNonMusicAPI: force standard YouTube API rather than Music
    returns:
          *Playlist: populated playlist metadata
          error: network or parsing error
*/
func (c *Client) LoadPlaylist(ctx context.Context, playlistID string, continuation *BuiltInContinuation, browseParams *string, playlistURL *string, useNonMusicAPI bool) (*Playlist, error) {
	playlistID = CleanPlaylistID(playlistID)

	if continuation != nil {
		items, cont, err := c.GetPlaylistContinuation(ctx, false, continuation.Token, continuation.PlaylistSkipAmt)
		if err != nil {
			return nil, err
		}
		var songs []Song
		for _, item := range items {
			if s, ok := item.(*Song); ok {
				songs = append(songs, *s)
			}
		}
		return &Playlist{
			ID:           playlistID,
			Items:        songs,
			Continuation: cont,
		}, nil
	}

	browseID := playlistID
	if browseParams == nil {
		browseID = formatBrowseID(playlistID)
	}

	var loadedPlaylistURL string
	if playlistURL != nil {
		loadedPlaylistURL = *playlistURL
	}

	var respData *YoutubeiBrowseResponse

	if loadedPlaylistURL == "" {
		var resp YoutubeiBrowseResponse
		bodyParams := map[string]any{
			"browseId": browseID,
		}
		if browseParams != nil {
			bodyParams["params"] = *browseParams
		}

		ctxMobile := GetContextWebRemix(c.hl)
		if useNonMusicAPI {
			ctxMobile = GetContextWeb(c.hl)
		}

		err := c.doInnerTube(ctx, "browse", ctxMobile, bodyParams, false, &resp)
		if err != nil {
			return nil, err
		}
		respData = &resp

		if resp.Microformat != nil {
			loadedPlaylistURL = resp.Microformat.MicroformatDataRenderer.URLCanonical
		}
	}

	if loadedPlaylistURL != "" {
		if idx := strings.Index(loadedPlaylistURL, "?list="); idx != -1 {
			start := idx + 6
			end := strings.Index(loadedPlaylistURL[start:], "&")
			if end == -1 {
				end = len(loadedPlaylistURL)
			} else {
				end = start + end
			}
			browseID = formatBrowseID(loadedPlaylistURL[start:end])
		}
	}

	initialPlaylist := parsePlaylistResponse(playlistID, respData, c.hl, false)
	if initialPlaylist != nil && initialPlaylist.Name != "" {
		c.cache.StorePlaylist(initialPlaylist)
		return initialPlaylist, nil
	}

	var resp YoutubeiBrowseResponse
	bodyParams := map[string]any{
		"browseId": browseID,
	}
	if browseParams != nil {
		bodyParams["params"] = *browseParams
	}

	ctxMobile := GetContextWebRemix(c.hl)
	if useNonMusicAPI {
		ctxMobile = GetContextWeb(c.hl)
	}

	err := c.doInnerTube(ctx, "browse", ctxMobile, bodyParams, false, &resp)
	if err != nil {
		return nil, err
	}

	playlist := parsePlaylistResponse(playlistID, &resp, c.hl, false)
	if playlist != nil {
		playlist.PlaylistURL = loadedPlaylistURL
		emptyPrefix := "https://www.gstatic.com/youtube/media/ytm/images/pbg/playlist-empty-state"
		if playlist.Thumbnail != nil && (strings.HasPrefix(playlist.Thumbnail.UrlA, emptyPrefix) || (playlist.Thumbnail.UrlB != nil && strings.HasPrefix(*playlist.Thumbnail.UrlB, emptyPrefix))) {
			playlist.Thumbnail = nil
		}
		c.cache.StorePlaylist(playlist)
	}

	return playlist, nil
}

func formatBrowseID(id string) string {
	if !strings.HasPrefix(id, "VL") && !strings.HasPrefix(id, "MPREb_") {
		return "VL" + id
	}
	return id
}

func parsePlaylistResponse(playlistID string, parsed *YoutubeiBrowseResponse, hl string, isRadio bool) *Playlist {
	if parsed == nil {
		return nil
	}

	if isRadio {
		defer func() { recover() }()
		tab := parsed.Contents.SingleColumnBrowseResultsRenderer.Tabs[0]
		shelf := tab.TabRenderer.Content.SectionListRenderer.Contents[0].MusicPlaylistShelfRenderer

		var items []Song
		for _, data := range shelf.Contents {
			if parsedItem, _ := data.ParseItem(hl); parsedItem != nil {
				if s, ok := parsedItem.(*Song); ok {
					items = append(items, *s)
				}
			}
		}

		var continuationToken string
		if len(shelf.Continuations) > 0 {
			continuationToken = shelf.Continuations[0].GetToken()
		}
		if continuationToken == "" {
			for _, data := range shelf.Contents {
				if tok := data.toContinuationToken(); tok != "" {
					continuationToken = tok
					break
				}
			}
		}

		var continuation *BuiltInContinuation
		if continuationToken != "" {
			continuation = &BuiltInContinuation{
				Token:  continuationToken,
				Type:   ContinuationSong,
				ItemID: playlistID,
			}
		}

		var thumbnailProvider *ThumbnailProvider
		if headerRenderer := parsed.Header.GetRenderer(); headerRenderer != nil {
			thumbnailProvider = NewThumbnailProvider(headerRenderer.GetThumbnails())
		}

		return &Playlist{
			ID:           playlistID,
			Items:        items,
			Continuation: continuation,
			Thumbnail:    thumbnailProvider,
			Type:         PlaylistTypeRadio,
		}
	}

	playlist := &Playlist{
		ID: playlistID,
	}

	headerRenderer := parsed.Header.GetRenderer()
	if headerRenderer != nil {
		if headerRenderer.Title != nil {
			playlist.Name = headerRenderer.Title.FirstText()
		}
		if headerRenderer.Description != nil {
			playlist.Description = headerRenderer.Description.FirstText()
		}
		playlist.Thumbnail = NewThumbnailProvider(headerRenderer.GetThumbnails())

		if headerRenderer.Subtitle != nil {
			var artists []Artist
			for _, run := range headerRenderer.Subtitle.Runs {
				if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
					be := run.NavigationEndpoint.BrowseEndpoint
					if be.GetMediaItemType() == "ARTIST" {
						artists = append(artists, Artist{
							ID:   be.BrowseID,
							Name: run.Text,
						})
					}
				} else {
					isYear := true
					for _, r := range run.Text {
						if r < '0' || r > '9' {
							isYear = false
							break
						}
					}
					if isYear && len(run.Text) == 4 {
						playlist.Year, _ = strconv.Atoi(run.Text)
					}
				}
			}
			playlist.Artists = artists
		}

		if headerRenderer.SecondSubtitle != nil {
			runs := headerRenderer.SecondSubtitle.Runs
			for i := 0; i < len(runs); i++ {
				revIdx := len(runs) - 1 - i
				text := runs[revIdx].Text
				if i == 0 {
					playlist.TotalDurationMs = parseDurationMs(text)
				} else if i == 1 {
					var digits string
					for _, r := range text {
						if r >= '0' && r <= '9' {
							digits += string(r)
						}
					}
					playlist.ItemCount, _ = strconv.Atoi(digits)
				}
			}
		}
	}

	if parsed.Header != nil && parsed.Header.MusicDetailHeaderRenderer != nil {
		d := parsed.Header.MusicDetailHeaderRenderer
		if d.Menu != nil && d.Menu.MenuRenderer.TopLevelButtons != nil {
			for _, btn := range d.Menu.MenuRenderer.TopLevelButtons {
				if btn.ButtonRenderer != nil && btn.ButtonRenderer.NavigationEndpoint != nil && btn.ButtonRenderer.NavigationEndpoint.BrowseEndpoint != nil {
					if btn.ButtonRenderer.NavigationEndpoint.BrowseEndpoint.BrowseID == "EDIT" {
						playlist.Type = PlaylistTypePlaylist
					}
				}
			}
		}
	}

	var shelves []YoutubeiShelf
	if parsed.Contents != nil {
		tabs := parsed.Contents.SingleColumnBrowseResultsRenderer.Tabs
		if len(tabs) == 0 && parsed.Contents.TwoColumnBrowseResultsRenderer != nil {
			tabs = parsed.Contents.TwoColumnBrowseResultsRenderer.Tabs
		}
		if len(tabs) > 0 && tabs[0].TabRenderer.Content != nil && tabs[0].TabRenderer.Content.SectionListRenderer != nil {
			shelves = append(shelves, tabs[0].TabRenderer.Content.SectionListRenderer.Contents...)
		}
		if parsed.Contents.TwoColumnBrowseResultsRenderer != nil && parsed.Contents.TwoColumnBrowseResultsRenderer.SecondaryContents != nil {
			shelves = append(shelves, parsed.Contents.TwoColumnBrowseResultsRenderer.SecondaryContents.SectionListRenderer.Contents...)
		}
	}

	for _, row := range shelves {
		if playlist.Name == "" && row.Title() != "" {
			playlist.Name = row.Title()
		}
		if playlist.Description == "" && row.Description() != "" {
			playlist.Description = row.Description()
		}
		if playlist.Thumbnail == nil && len(row.GetMediaItems(hl)) > 0 {
			playlist.Thumbnail = NewThumbnailProvider(row.Thumbnails())
		}
		if len(playlist.Artists) == 0 && row.Artist() != nil {
			playlist.Artists = []Artist{*row.Artist()}
		}

		itemsAndIDs := row.GetMediaItemsAndSetIDs(hl)
		var songs []Song
		var itemSetIDs []string
		for _, entry := range itemsAndIDs {
			if s, ok := entry.Item.(*Song); ok {
				songs = append(songs, *s)
				itemSetIDs = append(itemSetIDs, entry.SetID)
			}
		}
		playlist.Items = append(playlist.Items, songs...)

		var continuationToken string
		if row.MusicPlaylistShelfRenderer != nil {
			if len(row.MusicPlaylistShelfRenderer.Continuations) > 0 {
				continuationToken = row.MusicPlaylistShelfRenderer.Continuations[0].GetToken()
			}
			if continuationToken == "" {
				for _, data := range row.MusicPlaylistShelfRenderer.Contents {
					if tok := data.toContinuationToken(); tok != "" {
						continuationToken = tok
						break
					}
				}
			}
		}

		if continuationToken != "" {
			playlist.Continuation = &BuiltInContinuation{
				Token: continuationToken,
				Type:  ContinuationPlaylist,
			}
		}

		playlist.ItemSetIDs = itemSetIDs

		if row.MusicShelfRenderer != nil && len(row.MusicShelfRenderer.Contents) > 0 {
			if first := row.MusicShelfRenderer.Contents[0]; first.MusicResponsiveListItemRenderer != nil && first.MusicResponsiveListItemRenderer.Index != nil {
				playlist.Type = PlaylistTypeAlbum
			}
		}

		if playlist.Description != "" && playlist.Name != "" && len(playlist.Items) > 0 {
			break
		}
	}

	return playlist
}
