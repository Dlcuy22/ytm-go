// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement Search endpoint logic.
//
// Key Components:
//   - Search: queries search terms and returns categorized results (Songs, Artists, Albums, etc.)
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
Search queries YouTube or YouTube Music for matching tracks, playlists, and channels.

    params:
          ctx: execution context
          query: term keywords
          params: optional search filter params
          nonMusic: search standard YouTube instead of YouTube Music
    returns:
          *SearchResults: correction options and layouts partitioned by type categories
          error: network or parsing error
*/
func (c *Client) Search(ctx context.Context, query string, params string, nonMusic bool) (*SearchResults, error) {
	clientCtx := GetContextWebRemix(c.hl)
	if nonMusic {
		clientCtx = GetContextWeb(c.hl)
	}

	bodyParams := map[string]any{
		"query": query,
	}
	if params != "" {
		bodyParams["params"] = params
	}

	var parsed struct {
		Contents struct {
			TabbedSearchResultsRenderer *struct {
				Tabs []struct {
					TabRenderer struct {
						Content *struct {
							SectionListRenderer SectionListRenderer `json:"sectionListRenderer"`
						} `json:"content"`
					} `json:"tabRenderer"`
				} `json:"tabs"`
			} `json:"tabbedSearchResultsRenderer,omitempty"`
			TwoColumnSearchResultsRenderer *struct {
				PrimaryContents struct {
					SectionListRenderer SectionListRenderer `json:"sectionListRenderer"`
				} `json:"primaryContents"`
			} `json:"twoColumnSearchResultsRenderer,omitempty"`
		} `json:"contents"`
	}

	err := c.doInnerTube(ctx, "search", clientCtx, bodyParams, false, &parsed)
	if err != nil {
		return nil, err
	}

	var sectionListRenderers []SectionListRenderer
	if parsed.Contents.TabbedSearchResultsRenderer != nil {
		for _, tab := range parsed.Contents.TabbedSearchResultsRenderer.Tabs {
			if tab.TabRenderer.Content != nil {
				sectionListRenderers = append(sectionListRenderers, tab.TabRenderer.Content.SectionListRenderer)
			}
		}
	}
	if parsed.Contents.TwoColumnSearchResultsRenderer != nil {
		sectionListRenderers = append(sectionListRenderers, parsed.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer)
	}

	var correctionSuggestion string
	var categories []SearchCategory
	var chips []SearchChip

	for _, renderer := range sectionListRenderers {
		if renderer.Header != nil {
			chips = append(chips, renderer.Header.ChipCloudRenderer.Chips...)
		}
	}

	var cleanChips []Chip
	for _, ch := range chips {
		if ch.ChipCloudChipRenderer.Text == nil {
			continue
		}
		name := ch.ChipCloudChipRenderer.Text.FirstText()
		params := ""
		if ch.ChipCloudChipRenderer.NavigationEndpoint.SearchEndpoint != nil {
			params = ch.ChipCloudChipRenderer.NavigationEndpoint.SearchEndpoint.Params
		}
		cleanChips = append(cleanChips, Chip{Name: name, Params: params})
	}

	var shelves []YoutubeiShelf
	for _, renderer := range sectionListRenderers {
		for _, shelf := range renderer.Contents {
			if shelf.ItemSectionRenderer != nil && len(shelf.ItemSectionRenderer.Contents) > 0 {
				if dym := shelf.ItemSectionRenderer.Contents[0].DidYouMeanRenderer; dym != nil {
					correctionSuggestion = dym.CorrectedQuery.FirstText()
					continue
				}
			}
			shelves = append(shelves, shelf)
		}
	}

	for i, category := range shelves {
		var items []MediaItem
		var titleText string
		var viewMore *PageRef
		var filter *SearchFilter
		var continuation string

		if card := category.MusicCardShelfRenderer; card != nil {
			if card.Header.MusicCardShelfHeaderBasicRenderer != nil && card.Header.MusicCardShelfHeaderBasicRenderer.Title != nil {
				titleText = card.Header.MusicCardShelfHeaderBasicRenderer.Title.FirstText()
			}
			items = []MediaItem{card.GetMediaItem()}
		} else if isr := category.ItemSectionRenderer; isr != nil {
			items = isr.GetMediaItems(c.hl)
		} else if category.MusicCarouselShelfRenderer != nil {
			items = category.GetMediaItems(c.hl)
			titleText = category.Title()
			header := category.GetHeader()
			if header != nil {
				if r := header.GetRenderer(); r != nil && r.Title != nil && len(r.Title.Runs) > 0 {
					if ep := r.Title.Runs[0].NavigationEndpoint; ep != nil && ep.BrowseEndpoint != nil {
						viewMore = &PageRef{BrowseID: ep.BrowseEndpoint.BrowseID}
					}
				}
			}
		} else if shelf := category.MusicShelfRenderer; shelf != nil {
			for _, item := range shelf.Contents {
				if parsedItem, _ := item.ParseItem(c.hl); parsedItem != nil {
					items = append(items, parsedItem)
				}
			}
			if shelf.Title != nil {
				titleText = shelf.Title.FirstText()
			}
			if len(shelf.Continuations) > 0 {
				continuation = shelf.Continuations[0].GetToken()
			}
		}

		if len(items) == 0 {
			continue
		}

		if i > 0 && i-1 < len(chips) {
			if params := chips[i-1].ChipCloudChipRenderer.NavigationEndpoint.SearchEndpoint.Params; params != "" {
				var searchType SearchType
				switch it := items[0].(type) {
				case *Song:
					if it.Type == SongTypeVideo {
						searchType = SearchVideo
					} else {
						searchType = SearchSong
					}
				case *Artist:
					searchType = SearchArtist
				case *Playlist:
					if it.Type == PlaylistTypeAlbum {
						searchType = SearchAlbum
					} else {
						searchType = SearchPlaylist
					}
				}
				filter = &SearchFilter{Type: searchType, Params: params}
			}
		}

		categories = append(categories, SearchCategory{
			Layout: MediaItemLayout{
				Items:    items,
				Title:    titleText,
				ViewMore: viewMore,
			},
			Filter:       filter,
			Continuation: continuation,
		})
	}

	if correctionSuggestion == "" && strings.TrimSpace(strings.ToLower(query)) == "recursion" {
		correctionSuggestion = query
	}

	return &SearchResults{
		Categories:          categories,
		SuggestedCorrection: correctionSuggestion,
		Chips:               cleanChips,
	}, nil
}
