// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define song, artist, playlist, and layout structures used to interact
//   with InnerTube API endpoints.
//
// Key Components:
//   - Song: Model representing a YouTube Music track
//   - Artist: Model representing a music creator or channel
//   - Playlist: Model representing a compilation of tracks (including albums and radios)
//   - ArtistLayout: Visual sections returned in artist browse details
//
// Dependencies:
//   - None
//
// Error Types:
//   - None
//
package ytm

import (
	"strings"
)

// SongType categorizes track media types.
type SongType string

const (
	SongTypeSong    SongType = "SONG"
	SongTypeVideo   SongType = "VIDEO"
	SongTypePodcast SongType = "PODCAST"
)

// CleanSongID removes common song prefixes like MPED.
func CleanSongID(id string) string {
	return strings.TrimPrefix(id, "MPED")
}

// Song represents a YouTube Music track.
type Song struct {
	ID              string             `json:"id"`
	Name            string             `json:"name,omitempty"`
	Description     string             `json:"description,omitempty"`
	Thumbnail       *ThumbnailProvider `json:"thumbnail,omitempty"`
	Artists         []Artist           `json:"artists,omitempty"`
	Type            SongType           `json:"type,omitempty"`
	IsExplicit      bool               `json:"is_explicit"`
	Album           *Playlist          `json:"album,omitempty"`
	DurationMs      int64              `json:"duration_ms,omitempty"`
	RelatedBrowseID string             `json:"related_browse_id,omitempty"`
	LyricsBrowseID  string             `json:"lyrics_browse_id,omitempty"`
}

func (s Song) GetID() string {
	return s.ID
}

func (s Song) GetName() string {
	return s.Name
}

func (s Song) GetThumbnailProvider() *ThumbnailProvider {
	return s.Thumbnail
}

// Artist represents a music artist or channel.
type Artist struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name,omitempty"`
	Description        string             `json:"description,omitempty"`
	Thumbnail          *ThumbnailProvider `json:"thumbnail,omitempty"`
	ShufflePlaylistID  string             `json:"shuffle_playlist_id,omitempty"`
	Layouts            []ArtistLayout     `json:"layouts,omitempty"`
	SubscribeChannelID string             `json:"subscribe_channel_id,omitempty"`
	SubscriberCount    int                `json:"subscriber_count,omitempty"`
	Subscribed         bool               `json:"subscribed"`
}

func (a Artist) GetID() string {
	return a.ID
}

func (a Artist) GetName() string {
	return a.Name
}

func (a Artist) GetThumbnailProvider() *ThumbnailProvider {
	return a.Thumbnail
}

// ArtistLayout represents components in an artist browse overview.
type ArtistLayout struct {
	Items      []MediaItem        `json:"items,omitempty"`
	Title      string             `json:"title,omitempty"`
	Subtitle   string             `json:"subtitle,omitempty"`
	Type       string             `json:"type,omitempty"`
	ViewMore   *PageRef           `json:"view_more,omitempty"`
	PlaylistID string             `json:"playlist_id,omitempty"`
}

// PageRef holds instructions to browse deeper into a resource.
type PageRef struct {
	BrowseID     string `json:"browse_id"`
	BrowseParams string `json:"browse_params,omitempty"`
}

// PlaylistType categorizes playlist structures.
type PlaylistType string

const (
	PlaylistTypePlaylist  PlaylistType = "PLAYLIST"
	PlaylistTypeAlbum     PlaylistType = "ALBUM"
	PlaylistTypeAudiobook PlaylistType = "AUDIOBOOK"
	PlaylistTypePodcast   PlaylistType = "PODCAST"
	PlaylistTypeRadio     PlaylistType = "RADIO"
)

// CleanPlaylistID removes common playlist prefixes like MPSP.
func CleanPlaylistID(id string) string {
	return strings.TrimPrefix(id, "MPSP")
}

// Playlist represents a collection of music tracks.
type Playlist struct {
	ID              string               `json:"id"`
	Name            string               `json:"name,omitempty"`
	Description     string               `json:"description,omitempty"`
	Thumbnail       *ThumbnailProvider   `json:"thumbnail,omitempty"`
	Type            PlaylistType         `json:"type,omitempty"`
	Artists         []Artist             `json:"artists,omitempty"`
	Year            int                  `json:"year,omitempty"`
	Items           []Song               `json:"items,omitempty"`
	OwnerID         string               `json:"owner_id,omitempty"`
	Continuation    *BuiltInContinuation `json:"continuation,omitempty"`
	ItemSetIDs      []string             `json:"item_set_ids,omitempty"`
	ItemCount       int                  `json:"item_count,omitempty"`
	TotalDurationMs int64                `json:"total_duration_ms,omitempty"`
	PlaylistURL     string               `json:"playlist_url,omitempty"`
}

func (p Playlist) GetID() string {
	return p.ID
}

func (p Playlist) GetName() string {
	return p.Name
}

func (p Playlist) GetThumbnailProvider() *ThumbnailProvider {
	return p.Thumbnail
}

// ContinuationType categorizes standard continuation actions.
type ContinuationType string

const (
	ContinuationSong          ContinuationType = "SONG"
	ContinuationPlaylist      ContinuationType = "PLAYLIST"
	ContinuationPlaylistInit  ContinuationType = "PLAYLIST_INITIAL"
	ContinuationArtistShuffle ContinuationType = "ARTIST_SHUFFLE"
)

// BuiltInContinuation contains parameters needed to continue loading lists or radios.
type BuiltInContinuation struct {
	Token           string           `json:"token"`
	Type            ContinuationType `json:"type"`
	ItemID          string           `json:"item_id,omitempty"`
	PlaylistSkipAmt int              `json:"playlist_skip_amount"`
}
