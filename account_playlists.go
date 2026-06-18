// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement account playlist CRUD operations and editing.
//
// Key Components:
//   - GetAccountPlaylists: retrieves the playlists owned by the authenticated user
//   - CreateAccountPlaylist: creates a new empty playlist
//   - DeleteAccountPlaylist: deletes a user-owned playlist
//   - EditPlaylist: applies a list of raw playlist edit actions
//   - AddSongsToPlaylist: adds a list of video IDs to a playlist
//   - PlaylistEditor: stateful wrapper tracking playlist contents during updates
//
// Dependencies:
//   - context
//   - fmt
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

// PlaylistEditActionType describes InnerTube edit actions.
type PlaylistEditActionType string

const (
	PlaylistEditActionSetName        PlaylistEditActionType = "ACTION_SET_PLAYLIST_NAME"
	PlaylistEditActionSetDescription PlaylistEditActionType = "ACTION_SET_PLAYLIST_DESCRIPTION"
	PlaylistEditActionAddVideo       PlaylistEditActionType = "ACTION_ADD_VIDEO"
	PlaylistEditActionMoveVideo      PlaylistEditActionType = "ACTION_MOVE_VIDEO_BEFORE"
	PlaylistEditActionRemoveVideo    PlaylistEditActionType = "ACTION_REMOVE_VIDEO"
)

// PlaylistEditAction represents a raw payload command for /browse/edit_playlist.
type PlaylistEditAction struct {
	Action                   PlaylistEditActionType `json:"action"`
	PlaylistName             string                 `json:"playlistName,omitempty"`
	PlaylistDescription      string                 `json:"playlistDescription,omitempty"`
	AddedVideoID             string                 `json:"addedVideoId,omitempty"`
	DedupeOption             string                 `json:"dedupeOption,omitempty"`
	SetVideoID               string                 `json:"setVideoId,omitempty"`
	MovedSetVideoIDSuccessor string                 `json:"movedSetVideoIdSuccessor,omitempty"`
	RemovedVideoID           string                 `json:"removedVideoId,omitempty"`
}

// PlaylistEditorAction is the interface for stateful playlist edit commands.
type PlaylistEditorAction interface {
	isPlaylistEditorAction()
}

// EditorSetTitle updates the playlist title.
type EditorSetTitle struct{ Title string }

func (EditorSetTitle) isPlaylistEditorAction() {}

// EditorSetDescription updates the playlist description.
type EditorSetDescription struct{ Description string }

func (EditorSetDescription) isPlaylistEditorAction() {}

// EditorAdd adds a song to the playlist.
type EditorAdd struct {
	SongID string
}

func (EditorAdd) isPlaylistEditorAction() {}

// EditorRemove removes a song by index.
type EditorRemove struct{ Index int }

func (EditorRemove) isPlaylistEditorAction() {}

// EditorMove moves a song within the playlist.
type EditorMove struct{ From, To int }

func (EditorMove) isPlaylistEditorAction() {}

// PlaylistEditor tracks playlist elements and updates them.
type PlaylistEditor struct {
	client     *Client
	playlistID string
	itemIDs    []string
	itemSetIDs []string
}

/*
GetAccountPlaylists retrieves the playlists owned by the authenticated user.

    params:
          ctx: execution context
    returns:
          []Playlist: list of user playlists
          error: network or parsing error
*/
func (c *Client) GetAccountPlaylists(ctx context.Context) ([]Playlist, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, err
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": "FEmusic_liked_playlists",
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
			if item.MusicTwoRowItemRenderer == nil || item.MusicTwoRowItemRenderer.NavigationEndpoint.BrowseEndpoint == nil {
				continue
			}

			parsed, _ := item.ParseItem(c.hl)
			if pl, ok := parsed.(*Playlist); ok {
				playlist := *pl

				isOwned := playlist.ID == "VLLM" || playlist.ID == "LM"
				if !isOwned && item.MusicTwoRowItemRenderer.Menu != nil {
					for _, mi := range item.MusicTwoRowItemRenderer.Menu.MenuRenderer.Items {
						if mi.MenuNavigationItemRenderer != nil && mi.MenuNavigationItemRenderer.Icon.IconType == "DELETE" {
							isOwned = true
							break
						}
					}
				}
				if isOwned && c.auth != nil {
					playlist.OwnerID = c.auth.ChannelID
				}

				playlists = append(playlists, playlist)
			}
		}
	}

	return playlists, nil
}

/*
CreateAccountPlaylist creates a new empty playlist.

    params:
          ctx: execution context
          title: playlist title
          description: playlist description
    returns:
          string: the created playlist ID
          error: network error
*/
func (c *Client) CreateAccountPlaylist(ctx context.Context, title, description string) (string, error) {
	if err := requireAuth(c.auth); err != nil {
		return "", err
	}

	var resp struct {
		PlaylistID string `json:"playlistId"`
	}
	err := c.doInnerTube(ctx, "playlist/create", GetContextWebRemix(c.hl), map[string]any{
		"title":       title,
		"description": description,
	}, true, &resp)
	if err != nil {
		return "", err
	}
	return resp.PlaylistID, nil
}

/*
DeleteAccountPlaylist deletes the user-owned playlist with the given ID.

    params:
          ctx: execution context
          playlistID: playlist identifier
    returns:
          error: network error
*/
func (c *Client) DeleteAccountPlaylist(ctx context.Context, playlistID string) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}

	var resp struct{}
	return c.doInnerTube(ctx, "playlist/delete", GetContextWebRemix(c.hl), map[string]any{
		"playlistId": formatYoutubePlaylistID(playlistID),
	}, true, &resp)
}

