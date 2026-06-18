// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement search suggestions endpoint query.
//
// Key Components:
//   - GetSearchSuggestions: returns query autocomplete suggestions from InnerTube
//   - SearchSuggestion: suggestion metadata containing result query and history flag
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

// SearchSuggestion represents a single query autocomplete suggestion.
type SearchSuggestion struct {
	Query     string `json:"query"`
	IsHistory bool   `json:"is_history"`
}

/*
GetSearchSuggestions retrieves query autocomplete strings from YouTube Music.

    params:
          ctx: execution context
          query: prefix query keywords
    returns:
          []SearchSuggestion: list of suggestions
          error: network or parsing error
*/
func (c *Client) GetSearchSuggestions(ctx context.Context, query string) ([]SearchSuggestion, error) {
	var resp struct {
		Contents []struct {
			SearchSuggestionsSectionRenderer *struct {
				Contents []struct {
					SearchSuggestionRenderer *struct {
						NavigationEndpoint struct {
							SearchEndpoint struct {
								Query string `json:"query"`
							} `json:"searchEndpoint"`
						} `json:"navigationEndpoint"`
					} `json:"searchSuggestionRenderer"`
					HistorySuggestionRenderer *struct {
						NavigationEndpoint struct {
							SearchEndpoint struct {
								Query string `json:"query"`
							} `json:"searchEndpoint"`
						} `json:"navigationEndpoint"`
					} `json:"historySuggestionRenderer"`
				} `json:"contents"`
			} `json:"searchSuggestionsSectionRenderer"`
		} `json:"contents"`
	}

	err := c.doInnerTube(ctx, "music/get_search_suggestions", GetContextWebRemix(c.hl), map[string]any{
		"input": query,
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	var suggestions []SearchSuggestion
	if len(resp.Contents) > 0 && resp.Contents[0].SearchSuggestionsSectionRenderer != nil {
		for _, suggestion := range resp.Contents[0].SearchSuggestionsSectionRenderer.Contents {
			if suggestion.SearchSuggestionRenderer != nil {
				suggestions = append(suggestions, SearchSuggestion{
					Query:     suggestion.SearchSuggestionRenderer.NavigationEndpoint.SearchEndpoint.Query,
					IsHistory: false,
				})
			} else if suggestion.HistorySuggestionRenderer != nil {
				suggestions = append(suggestions, SearchSuggestion{
					Query:     suggestion.HistorySuggestionRenderer.NavigationEndpoint.SearchEndpoint.Query,
					IsHistory: true,
				})
			}
		}
	}

	return suggestions, nil
}
