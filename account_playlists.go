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

// PlaylistPrivacy controls the visibility of a YouTube Music playlist.
type PlaylistPrivacy string

const (
	PlaylistPrivate  PlaylistPrivacy = "PRIVATE"
	PlaylistUnlisted PlaylistPrivacy = "UNLISTED"
	PlaylistPublic   PlaylistPrivacy = "PUBLIC"
)

// CreatePlaylistOptions configures a new account playlist.
type CreatePlaylistOptions struct {
	Title       string
	Description string
	Privacy     PlaylistPrivacy // empty defaults to PlaylistPrivate
	VideoIDs    []string
}

// DedupeMode controls how InnerTube handles songs already present in a playlist.
type DedupeMode int

const (
	// DedupeAsk refuses the add and returns ErrDuplicates if any track is already in the playlist.
	DedupeAsk DedupeMode = iota
	// DedupeDrop skips tracks already in the playlist and adds only new ones.
	DedupeDrop
	// DedupeAdd skips the duplicate check and appends every track, even if already present.
	DedupeAdd
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
		"browseId": libraryPlaylistsBrowseID,
	}, true, &resp)
	if err != nil {
		return nil, err
	}
	if !c.SignedIn() {
		return nil, ErrSessionExpired
	}

	return parseGridPlaylists(resp.libraryShelves(), c.hl, c.auth), nil
}

func sanitizePlaylistTitle(title string) string {
	return strings.TrimSpace(strings.NewReplacer("<", "", ">", "").Replace(title))
}

// CreatePlaylist creates a new playlist on the signed-in account. Empty Privacy
// defaults to PlaylistPrivate, and angle brackets are stripped from Title.
func (c *Client) CreatePlaylist(ctx context.Context, opts CreatePlaylistOptions) (string, error) {
	if err := requireAuth(c.auth); err != nil {
		return "", err
	}
	title := sanitizePlaylistTitle(opts.Title)
	if title == "" {
		return "", fmt.Errorf("ytm: playlist title is required")
	}
	privacy := opts.Privacy
	if privacy == "" {
		privacy = PlaylistPrivate
	}

	body := map[string]any{
		"title":         title,
		"description":   opts.Description,
		"privacyStatus": string(privacy),
	}
	if len(opts.VideoIDs) > 0 {
		ids := make([]string, 0, len(opts.VideoIDs))
		for _, id := range opts.VideoIDs {
			if cleaned := CleanSongID(strings.TrimSpace(id)); cleaned != "" {
				ids = append(ids, cleaned)
			}
		}
		if len(ids) > 0 {
			body["videoIds"] = ids
		}
	}

	var resp struct {
		writeResponse
		PlaylistID string `json:"playlistId"`
	}
	if err := c.doInnerTube(ctx, "playlist/create", GetContextWebRemix(c.hl), body, true, &resp); err != nil {
		return "", err
	}
	if err := resp.checkStatus(); err != nil {
		return "", err
	}
	if resp.PlaylistID == "" {
		return "", fmt.Errorf("%w: missing playlistId", ErrEditRejected)
	}
	return resp.PlaylistID, nil
}

/*
CreateAccountPlaylist creates a new empty private playlist.

    params:
          ctx: execution context
          title: playlist title
          description: playlist description
    returns:
          string: the created playlist ID
          error: network or rejection error
*/
func (c *Client) CreateAccountPlaylist(ctx context.Context, title, description string) (string, error) {
	return c.CreatePlaylist(ctx, CreatePlaylistOptions{
		Title:       title,
		Description: description,
	})
}

/*
DeleteAccountPlaylist deletes the user-owned playlist with the given ID.

    params:
          ctx: execution context
          playlistID: playlist identifier
    returns:
          error: network or rejection error
*/
func (c *Client) DeleteAccountPlaylist(ctx context.Context, playlistID string) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}

	var resp writeResponse
	if err := c.doInnerTube(ctx, "playlist/delete", GetContextWebRemix(c.hl), map[string]any{
		"playlistId": formatYoutubePlaylistID(playlistID),
	}, true, &resp); err != nil {
		return err
	}
	return resp.checkStatus()
}

func (c *Client) editPlaylistRaw(ctx context.Context, playlistID string, actions []PlaylistEditAction) (writeResponse, error) {
	if err := requireAuth(c.auth); err != nil {
		return writeResponse{}, err
	}

	var resp writeResponse
	err := c.doInnerTube(ctx, "browse/edit_playlist", GetContextWebRemix(c.hl), map[string]any{
		"playlistId": formatYoutubePlaylistID(playlistID),
		"actions":    actions,
	}, true, &resp)
	return resp, err
}

/*
EditPlaylist applies raw commands to mutate playlist settings or elements.

    params:
          ctx: execution context
          playlistID: playlist identifier
          actions: list of edit action commands
    returns:
          error: network or rejection error
*/
func (c *Client) EditPlaylist(ctx context.Context, playlistID string, actions []PlaylistEditAction) error {
	resp, err := c.editPlaylistRaw(ctx, playlistID, actions)
	if err != nil {
		return err
	}
	return resp.checkStatus()
}

// AddSongsToPlaylistOpts adds the specified track IDs to a playlist with the
// chosen duplicate-handling mode.
func (c *Client) AddSongsToPlaylistOpts(ctx context.Context, playlistID string, videoIDs []string, mode DedupeMode) error {
	if err := requireAuth(c.auth); err != nil {
		return err
	}
	var option string
	switch mode {
	case DedupeAsk:
		option = "DEDUPE_OPTION_CHECK"
	case DedupeDrop:
		option = "DEDUPE_OPTION_DROP_DUPLICATE"
	case DedupeAdd:
		option = "DEDUPE_OPTION_SKIP"
	default:
		return fmt.Errorf("ytm: unknown dedupe mode %d", mode)
	}

	var actions []PlaylistEditAction
	for _, vid := range videoIDs {
		if cleaned := CleanSongID(strings.TrimSpace(vid)); cleaned != "" {
			actions = append(actions, PlaylistEditAction{
				Action:       PlaylistEditActionAddVideo,
				AddedVideoID: cleaned,
				DedupeOption: option,
			})
		}
	}
	if len(actions) == 0 {
		return nil
	}

	resp, err := c.editPlaylistRaw(ctx, playlistID, actions)
	if err != nil {
		return err
	}
	if mode == DedupeAsk && resp.asksAboutDuplicates() {
		return ErrDuplicates
	}
	return resp.checkStatus()
}

/*
AddSongsToPlaylist adds the specified track IDs to a playlist without duplicate
checking (DedupeAdd), so tracks already in the playlist can be added again. Prefer
AddSongsToPlaylistOpts when duplicate handling matters.

    params:
          ctx: execution context
          playlistID: target playlist identifier
          videoIDs: tracks to add
    returns:
          error: network or rejection error
*/
func (c *Client) AddSongsToPlaylist(ctx context.Context, playlistID string, videoIDs []string) error {
	return c.AddSongsToPlaylistOpts(ctx, playlistID, videoIDs, DedupeAdd)
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
