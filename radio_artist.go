// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement ArtistRadio loading.
//
// Key Components:
//   - GetArtistRadio: returns the artist radio playlist by loading playlist ID prefixed with RART
//
// Dependencies:
//   - context
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
)

/*
GetArtistRadio retrieves the artist automix radio playlist.

    params:
          ctx: execution context
          artistID: channel browse ID
          continuation: optional continuation token
    returns:
          *Playlist: artist radio playlist
          error: network or parsing error
*/
func (c *Client) GetArtistRadio(ctx context.Context, artistID string, continuation *BuiltInContinuation) (*Playlist, error) {
	return c.LoadPlaylist(ctx, "RART"+artistID, continuation, nil, nil, false)
}
