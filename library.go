// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement the account library surface: the liked-songs list, and adding or
//   removing a song from the library.
//
// Key Components:
//   - GetLikedSongs / GetLikedSongsContinuation: FEmusic_liked_videos paging
//   - SetSongInLibrary: toggles library membership via /feedback tokens
//   - SendFeedback: the low-level token submission
//   - libraryToggleTokens: pulls the add/remove tokens out of a /next menu
//
// Dependencies:
//   - context
//   - fmt
//
// Error Types:
//   - ErrLoginRequired: returned when the action is attempted without credentials
//   - ErrFeedbackTokenMissing: the /next response carried no library toggle
//
package ytm

import (
	"context"
	"errors"
	"fmt"
)

// ErrFeedbackTokenMissing means the /next response did not contain the library
// toggle the caller asked to act on. It is distinct from a network failure: the
// track may be unplayable, or the account may not be allowed to save it.
var ErrFeedbackTokenMissing = errors.New("ytm: library feedback token not found")

// libraryBrowseID is the browse target for the account's liked songs. It is
// the list behind the "Liked songs" row, not the LM playlist: both exist and
// both work, but this one is a browse shelf and pages the same way the rest of
// the library does.
const libraryBrowseID = "FEmusic_liked_videos"

/*
GetLikedSongs retrieves the first page of songs in the user's library.

    params:
          ctx: execution context
    returns:
          []Song: liked songs, in library order
          *BuiltInContinuation: next page token, or nil on the last page
          error: network or parsing error
*/
func (c *Client) GetLikedSongs(ctx context.Context) ([]Song, *BuiltInContinuation, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, nil, err
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": libraryBrowseID,
	}, true, &resp)
	if err != nil {
		return nil, nil, err
	}

	songs := parseLibrarySongs(resp.libraryShelves(), c.hl)
	if !c.SignedIn() {
		return nil, nil, ErrSessionExpired
	}
	return songs, resp.libraryContinuation(), nil
}

/*
GetLikedSongsContinuation loads the next page of liked songs.

    params:
          ctx: execution context
          token: continuation token from a previous page
    returns:
          []Song: the next page of liked songs
          *BuiltInContinuation: following page token, or nil on the last page
          error: network or parsing error
*/
func (c *Client) GetLikedSongsContinuation(ctx context.Context, token string) ([]Song, *BuiltInContinuation, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, nil, err
	}
	if token == "" {
		return nil, nil, fmt.Errorf("ytm: continuation requires a token")
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"ctoken":       token,
		"continuation": token,
	}, true, &resp)
	if err != nil {
		return nil, nil, err
	}

	var shelf *MusicShelfRenderer
	if resp.ContinuationContents != nil {
		shelf = resp.ContinuationContents.MusicShelfContinuation
	}
	if shelf == nil {
		return nil, nil, nil
	}

	songs := parseLibrarySongs([]YoutubeiShelf{{MusicShelfRenderer: shelf}}, c.hl)
	return songs, shelfContinuation(shelf), nil
}

// librarySectionList returns the first tab's section list, tolerating both the
// single-column and two-column layouts the endpoints alternate between.
//
// The two-column branch matters: reading SingleColumnBrowseResultsRenderer.Tabs
// without a nil check panics on a response that only has the two-column form.
func (br *YoutubeiBrowseResponse) librarySectionList() *SectionListRenderer {
	if br == nil || br.Contents == nil {
		return nil
	}
	var tabs []Tab
	if single := br.Contents.SingleColumnBrowseResultsRenderer; single != nil {
		tabs = single.Tabs
	}
	if len(tabs) == 0 && br.Contents.TwoColumnBrowseResultsRenderer != nil {
		tabs = br.Contents.TwoColumnBrowseResultsRenderer.Tabs
	}
	if len(tabs) == 0 {
		return nil
	}
	content := tabs[0].TabRenderer.Content
	if content == nil {
		return nil
	}
	return content.SectionListRenderer
}

// libraryShelves returns the shelves of the first tab.
func (br *YoutubeiBrowseResponse) libraryShelves() []YoutubeiShelf {
	if sl := br.librarySectionList(); sl != nil {
		return sl.Contents
	}
	return nil
}

