// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement song feed browse views and view more pagination.
//
// Key Components:
//   - GetSongFeed: requests and pages the home music feed using browse endpoint
//   - GetGenericFeedViewMore: loads a specific view-more section (like listen again)
//   - processRows: maps unmarshaled browse shelf rows into display layouts
//
// Dependencies:
//   - context
//   - strings
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"strings"
)

/*
GetSongFeed retrieves the structured feed layouts for home browse view.

    params:
          ctx: execution context
          minRows: minimum rows to load before stopping pagination
          params: optional InnerTube chip search params
          continuation: optional pagination ctoken
    returns:
          *FeedLoadResult: list of loaded layouts and continuation metadata
          error: network or parsing error
*/
func (c *Client) GetSongFeed(ctx context.Context, minRows int, params *string, continuation *string) (*FeedLoadResult, error) {
	hl := c.hl

	performRequest := func(ctoken string) (*YoutubeiBrowseResponse, error) {
		bodyParams := map[string]any{}
		if params != nil {
			bodyParams["params"] = *params
		}

		path := "browse"
		if ctoken != "" {
			path = "browse?ctoken=" + ctoken + "&continuation=" + ctoken + "&type=next"
		}

		var resp YoutubeiBrowseResponse
		err := c.doInnerTube(ctx, path, GetContextWebRemix(c.hl), bodyParams, false, &resp)
		if err != nil {
			return nil, err
		}
		return &resp, nil
	}

	var ctoken string
	if continuation != nil {
		ctoken = *continuation
	}

	data, err := performRequest(ctoken)
	if err != nil {
		return nil, err
	}

	filterChips := data.GetHeaderChips()

	var layouts []MediaItemLayout
	shelves := data.GetShelves(ctoken != "")
	layouts = append(layouts, c.processRows(shelves, hl)...)

	nextCtoken := data.CToken()
	for nextCtoken != "" && minRows >= 1 && len(layouts) < minRows {
		data, err = performRequest(nextCtoken)
		if err != nil {
			break
		}
		nextCtoken = data.CToken()
		shelves = data.GetShelves(true)
		if len(shelves) == 0 {
			break
		}
		layouts = append(layouts, c.processRows(shelves, hl)...)
	}

	return &FeedLoadResult{
		Layouts:      layouts,
		Continuation: nextCtoken,
		FilterChips:  filterChips,
	}, nil
}

/*
GetGenericFeedViewMore loads items for a dedicated section browse ID.

    params:
          ctx: execution context
          browseID: browse target ID
    returns:
          []MediaItem: list of loaded media items
          error: network or parsing error
*/
func (c *Client) GetGenericFeedViewMore(ctx context.Context, browseID string) ([]MediaItem, error) {
	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextWebRemix(c.hl), map[string]any{
		"browseId": browseID,
	}, true, &resp)
	if err != nil {
		return nil, err
	}

	defer func() { recover() }()
	shelves := resp.GetShelves(false)
	if len(shelves) > 0 {
		return shelves[0].GetMediaItems(c.hl), nil
	}
	return nil, nil
}

func (c *Client) processRows(shelves []YoutubeiShelf, hl string) []MediaItemLayout {
	var layouts []MediaItemLayout
	for _, shelf := range shelves {
		renderer := shelf.GetRenderer()
		if renderer == nil {
			continue
		}

		var header *HeaderRenderer
		if shelf.MusicCarouselShelfRenderer != nil && shelf.MusicCarouselShelfRenderer.Header != nil {
			header = shelf.MusicCarouselShelfRenderer.Header.GetRenderer()
		} else if shelf.MusicShelfRenderer != nil {
			header = &HeaderRenderer{Title: shelf.MusicShelfRenderer.Title}
		} else if shelf.MusicPlaylistShelfRenderer != nil {
			header = &HeaderRenderer{Title: shelf.MusicPlaylistShelfRenderer.Title}
		} else if shelf.MusicCardShelfRenderer != nil {
			if shelf.MusicCardShelfRenderer.Header.MusicCardShelfHeaderBasicRenderer != nil {
				header = &HeaderRenderer{Title: shelf.MusicCardShelfRenderer.Header.MusicCardShelfHeaderBasicRenderer.Title}
			}
		}

		if header == nil || header.Title == nil {
			continue
		}

		titleText := header.Title.FirstText()
		subtitleText := ""
		if header.Subtitle != nil {
			subtitleText = header.Subtitle.FirstText()
		} else if header.Strapline != nil {
			subtitleText = header.Strapline.FirstText()
		}

		items := shelf.GetMediaItems(hl)

		var viewMore *PageRef
		if shelf.GetNavigationEndpoint() != nil && shelf.GetNavigationEndpoint().BrowseEndpoint != nil {
			be := shelf.GetNavigationEndpoint().BrowseEndpoint
			viewMore = &PageRef{
				BrowseID:     be.BrowseID,
				BrowseParams: be.Params,
			}
		}

		if len(header.Title.Runs) > 0 && header.Title.Runs[0].NavigationEndpoint != nil && header.Title.Runs[0].NavigationEndpoint.BrowseEndpoint != nil {
			be := header.Title.Runs[0].NavigationEndpoint.BrowseEndpoint
			if strings.HasPrefix(be.BrowseID, "FEmusic_") {
				viewMore = &PageRef{
					BrowseID: be.BrowseID,
				}
			}
		}

		layoutType := LayoutTypeRow
		if shelf.MusicCarouselShelfRenderer != nil && shelf.MusicCarouselShelfRenderer.NumItemsPerColumn != nil && *shelf.MusicCarouselShelfRenderer.NumItemsPerColumn > 1 {
			layoutType = LayoutTypeGridAlt
		}

		layouts = append(layouts, MediaItemLayout{
			Items:    items,
			Title:    titleText,
			Subtitle: subtitleText,
			ViewMore: viewMore,
			Type:     layoutType,
		})
	}
	return layouts
}
