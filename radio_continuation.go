// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement radio and queue continuation loading.
//
// Key Components:
//   - LoadContinuation: resolves the next list of media items based on the continuation parameters
//
// Dependencies:
//   - context
//   - fmt
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"fmt"
)

/*
LoadContinuation dispatches request to resolve items for any continuation token.

    params:
          ctx: execution context
          client: client orchestrator
          filters: optional list of tuner filters
    returns:
          []MediaItem: resolved continuation items
          *BuiltInContinuation: next page continuation token
          error: network or parsing error
*/
func (bc *BuiltInContinuation) LoadContinuation(ctx context.Context, client *Client, filters []RadioBuilderModifier) ([]MediaItem, *BuiltInContinuation, error) {
	switch bc.Type {
	case ContinuationSong:
		radio, err := client.GetSongRadio(ctx, bc.ItemID, &bc.Token, filters)
		if err != nil {
			return nil, nil, err
		}
		var items []MediaItem
		for _, item := range radio.Items {
			items = append(items, item)
		}
		var nextCont *BuiltInContinuation
		if radio.Continuation != "" {
			nextCont = &BuiltInContinuation{
				Token:  radio.Continuation,
				Type:   ContinuationSong,
				ItemID: bc.ItemID,
			}
		}
		return items, nextCont, nil

	case ContinuationPlaylist:
		return client.GetPlaylistContinuation(ctx, false, bc.Token, 0)

	case ContinuationPlaylistInit:
		return client.GetPlaylistContinuation(ctx, true, bc.Token, bc.PlaylistSkipAmt)

	case ContinuationArtistShuffle:
		playlist, err := client.GetArtistShuffle(ctx, bc.ItemID, &bc.Token)
		if err != nil {
			return nil, nil, err
		}
		var items []MediaItem
		for _, item := range playlist.Items {
			items = append(items, item)
		}
		return items, playlist.Continuation, nil
	}

	return nil, nil, fmt.Errorf("unsupported continuation type: %s", bc.Type)
}
