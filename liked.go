// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement queries for liked library items (albums, artists, and playlists).
//
// Key Components:
//   - GetLikedAlbums: retrieves the list of albums liked by the user
//   - GetLikedArtists: retrieves the list of artists followed/liked by the user
//   - GetLikedPlaylists: retrieves the list of playlists liked by the user (delegates to GetAccountPlaylists)
//
// Dependencies:
//   - context
//   - strings
//
// Error Types:
//   - ErrLoginRequired: returned if the action is attempted without credentials
//
package ytm

import (
	"context"
	"strings"
)

/*
GetLikedAlbums retrieves the list of liked albums in the user's library.

    params:
          ctx: execution context
    returns:
          []Playlist: list of liked albums
          error: network or parsing error
*/
func (c *Client) GetLikedAlbums(ctx context.Context) ([]Playlist, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, err
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": "FEmusic_liked_albums",
	}, true, &resp)
	if err != nil {
		return nil, err
	}

	var playlists []Playlist
	var shelves []YoutubeiShelf
	if resp.Contents != nil {
		tabs := resp.Contents.SingleColumnBrowseResultsRenderer.Tabs
		if len(tabs) == 0 && resp.Contents.TwoColumnBrowseResultsRenderer != nil {
			tabs = resp.Contents.TwoColumnBrowseResultsRenderer.Tabs
		}
		if len(tabs) > 0 && tabs[0].TabRenderer.Content != nil && tabs[0].TabRenderer.Content.SectionListRenderer != nil {
			shelves = append(shelves, tabs[0].TabRenderer.Content.SectionListRenderer.Contents...)
		}
	}

	for _, shelf := range shelves {
		if shelf.GridRenderer == nil {
			continue
		}
		for _, item := range shelf.GridRenderer.Items {
			parsed, _ := item.ParseItem(c.hl)
			if pl, ok := parsed.(*Playlist); ok {
				playlists = append(playlists, *pl)
			}
		}
	}

	return playlists, nil
}

/*
GetLikedArtists retrieves the list of artists from the user's library.

    params:
          ctx: execution context
    returns:
          []Artist: list of followed or liked artists
          error: network or parsing error
*/
func (c *Client) GetLikedArtists(ctx context.Context) ([]Artist, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, err
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": "FEmusic_library_corpus_track_artists",
	}, true, &resp)
	if err != nil {
		return nil, err
	}

	var artists []Artist
	var shelves []YoutubeiShelf
	if resp.Contents != nil {
		tabs := resp.Contents.SingleColumnBrowseResultsRenderer.Tabs
		if len(tabs) == 0 && resp.Contents.TwoColumnBrowseResultsRenderer != nil {
			tabs = resp.Contents.TwoColumnBrowseResultsRenderer.Tabs
		}
		if len(tabs) > 0 && tabs[0].TabRenderer.Content != nil && tabs[0].TabRenderer.Content.SectionListRenderer != nil {
			shelves = append(shelves, tabs[0].TabRenderer.Content.SectionListRenderer.Contents...)
		}
	}

	for _, shelf := range shelves {
		if shelf.MusicShelfRenderer == nil {
			continue
		}
		for _, item := range shelf.MusicShelfRenderer.Contents {
			parsed, _ := item.ParseItem(c.hl)
			if art, ok := parsed.(*Artist); ok {
				artist := *art
				if strings.HasPrefix(artist.ID, "MPLA") {
					artist.ID = artist.ID[4:]
				}
				artists = append(artists, artist)
			}
		}
	}

	return artists, nil
}

/*
GetLikedPlaylists retrieves the list of user-owned and liked playlists.

    params:
          ctx: execution context
    returns:
          []Playlist: list of liked playlists
          error: network or parsing error
*/
func (c *Client) GetLikedPlaylists(ctx context.Context) ([]Playlist, error) {
	return c.GetAccountPlaylists(ctx)
}
