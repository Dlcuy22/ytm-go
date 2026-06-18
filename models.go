// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define core shared types, interfaces, and utilities including media items,
//   thumbnails, and authentication models.
//
// Key Components:
//   - MediaItem: Interface for all YouTube Music items (songs, artists, playlists)
//   - Thumbnail: Representation of an image asset
//   - ThumbnailProvider: Structured selector for various resolution thumbnails
//
// Dependencies:
//   - None
//
// Error Types:
//   - None
//
package ytm

import (
	"fmt"
	"strconv"
	"strings"
)

// MediaItem represents a generic media item scraped from YouTube Music.
type MediaItem interface {
	GetID() string
	GetName() string
	GetThumbnailProvider() *ThumbnailProvider
}

// Thumbnail represents a single thumbnail image.
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ThumbnailQuality specifies the quality constraint for the thumbnail URL.
type ThumbnailQuality string

const (
	ThumbnailQualityLow  ThumbnailQuality = "LOW"
	ThumbnailQualityHigh ThumbnailQuality = "HIGH"
)

// ThumbnailProvider resolves thumbnail URLs dynamically based on target size.
type ThumbnailProvider struct {
	UrlA string  `json:"url_a"`
	UrlB *string `json:"url_b,omitempty"`
}

/*
NewThumbnailProvider creates a ThumbnailProvider from a slice of Thumbnails.

    params:
          thumbnails: a list of thumbnails with different sizes
    returns:
          *ThumbnailProvider: resolved thumbnail provider, or nil if slice is empty
*/
func NewThumbnailProvider(thumbnails []Thumbnail) *ThumbnailProvider {
	if len(thumbnails) == 0 {
		return nil
	}

	for _, t := range thumbnails {
		wIndex := strings.LastIndex(t.URL, fmt.Sprintf("w%d", t.Width))
		if wIndex == -1 {
			continue
		}

		hIndex := strings.LastIndex(t.URL, fmt.Sprintf("-h%d", t.Height))
		if hIndex == -1 {
			continue
		}

		heightStr := strconv.Itoa(t.Height)

		return &ThumbnailProvider{
			UrlA: t.URL[:wIndex+1],
			UrlB: stringPtr(t.URL[hIndex+2+len(heightStr):]),
		}
	}

	highURL := ""
	highSize := -1
	lowURL := ""
	lowSize := -1

	for _, t := range thumbnails {
		size := t.Width * t.Height
		if highSize == -1 || size > highSize {
			highSize = size
			highURL = t.URL
		}
		if lowSize == -1 || size < lowSize {
			lowSize = size
			lowURL = t.URL
		}
	}

	var urlB *string
	if highURL != lowURL {
		urlB = &lowURL
	}

	return &ThumbnailProvider{
		UrlA: highURL,
		UrlB: urlB,
	}
}

/*
NewThumbnailProviderFromURL creates a simple static ThumbnailProvider from a URL.

    params:
          url: image source URL
    returns:
          *ThumbnailProvider: static thumbnail provider
*/
func NewThumbnailProviderFromURL(url string) *ThumbnailProvider {
	return &ThumbnailProvider{
		UrlA: url,
	}
}

/*
GetThumbnailURL resolves the URL for the requested quality.

    params:
          quality: desired quality constraint (LOW, HIGH)
    returns:
          string: resolved image URL
*/
func (p *ThumbnailProvider) GetThumbnailURL(quality ThumbnailQuality) string {
	if p.UrlB == nil || strings.HasPrefix(*p.UrlB, "https://") {
		if p.UrlB == nil {
			return p.UrlA
		}
		if quality == ThumbnailQualityHigh {
			return p.UrlA
		}
		return *p.UrlB
	}

	var width, height int
	if quality == ThumbnailQualityLow {
		width, height = 180, 180
	} else {
		width, height = 720, 720
	}

	return fmt.Sprintf("%s%d-h%d%s", p.UrlA, width, height, *p.UrlB)
}

func stringPtr(s string) *string {
	return &s
}