// libraryContinuation reads the paging token for a library page. The token sits
// on the shelf for a music shelf and on the section list or the grid itself for
// the grid-backed pages, so all three places are checked.
func (br *YoutubeiBrowseResponse) libraryContinuation() *BuiltInContinuation {
	sl := br.librarySectionList()
	if sl == nil {
		return nil
	}
	for _, shelf := range sl.Contents {
		if shelf.MusicShelfRenderer != nil {
			if cont := shelfContinuation(shelf.MusicShelfRenderer); cont != nil {
				return cont
			}
		}
		if shelf.GridRenderer != nil {
			if cont := gridContinuation(shelf.GridRenderer); cont != nil {
				return cont
			}
		}
	}
	for _, c := range sl.Continuations {
		if token := c.GetToken(); token != "" {
			return &BuiltInContinuation{Token: token, Type: ContinuationPlaylist}
		}
	}
	return nil
}

func shelfContinuation(shelf *MusicShelfRenderer) *BuiltInContinuation {
	for _, c := range shelf.Continuations {
		if token := c.GetToken(); token != "" {
			return &BuiltInContinuation{Token: token, Type: ContinuationPlaylist}
		}
	}
	return nil
}

func gridContinuation(grid *GridRenderer) *BuiltInContinuation {
	for _, c := range grid.Continuations {
		if token := c.GetToken(); token != "" {
			return &BuiltInContinuation{Token: token, Type: ContinuationPlaylist}
		}
	}
	return nil
}

// parseLibrarySongs pulls the Song values out of library shelves.
func parseLibrarySongs(shelves []YoutubeiShelf, hl string) []Song {
	var songs []Song
	for _, shelf := range shelves {
		if shelf.MusicShelfRenderer == nil {
			continue
		}
		for _, item := range shelf.MusicShelfRenderer.Contents {
			parsed, _ := item.ParseItem(hl)
			if song, ok := parsed.(*Song); ok {
				songs = append(songs, *song)
			}
		}
	}
	return songs
}

/*
SetSongInLibrary adds or removes a song from the account library.

The endpoint does not take a video ID for this operation: it takes a feedback
token that a /next response mints for that track and that account. So the call
is two round trips: read the queue entry's library toggle, then submit the token
matching the requested direction.

If the track is already in the requested state the call is a no-op and returns
nil, which makes it safe to call idempotently.

    params:
          ctx: execution context
          songID: YouTube track ID
          inLibrary: true to save, false to remove
    returns:
          error: network, parsing, or ErrFeedbackTokenMissing
*/
func (c *Client) SetSongInLibrary(ctx context.Context, songID string, inLibrary bool) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}
	songID = CleanSongID(songID)

	add, remove, isToggled, err := c.libraryToggleTokens(ctx, songID)
	if err != nil {
		return err
	}

	token := add
	switch {
	case inLibrary && isToggled:
		// Already saved; nothing to do.
		return nil
	case inLibrary:
		token = add
	case isToggled:
		token = remove
	default:
		// Already absent; nothing to do.
		return nil
	}
	if token == "" {
		return fmt.Errorf("%w for %s", ErrFeedbackTokenMissing, songID)
	}

	return c.SendFeedback(ctx, token)
}

// libraryToggleTokens reads the library toggle for one track out of /next.
//
// It returns the token that saves, the token that removes, and whether the
// track is currently saved. A track with no toggle at all (unplayable, or not
// savable) yields ErrFeedbackTokenMissing.
func (c *Client) libraryToggleTokens(ctx context.Context, songID string) (add, remove string, isToggled bool, err error) {
	var resp YoutubeiNextResponse
	err = c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), map[string]any{
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"videoId":                       songID,
	}, true, &resp)
	if err != nil {
		return "", "", false, err
	}

	defer func() {
		if r := recover(); r != nil {
			c.logPanic("libraryToggleTokens", r)
		}
	}()
	tabs := resp.Contents.SingleColumnMusicWatchNextResultsRenderer.TabbedRenderer.WatchNextTabbedResultsRenderer.Tabs
	if len(tabs) == 0 {
		return "", "", false, fmt.Errorf("%w for %s", ErrFeedbackTokenMissing, songID)
	}
	queue := tabs[0].TabRenderer.Content.MusicQueueRenderer
	if queue == nil || queue.Content == nil {
		return "", "", false, fmt.Errorf("%w for %s", ErrFeedbackTokenMissing, songID)
	}

	for _, item := range queue.Content.PlaylistPanelRenderer.Contents {
		video := item.GetRenderer()
		if video == nil || CleanSongID(video.VideoID) != songID {
			continue
		}
		add, remove, isToggled = video.LibraryToggle()
		if add == "" && remove == "" {
			return "", "", false, fmt.Errorf("%w for %s", ErrFeedbackTokenMissing, songID)
		}
		return add, remove, isToggled, nil
	}

	return "", "", false, fmt.Errorf("%w for %s", ErrFeedbackTokenMissing, songID)
}

