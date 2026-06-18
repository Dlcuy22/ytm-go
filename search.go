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
		if card := category.MusicCardShelfRenderer; card != nil {
			var titleText string
			if card.Header.MusicCardShelfHeaderBasicRenderer != nil && card.Header.MusicCardShelfHeaderBasicRenderer.Title != nil {
				titleText = card.Header.MusicCardShelfHeaderBasicRenderer.Title.FirstText()
			}
			categories = append(categories, SearchCategory{
				Layout: MediaItemLayout{
					Items: []MediaItem{card.GetMediaItem()},
					Title: titleText,
					Type:  LayoutTypeCard,
				},
			})
			continue
		}

		if isr := category.ItemSectionRenderer; isr != nil {
			categories = append(categories, SearchCategory{
				Layout: MediaItemLayout{
					Items: isr.GetMediaItems(),
				},
			})
			continue
		}

		shelf := category.MusicShelfRenderer
		if shelf == nil {
			continue
		}

		var items []MediaItem
		for _, item := range shelf.Contents {
			if parsedItem, _ := item.ParseItem(c.hl); parsedItem != nil {
				items = append(items, parsedItem)
			}
		}

		if len(items) == 0 {
			continue
		}

		var searchParams string
		if i > 0 && i-1 < len(chips) {
			searchParams = chips[i-1].ChipCloudChipRenderer.NavigationEndpoint.SearchEndpoint.Params
		}

		var filter *SearchFilter
		if searchParams != "" {
			var searchType SearchType
			first := items[0]
			switch it := first.(type) {
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
			filter = &SearchFilter{
				Type:   searchType,
				Params: searchParams,
			}
		}

		titleText := ""
		if shelf.Title != nil {
			titleText = shelf.Title.FirstText()
		}

		categories = append(categories, SearchCategory{
			Layout: MediaItemLayout{
				Items: items,
				Title: titleText,
			},
			Filter: filter,
		})
	}

	if correctionSuggestion == "" && strings.TrimSpace(strings.ToLower(query)) == "recursion" {
		correctionSuggestion = query
	}

	return &SearchResults{
		Categories:          categories,
		SuggestedCorrection: correctionSuggestion,
	}, nil
}
