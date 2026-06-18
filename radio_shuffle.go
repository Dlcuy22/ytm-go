// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement ArtistShuffle details loading and parsing.
//
// Key Components:
//   - GetArtistShuffle: loads artist shuffle queue list prefixed with RAS
//   - GetArtistShuffleContinuation: pages the artist shuffle queue
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
GetArtistShuffle retrieves artist tracks in random order.

    params:
          ctx: execution context
          artistID: channel browse ID
          continuation: optional continuation token
    returns:
          *Playlist: artist shuffle playlist
          error: network or parsing error
*/
func (c *Client) GetArtistShuffle(ctx context.Context, artistID string, continuation *string) (*Playlist, error) {
	if continuation != nil {
		items, cont, err := c.GetArtistShuffleContinuation(ctx, artistID, *continuation)
		if err != nil {
			return nil, err
		}
		var songs []Song
		for _, it := range items {
			if s, ok := it.(*Song); ok {
				songs = append(songs, *s)
			}
		}
		return &Playlist{
			ID:           artistID,
			Items:        songs,
			Continuation: cont,
		}, nil
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": "RAS" + artistID,
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	defer func() { recover() }()
	tab := resp.Contents.SingleColumnBrowseResultsRenderer.Tabs[0]
	shelf := tab.TabRenderer.Content.SectionListRenderer.Contents[0].MusicPlaylistShelfRenderer

	var songs []Song
	for _, item := range shelf.Contents {
		if parsed, _ := item.ParseItem(c.hl); parsed != nil {
			if s, ok := parsed.(*Song); ok {
				songs = append(songs, *s)
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

	var cont *BuiltInContinuation
	if continuationToken != "" {
		cont = &BuiltInContinuation{
			Token:  continuationToken,
			Type:   ContinuationArtistShuffle,
			ItemID: artistID,
		}
	}

	return &Playlist{
		ID:           artistID,
		Items:        songs,
		Continuation: cont,
	}, nil
}

/*
GetArtistShuffleContinuation retrieves next page of artist shuffle queue.

    params:
          ctx: execution context
          artistID: channel browse ID
          token: continuation token
    returns:
          []MediaItem: list of continuation items
          *BuiltInContinuation: next page continuation token
          error: network or parsing error
*/
func (c *Client) GetArtistShuffleContinuation(ctx context.Context, artistID string, token string) ([]MediaItem, *BuiltInContinuation, error) {
	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"ctoken":       token,
		"continuation": token,
	}, false, &resp)
	if err != nil {
		return nil, nil, err
	}

	if resp.ContinuationContents != nil && resp.ContinuationContents.MusicPlaylistShelfContinuation != nil {
		shelf := resp.ContinuationContents.MusicPlaylistShelfContinuation
		var items []MediaItem
		for _, item := range shelf.Contents {
			if parsed, _ := item.ParseItem(c.hl); parsed != nil {
				items = append(items, parsed)
			}
		}

		var continuationToken string
		if len(shelf.Continuations) > 0 {
			continuationToken = shelf.Continuations[0].GetToken()
		}

		var cont *BuiltInContinuation
		if continuationToken != "" {
			cont = &BuiltInContinuation{
				Token:  continuationToken,
				Type:   ContinuationArtistShuffle,
				ItemID: artistID,
			}
		}
		return items, cont, nil
	}

	return nil, nil, nil
}