/*
SendFeedback submits one or more feedback tokens.

It is exported because tokens are minted for more than library membership:
liking, pinning, and other menu actions all ride the same endpoint. Callers who
want a specific action should prefer the named helper, which reads the token for
that action from a fresh /next response.

    params:
          ctx: execution context
          tokens: feedback tokens to submit together
    returns:
          error: network or parsing error
*/
func (c *Client) SendFeedback(ctx context.Context, tokens ...string) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("ytm: SendFeedback requires at least one token")
	}

	var resp struct {
		FeedbackResponses []struct {
			IsProcessed bool `json:"isProcessed"`
		} `json:"feedbackResponses"`
	}
	err := c.doInnerTube(ctx, "feedback", GetContextWebRemix(c.hl), map[string]any{
		"feedbackTokens":             tokens,
		"isFeedbackTokenUnencrypted": false,
		"shouldMerge":                false,
	}, true, &resp)
	if err != nil {
		return err
	}

	// The endpoint answers 200 even when it refuses a token, so a present
	// response with isProcessed false is the only refusal signal available.
	for _, r := range resp.FeedbackResponses {
		if !r.IsProcessed {
			return fmt.Errorf("ytm: feedback token was rejected")
		}
	}
	return nil
}

// LibraryToggle returns the add and remove feedback tokens for a queue entry,
// and whether the track is currently saved.
func (vr PlaylistPanelVideoRenderer) LibraryToggle() (add, remove string, isToggled bool) {
	for _, item := range vr.Menu.MenuRenderer.Items {
		toggle := item.ToggleMenuServiceItemRenderer
		if toggle == nil {
			continue
		}
		if !isLibraryToggleText(toggle.DefaultText, toggle.ToggledText) {
			continue
		}
		return toggle.DefaultServiceEndpoint.FeedbackToken(),
			toggle.ToggledServiceEndpoint.FeedbackToken(),
			toggle.IsToggled
	}
	return "", "", false
}

// isLibraryToggleText recognises the library toggle by its labels rather than
// by menu position, because the menu also carries pin and like toggles.
func isLibraryToggleText(defaultText, toggledText TextRuns) bool {
	return defaultText.FirstText() == "Save to library" &&
		toggledText.FirstText() == "Remove from library"
}

// The two library pages that list playlists and albums. Both are grid-backed and
// page the grid, unlike the liked-songs page which pages a music shelf.
const (
	libraryPlaylistsBrowseID = "FEmusic_liked_playlists"
	libraryAlbumsBrowseID    = "FEmusic_liked_albums"
)

/*
GetLibraryPlaylists retrieves the first page of the account's playlists.

This is the paging form of GetAccountPlaylists. The two read the same page; this
one also returns the token for the next one.

	params:
	      ctx: execution context
	returns:
	      []Playlist: playlists, in library order
	      *BuiltInContinuation: next page token, or nil on the last page
	      error: network, parsing, or ErrSessionExpired
*/
func (c *Client) GetLibraryPlaylists(ctx context.Context) ([]Playlist, *BuiltInContinuation, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, nil, err
	}

	var resp YoutubeiBrowseResponse
	if err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": libraryPlaylistsBrowseID,
	}, true, &resp); err != nil {
		return nil, nil, err
	}
	if !c.SignedIn() {
		return nil, nil, ErrSessionExpired
	}

	return parseGridPlaylists(resp.libraryShelves(), c.hl, c.auth), resp.libraryContinuation(), nil
}

/*
GetLibraryPlaylistsContinuation loads the next page of the account's playlists.

	params:
	      ctx: execution context
	      token: continuation token from a previous page
	returns:
	      []Playlist: the next page
	      *BuiltInContinuation: following page token, or nil on the last page
	      error: network, parsing, or ErrSessionExpired
*/
func (c *Client) GetLibraryPlaylistsContinuation(ctx context.Context, token string) ([]Playlist, *BuiltInContinuation, error) {
	return c.gridPlaylistContinuation(ctx, token)
}

