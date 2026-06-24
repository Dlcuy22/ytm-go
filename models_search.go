// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define search outcome and category filtering options.
//
// Key Components:
//   - SearchResults: Scraped search outcomes
//   - SearchCategory: Result groups (Songs, Albums, Artists, etc.)
//   - SearchFilter: Parameter parameters used for pagination or category switching
//
// Dependencies:
//   - None
//
// Error Types:
//   - None
//
package ytm

// SearchType categorizes search filters.
type SearchType string

const (
	SearchSong     SearchType = "SONG"
	SearchVideo    SearchType = "VIDEO"
	SearchPlaylist SearchType = "PLAYLIST"
	SearchAlbum    SearchType = "ALBUM"
	SearchArtist   SearchType = "ARTIST"
)

// SearchFilter defines a filter target to search specific items.
type SearchFilter struct {
	Type   SearchType `json:"type"`
	Params string     `json:"params"`
}

// SearchCategory clusters search result layouts by category.
type SearchCategory struct {
	Layout       MediaItemLayout `json:"layout"`
	Filter       *SearchFilter   `json:"filter,omitempty"`
	Continuation string          `json:"continuation,omitempty"`
}

// Chip represents a search filter chip from the chip cloud.
type Chip struct {
	Name   string     `json:"name"`
	Params string     `json:"params,omitempty"`
	Type   SearchType `json:"type,omitempty"`
}

// SearchResults contains the parsed search response contents.
type SearchResults struct {
	Categories          []SearchCategory `json:"categories"`
	SuggestedCorrection string           `json:"suggested_correction,omitempty"`
	Chips               []Chip           `json:"chips,omitempty"`
}
