// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement artist details loading and parsing.
//
// Key Components:
//   - LoadArtist: loads detail metadata for an artist using /browse endpoint and MOBILE context
//   - parseArtistResponse: maps InnerTube browse structures into the Artist model
//   - parseSubscribers: parses subscriber string counts (like 10M or 250K) into integer metrics
//
// Dependencies:
//   - context
//   - strconv
//   - strings
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"strconv"
	"strings"
)

/*
LoadArtist retrieves detail layouts and channel metadata for the specified artist ID.

    params:
          ctx: execution context
          artistID: artist browse or channel ID
    returns:
          *Artist: populated artist details
          error: network or parsing error
*/
func (c *Client) LoadArtist(ctx context.Context, artistID string) (*Artist, error) {
	var resp YoutubeiBrowseResponse
	err := c.doInnerTube(ctx, "browse", GetContextMobile(c.hl), map[string]any{
		"browseId": artistID,
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	artist := parseArtistResponse(artistID, &resp, c.hl)
	if artist != nil {
		c.cache.StoreArtist(artist)
	}
	return artist, nil
}

func parseArtistResponse(artistID string, parsed *YoutubeiBrowseResponse, hl string) *Artist {
	artist := &Artist{
		ID: artistID,
	}

	headerRenderer := parsed.Header.GetRenderer()
	if headerRenderer != nil {
		if headerRenderer.Title != nil {
			artist.Name = headerRenderer.Title.FirstText()
		}
		if headerRenderer.Description != nil {
			artist.Description = headerRenderer.Description.FirstText()
		}
		artist.Thumbnail = NewThumbnailProvider(headerRenderer.GetThumbnails())

		if headerRenderer.SubscriptionButton != nil {
			sb := headerRenderer.SubscriptionButton.SubscribeButtonRenderer
			artist.SubscribeChannelID = sb.ChannelID
			artist.Subscribed = sb.Subscribed
			artist.SubscriberCount = parseSubscribers(sb.SubscriberCountText.FirstText())
		}

		if headerRenderer.PlayButton != nil && headerRenderer.PlayButton.ButtonRenderer.NavigationEndpoint.WatchEndpoint != nil {
			artist.ShufflePlaylistID = headerRenderer.PlayButton.ButtonRenderer.NavigationEndpoint.WatchEndpoint.PlaylistID
		}
	}

	var sectionListRenderer *SectionListRenderer
	if parsed.Contents != nil {
		if parsed.Contents.SingleColumnBrowseResultsRenderer != nil && len(parsed.Contents.SingleColumnBrowseResultsRenderer.Tabs) > 0 {
			content := parsed.Contents.SingleColumnBrowseResultsRenderer.Tabs[0].TabRenderer.Content
			if content != nil {
				sectionListRenderer = content.SectionListRenderer
			}
		} else if parsed.Contents.TwoColumnBrowseResultsRenderer != nil && parsed.Contents.TwoColumnBrowseResultsRenderer.SecondaryContents != nil {
			sectionListRenderer = &parsed.Contents.TwoColumnBrowseResultsRenderer.SecondaryContents.SectionListRenderer
		}
	}

	if sectionListRenderer != nil {
		var layouts []ArtistLayout
		for i, row := range sectionListRenderer.Contents {
			// Extract description
			if row.MusicDescriptionShelfRenderer != nil {
				artist.Description = row.MusicDescriptionShelfRenderer.Description.FirstText()
				continue
			}

			itemsAndIDs := row.GetMediaItemsAndSetIDs(hl)
			var items []MediaItem
			for _, entry := range itemsAndIDs {
				items = append(items, entry.Item)
			}

			var continuationToken string
			if row.MusicPlaylistShelfRenderer != nil && len(row.MusicPlaylistShelfRenderer.Continuations) > 0 {
				continuationToken = row.MusicPlaylistShelfRenderer.Continuations[0].GetToken()
			} else if row.MusicShelfRenderer != nil && len(row.MusicShelfRenderer.Continuations) > 0 {
				continuationToken = row.MusicShelfRenderer.Continuations[0].GetToken()
			}

			layoutTitle := row.Title()

			var viewMore *PageRef
			if endpoint := row.GetNavigationEndpoint(); endpoint != nil {
				if endpoint.BrowseEndpoint != nil {
					viewMore = &PageRef{
						BrowseID:     endpoint.BrowseEndpoint.BrowseID,
						BrowseParams: endpoint.BrowseEndpoint.Params,
					}
				}
			}

			layoutType := "GRID"
			if i == 0 {
				layoutType = "NUMBERED_LIST"
			}

			layouts = append(layouts, ArtistLayout{
				Items:      items,
				Title:      layoutTitle,
				Type:       layoutType,
				ViewMore:   viewMore,
				PlaylistID: continuationToken,
			})
		}
		artist.Layouts = layouts
	}

	return artist
}

func parseSubscribers(s string) int {
	s = strings.ToLower(s)
	var numStr string
	multiplier := 1
	for _, char := range s {
		if (char >= '0' && char <= '9') || char == '.' {
			numStr += string(char)
		} else if char == 'k' {
			multiplier = 1000
			break
		} else if char == 'm' {
			multiplier = 1000000
			break
		} else if char == 'b' {
			multiplier = 1000000000
			break
		}
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	return int(val * float64(multiplier))
}