/*
GetLikedAlbumsContinuation loads the next page of the account's liked albums.

	params:
	      ctx: execution context
	      token: continuation token from a previous page
	returns:
	      []Playlist: the next page
	      *BuiltInContinuation: following page token, or nil on the last page
	      error: network, parsing, or ErrSessionExpired
*/
func (c *Client) GetLikedAlbumsContinuation(ctx context.Context, token string) ([]Playlist, *BuiltInContinuation, error) {
	return c.gridPlaylistContinuation(ctx, token)
}

// gridPlaylistContinuation is the shared body of the two grid paging calls. They
// differ only in which page they follow, and the server's continuation request
// carries the token alone.
func (c *Client) gridPlaylistContinuation(ctx context.Context, token string) ([]Playlist, *BuiltInContinuation, error) {
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
	return parsePlaylistItems(items, c.hl, c.auth), next, nil
}

// continuationItems collects the items and the next token out of a continuation
// response, whichever of the three shapes the server picked.
func (br *YoutubeiBrowseResponse) continuationItems() ([]YoutubeiShelfContentsItem, *BuiltInContinuation) {
	if br.ContinuationContents != nil {
		if grid := br.ContinuationContents.GridContinuation; grid != nil {
			return grid.Items, gridContinuation(grid)
		}
		if shelf := br.ContinuationContents.MusicShelfContinuation; shelf != nil {
			return shelf.Contents, shelfContinuation(shelf)
		}
		if shelf := br.ContinuationContents.MusicPlaylistShelfContinuation; shelf != nil {
			return shelf.Contents, shelfContinuation(shelf)
		}
	}

	for _, action := range br.OnResponseReceivedActions {
		if action.AppendContinuationItemsAction == nil {
			continue
		}
		items := action.AppendContinuationItemsAction.ContinuationItems
		var out []YoutubeiShelfContentsItem
		var next *BuiltInContinuation
		for _, item := range items {
			if item.ContinuationItemRenderer != nil {
				next = &BuiltInContinuation{
					Token: item.ContinuationItemRenderer.ContinuationEndpoint.ContinuationCommand.Token,
					Type:  ContinuationPlaylist,
				}
				continue
			}
			out = append(out, item)
		}
		return out, next
	}
	return nil, nil
}

// parseGridPlaylists pulls playlists out of grid shelves.
func parseGridPlaylists(shelves []YoutubeiShelf, hl string, auth *AuthState) []Playlist {
	var playlists []Playlist
	for _, shelf := range shelves {
		if shelf.GridRenderer == nil {
			continue
		}
		playlists = append(playlists, parsePlaylistItems(shelf.GridRenderer.Items, hl, auth)...)
	}
	return playlists
}

// parsePlaylistItems converts grid items into playlists, marking the ones the
// account owns so OwnerID can be filled from the session channel.
func parsePlaylistItems(items []YoutubeiShelfContentsItem, hl string, auth *AuthState) []Playlist {
	var playlists []Playlist
	for _, item := range items {
		if item.MusicTwoRowItemRenderer == nil || item.MusicTwoRowItemRenderer.NavigationEndpoint.BrowseEndpoint == nil {
			continue
		}

		parsed, _ := item.ParseItem(hl)
		pl, ok := parsed.(*Playlist)
		if !ok {
			continue
		}
		playlist := *pl

		if isOwnedPlaylist(playlist.ID, item) && auth != nil {
			playlist.OwnerID = auth.ChannelID
		}
		playlists = append(playlists, playlist)
	}
	return playlists
}

// isOwnedPlaylist decides ownership by the built-in IDs or by the presence of a
// delete action, which only an owned playlist offers.
func isOwnedPlaylist(playlistID string, item YoutubeiShelfContentsItem) bool {
	if playlistID == "VLLM" || playlistID == "LM" {
		return true
	}
	if item.MusicTwoRowItemRenderer == nil || item.MusicTwoRowItemRenderer.Menu == nil {
		return false
	}
	for _, mi := range item.MusicTwoRowItemRenderer.Menu.MenuRenderer.Items {
		if mi.MenuNavigationItemRenderer != nil && mi.MenuNavigationItemRenderer.Icon.IconType == "DELETE" {
			return true
		}
	}
	return false
}
