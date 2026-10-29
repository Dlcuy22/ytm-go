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
	"fmt"
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
		"browseId": libraryAlbumsBrowseID,
	}, true, &resp)
	if err != nil {
		return nil, err
	}
	if !c.SignedIn() {
		return nil, ErrSessionExpired
	}

	// The page is grid-backed when it has albums and carries an item section with
	// a "No albums yet" message when it does not. Both shapes are tolerated here;
	// the second one simply yields nothing.
	return parseGridPlaylists(resp.libraryShelves(), c.hl, c.auth), nil
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
	if !c.SignedIn() {
		return nil, ErrSessionExpired
	}

	return parseLibraryArtists(resp.libraryShelves(), c.hl), nil
}

/*
GetLikedArtistsContinuation loads the next page of the account's artists.

	params:
	      ctx: execution context
	      token: continuation token from a previous page
	returns:
	      []Artist: the next page
	      *BuiltInContinuation: following page token, or nil on the last page
	      error: network, parsing, or ErrSessionExpired
*/
func (c *Client) GetLikedArtistsContinuation(ctx context.Context, token string) ([]Artist, *BuiltInContinuation, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, nil, err
	}
	if token == "" {
		return nil, nil, fmt.Errorf("ytm: continuation requires a token")
	}

	var resp YoutubeiBrowseResponse
	if err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"ctoken":       token,
		"continuation": token,
	}, true, &resp); err != nil {
		return nil, nil, err
	}

	items, next := resp.continuationItems()
	return parseArtistsFromItems(items, c.hl), next, nil
}

// parseLibraryArtists pulls Artist values out of music shelves.
func parseLibraryArtists(shelves []YoutubeiShelf, hl string) []Artist {
	var artists []Artist
	for _, shelf := range shelves {
		if shelf.MusicShelfRenderer == nil {
			continue
		}
		artists = append(artists, parseArtistsFromItems(shelf.MusicShelfRenderer.Contents, hl)...)
	}
	return artists
}

// parseArtistsFromItems converts rows into artists, stripping the MPLA prefix
// the library adds to artist IDs.
func parseArtistsFromItems(items []YoutubeiShelfContentsItem, hl string) []Artist {
	var artists []Artist
	for _, item := range items {
		parsed, _ := item.ParseItem(hl)
		art, ok := parsed.(*Artist)
		if !ok {
			continue
		}
		artist := *art
		artist.ID = strings.TrimPrefix(artist.ID, "MPLA")
		artists = append(artists, artist)
	}
	return artists
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
