// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement playlist continuation paging.
//
// Key Components:
//   - GetPlaylistContinuation: loads the next page of playlist items
//   - toContinuationToken: helper to extract pagination tokens
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
GetPlaylistContinuation fetches paginated elements from the playlist.

    params:
          ctx: execution context
          initial: is this the first continuation page
          token: continuation token
          skipInitial: number of items to skip
    returns:
          []MediaItem: list of continuation items
          *BuiltInContinuation: next page continuation token
          error: network or parsing error
*/
func (c *Client) GetPlaylistContinuation(ctx context.Context, initial bool, token string, skipInitial int) ([]MediaItem, *BuiltInContinuation, error) {
	if initial {
		playlist, err := c.LoadPlaylist(ctx, token, nil, nil, nil, false)
		if err != nil {
			return nil, nil, err
		}
		items := playlist.Items
		if skipInitial < len(items) {
			items = items[skipInitial:]
		} else {
			items = nil
		}
		var genericItems []MediaItem
		for _, it := range items {
			genericItems = append(genericItems, it)
		}
		return genericItems, playlist.Continuation, nil
	}

	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"ctoken":       token,
		"continuation": token,
	}, false, &resp)
	if err != nil {
		return nil, nil, err
	}

	if len(resp.OnResponseReceivedActions) > 0 {
		var items []MediaItem
		var nextToken string
		for _, action := range resp.OnResponseReceivedActions {
			if action.AppendContinuationItemsAction != nil {
				for _, shelfItem := range action.AppendContinuationItemsAction.ContinuationItems {
					if parsed, _ := shelfItem.ParseItem(c.hl); parsed != nil {
						items = append(items, parsed)
					}
					if tok := shelfItem.toContinuationToken(); tok != "" {
						nextToken = tok
					}
				}
			}
		}

		if skipInitial < len(items) {
			items = items[skipInitial:]
		} else {
			items = nil
		}

		var cont *BuiltInContinuation
		if nextToken != "" {
			cont = &BuiltInContinuation{
				Token: nextToken,
				Type:  ContinuationPlaylist,
			}
		}
		return items, cont, nil
	}

	if resp.ContinuationContents != nil && resp.ContinuationContents.MusicPlaylistShelfContinuation != nil {
		shelf := resp.ContinuationContents.MusicPlaylistShelfContinuation
		var items []MediaItem
		for _, item := range shelf.Contents {
			if parsed, _ := item.ParseItem(c.hl); parsed != nil {
				items = append(items, parsed)
			}
		}

		if skipInitial < len(items) {
			items = items[skipInitial:]
		} else {
			items = nil
		}

		var cont *BuiltInContinuation
		if len(shelf.Continuations) > 0 {
			if nextToken := shelf.Continuations[0].GetToken(); nextToken != "" {
				cont = &BuiltInContinuation{
					Token: nextToken,
					Type:  ContinuationPlaylist,
				}
			}
		}
		return items, cont, nil
	}

	return nil, nil, nil
}

func (i YoutubeiShelfContentsItem) toContinuationToken() string {
	if i.ContinuationItemRenderer != nil {
		return i.ContinuationItemRenderer.ContinuationEndpoint.ContinuationCommand.Token
	}
	return ""
}
