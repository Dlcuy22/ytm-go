// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement in-memory cache layer to avoid duplicate InnerTube details fetching.
//
// Key Components:
//   - ItemCache: thread-safe container storing tracks, artists, and playlists
//   - GetOrLoadSong / GetOrLoadArtist / GetOrLoadPlaylist: helper loaders with cache fallbacks
//
// Dependencies:
//   - context
//   - sync
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"sync"
)

// ItemCache stores retrieved metadata items.
type ItemCache struct {
	songs     sync.Map
	artists   sync.Map
	playlists sync.Map
}

/*
NewItemCache initializes a blank ItemCache.

    returns:
          *ItemCache: newly allocated cache instance
*/
func NewItemCache() *ItemCache {
	return &ItemCache{}
}

/*
StoreSong saves the song details in memory.

    params:
          s: song model pointer
*/
func (c *ItemCache) StoreSong(s *Song) {
	if s != nil && s.ID != "" {
		c.songs.Store(s.ID, s)
	}
}

/*
StoreArtist saves the artist details in memory.

    params:
          a: artist model pointer
*/
func (c *ItemCache) StoreArtist(a *Artist) {
	if a != nil && a.ID != "" {
		c.artists.Store(a.ID, a)
	}
}

/*
StorePlaylist saves the playlist details in memory.

    params:
          p: playlist model pointer
*/
func (c *ItemCache) StorePlaylist(p *Playlist) {
	if p != nil && p.ID != "" {
		c.playlists.Store(p.ID, p)
	}
}

/*
GetSong retrieves a song by ID from cache.

    params:
          id: YouTube video ID
    returns:
          *Song: song model if found, nil otherwise
          bool: true if found, false otherwise
*/
func (c *ItemCache) GetSong(id string) (*Song, bool) {
	val, ok := c.songs.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Song), true
}

/*
GetArtist retrieves an artist by ID from cache.

    params:
          id: artist channel ID
    returns:
          *Artist: artist model if found, nil otherwise
          bool: true if found, false otherwise
*/
func (c *ItemCache) GetArtist(id string) (*Artist, bool) {
	val, ok := c.artists.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Artist), true
}

/*
GetPlaylist retrieves a playlist by ID from cache.

    params:
          id: playlist browse ID
    returns:
          *Playlist: playlist model if found, nil otherwise
          bool: true if found, false otherwise
*/
func (c *ItemCache) GetPlaylist(id string) (*Playlist, bool) {
	val, ok := c.playlists.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Playlist), true
}

/*
GetOrLoadSong attempts retrieval from cache before fetching from remote.

    params:
          ctx: execution context
          client: API connection client
          id: YouTube video ID
    returns:
          *Song: retrieved song details
          error: network or parsing error
*/
func (c *ItemCache) GetOrLoadSong(ctx context.Context, client *Client, id string) (*Song, error) {
	if song, ok := c.GetSong(id); ok {
		return song, nil
	}
	return client.LoadSong(ctx, id)
}

/*
GetOrLoadArtist attempts retrieval from cache before fetching from remote.

    params:
          ctx: execution context
          client: API connection client
          id: artist channel ID
    returns:
          *Artist: retrieved artist details
          error: network or parsing error
*/
func (c *ItemCache) GetOrLoadArtist(ctx context.Context, client *Client, id string) (*Artist, error) {
	if artist, ok := c.GetArtist(id); ok {
		return artist, nil
	}
	return client.LoadArtist(ctx, id)
}

/*
GetOrLoadPlaylist attempts retrieval from cache before fetching from remote.

    params:
          ctx: execution context
          client: API connection client
          id: playlist browse ID
    returns:
          *Playlist: retrieved playlist details
          error: network or parsing error
*/
func (c *ItemCache) GetOrLoadPlaylist(ctx context.Context, client *Client, id string) (*Playlist, error) {
	if playlist, ok := c.GetPlaylist(id); ok {
		return playlist, nil
	}
	return client.LoadPlaylist(ctx, id, nil, nil, nil, false)
}