/*
EditPlaylist applies raw commands to mutate playlist settings or elements.

    params:
          ctx: execution context
          playlistID: playlist identifier
          actions: list of edit action commands
    returns:
          error: network error
*/
func (c *Client) EditPlaylist(ctx context.Context, playlistID string, actions []PlaylistEditAction) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}

	var resp struct{}
	return c.doInnerTube(ctx, "browse/edit_playlist", GetContextWebRemix(c.hl), map[string]any{
		"playlistId": formatYoutubePlaylistID(playlistID),
		"actions":    actions,
	}, true, &resp)
}

/*
AddSongsToPlaylist adds the specified track IDs to a playlist.

    params:
          ctx: execution context
          playlistID: target playlist identifier
          videoIDs: tracks to add
    returns:
          error: network error
*/
func (c *Client) AddSongsToPlaylist(ctx context.Context, playlistID string, videoIDs []string) error {
	var actions []PlaylistEditAction
	for _, vid := range videoIDs {
		actions = append(actions, PlaylistEditAction{
			Action:       PlaylistEditActionAddVideo,
			AddedVideoID: vid,
			DedupeOption: "DEDUPE_OPTION_SKIP",
		})
	}
	return c.EditPlaylist(ctx, playlistID, actions)
}

/*
GetPlaylistEditor returns an editor instance for a playlist.

    params:
          playlistID: ID of the playlist
          itemIDs: ordered list of media item IDs in the playlist
          itemSetIDs: ordered list of item set IDs in the playlist
    returns:
          *PlaylistEditor: the playlist editor instance
*/
func (c *Client) GetPlaylistEditor(playlistID string, itemIDs, itemSetIDs []string) *PlaylistEditor {
	return &PlaylistEditor{
		client:     c,
		playlistID: playlistID,
		itemIDs:    append([]string(nil), itemIDs...),
		itemSetIDs: append([]string(nil), itemSetIDs...),
	}
}

/*
PerformAndCommitActions runs a sequence of changes against the playlist.

    params:
          ctx: execution context
          actions: slice of actions to apply
    returns:
          error: network error or invalid index error
*/
func (e *PlaylistEditor) PerformAndCommitActions(ctx context.Context, actions []PlaylistEditorAction) error {
	if len(actions) == 0 {
		return nil
	}

	var editActions []PlaylistEditAction
	for _, action := range actions {
		switch a := action.(type) {
		case EditorSetTitle:
			editActions = append(editActions, PlaylistEditAction{
				Action:       PlaylistEditActionSetName,
				PlaylistName: a.Title,
			})
		case EditorSetDescription:
			editActions = append(editActions, PlaylistEditAction{
				Action:              PlaylistEditActionSetDescription,
				PlaylistDescription: a.Description,
			})
		case EditorAdd:
			editActions = append(editActions, PlaylistEditAction{
				Action:       PlaylistEditActionAddVideo,
				AddedVideoID: a.SongID,
				DedupeOption: "DEDUPE_OPTION_SKIP",
			})
		case EditorMove:
			if a.From == a.To {
				continue
			}
			if a.From < 0 || a.From >= len(e.itemSetIDs) {
				return fmt.Errorf("invalid move from index: %d", a.From)
			}
			setVideoID := e.itemSetIDs[a.From]
			successorIdx := a.To
			if a.To > a.From {
				successorIdx = a.To + 1
			}
			successorSetVideoID := ""
			if successorIdx >= 0 && successorIdx < len(e.itemSetIDs) {
				successorSetVideoID = e.itemSetIDs[successorIdx]
			}
			editActions = append(editActions, PlaylistEditAction{
				Action:                   PlaylistEditActionMoveVideo,
				SetVideoID:               setVideoID,
				MovedSetVideoIDSuccessor: successorSetVideoID,
			})
		case EditorRemove:
			if a.Index < 0 || a.Index >= len(e.itemIDs) {
				return fmt.Errorf("invalid remove index: %d", a.Index)
			}
			editActions = append(editActions, PlaylistEditAction{
				Action:         PlaylistEditActionRemoveVideo,
				RemovedVideoID: e.itemIDs[a.Index],
				SetVideoID:     e.itemSetIDs[a.Index],
			})
		}
	}

	err := e.client.EditPlaylist(ctx, e.playlistID, editActions)
	if err != nil {
		return err
	}

	for _, action := range actions {
		switch a := action.(type) {
		case EditorAdd:
			e.itemIDs = append(e.itemIDs, a.SongID)
		case EditorMove:
			if a.From == a.To {
				continue
			}
			tmpID := e.itemIDs[a.From]
			e.itemIDs = append(e.itemIDs[:a.From], e.itemIDs[a.From+1:]...)
			e.itemIDs = insertStringSlice(e.itemIDs, tmpID, a.To)

			tmpSetID := e.itemSetIDs[a.From]
			e.itemSetIDs = append(e.itemSetIDs[:a.From], e.itemSetIDs[a.From+1:]...)
			e.itemSetIDs = insertStringSlice(e.itemSetIDs, tmpSetID, a.To)
		case EditorRemove:
			e.itemIDs = append(e.itemIDs[:a.Index], e.itemIDs[a.Index+1:]...)
			e.itemSetIDs = append(e.itemSetIDs[:a.Index], e.itemSetIDs[a.Index+1:]...)
		}
	}

	return nil
}

func formatYoutubePlaylistID(id string) string {
	return strings.TrimPrefix(id, "VL")
}

func insertStringSlice(slice []string, val string, idx int) []string {
	if idx < 0 {
		idx = 0
	}
	if idx > len(slice) {
		idx = len(slice)
	}
	slice = append(slice, "")
	copy(slice[idx+1:], slice[idx:])
	slice[idx] = val
	return slice
}
