// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define visual feed layout and chip structures for home feed and browse pages.
//
// Key Components:
//   - FeedLoadResult: Scraped home feed outcome
//   - MediaItemLayout: Generic visual group of MediaItems (grid, row, list, etc.)
//   - FilterChip: InnerTube category pill
//
// Dependencies:
//   - None
//
// Error Types:
//   - None
//
package ytm

// LayoutType defines visual layouts for item lists.
type LayoutType string

const (
	LayoutTypeGrid         LayoutType = "GRID"
	LayoutTypeGridAlt      LayoutType = "GRID_ALT"
	LayoutTypeRow          LayoutType = "ROW"
	LayoutTypeList         LayoutType = "LIST"
	LayoutTypeNumberedList LayoutType = "NUMBERED_LIST"
	LayoutTypeCard         LayoutType = "CARD"
)

// MediaItemLayout displays a generic list of media items.
type MediaItemLayout struct {
	Items    []MediaItem `json:"items"`
	Title    string      `json:"title,omitempty"`
	Subtitle string      `json:"subtitle,omitempty"`
	ViewMore *PageRef    `json:"view_more,omitempty"`
	Type     LayoutType  `json:"type,omitempty"`
}

// FilterChip represents a tag/pill for filtering feeds.
type FilterChip struct {
	Text   string `json:"text"`
	Params string `json:"params"`
}

// FeedLoadResult contains the parsed feed rows and next pagination tokens.
type FeedLoadResult struct {
	Layouts      []MediaItemLayout `json:"layouts"`
	Continuation string            `json:"continuation,omitempty"`
	FilterChips  []FilterChip      `json:"filter_chips,omitempty"`
}
