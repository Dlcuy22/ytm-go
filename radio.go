// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement SongRadio endpoints and modifier generation.
//
// Key Components:
//   - GetSongRadio: fetches a song-radio stream and alternative modifiers
//   - RadioBuilderModifier: enum matching InnerTube radio filters (discover, upbeat, chill, etc.)
//   - videoIdToRadio / radioToFilters: encodes/decodes InnerTube RDAT radio keys
//
// Dependencies:
//   - context
//   - fmt
//   - strings
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"fmt"
	"strings"
)

type RadioBuilderModifier string

const (
	ModifierArtist      RadioBuilderModifier = "ARTIST"
	ModifierLowVariety  RadioBuilderModifier = "rX"
	ModifierHighVariety RadioBuilderModifier = "rZ"
	ModifierFamiliar    RadioBuilderModifier = "iY"
	ModifierDiscover    RadioBuilderModifier = "iX"
	ModifierPopular     RadioBuilderModifier = "pY"
	ModifierHidden      RadioBuilderModifier = "pX"
	ModifierNew         RadioBuilderModifier = "dX"
	ModifierPumpUp      RadioBuilderModifier = "mY"
	ModifierChill       RadioBuilderModifier = "mX"
	ModifierUpbeat      RadioBuilderModifier = "mb"
	ModifierDownbeat    RadioBuilderModifier = "mc"
	ModifierFocus       RadioBuilderModifier = "ma"
)

func ModifierFromString(s string) RadioBuilderModifier {
	switch s {
	case "iY":
		return ModifierFamiliar
	case "iX":
		return ModifierDiscover
	case "pY":
		return ModifierPopular
	case "pX":
		return ModifierHidden
	case "dX":
		return ModifierNew
	case "mY":
		return ModifierPumpUp
	case "mX":
		return ModifierChill
	case "mb":
		return ModifierUpbeat
	case "mc":
		return ModifierDownbeat
	case "ma":
		return ModifierFocus
	case "rX":
		return ModifierLowVariety
	case "rZ":
		return ModifierHighVariety
	}
	return ""
}

type RadioData struct {
	Items        []Song                   `json:"items"`
	Continuation string                   `json:"continuation,omitempty"`
	Filters      [][]RadioBuilderModifier `json:"filters,omitempty"`
}

/*
GetSongRadio retrieves tracks on the automix song radio feed.

    params:
          ctx: execution context
          songID: seed track ID
          continuation: optional continuation pagination token
          filters: list of active radio filters (upbeat, low variety, etc.)
    returns:
          *RadioData: matching radio tracks and filter alternatives
          error: network or parsing error
*/
func (c *Client) GetSongRadio(ctx context.Context, songID string, continuation *string, filters []RadioBuilderModifier) (*RadioData, error) {
	for _, f := range filters {
		if f == ModifierArtist {
			song, err := c.LoadSong(ctx, songID)
			if err != nil {
				return nil, err
			}
			if len(song.Artists) == 0 {
				return nil, fmt.Errorf("song has no artists")
			}
			artistRadio, err := c.GetArtistRadio(ctx, song.Artists[0].ID, nil)
			if err != nil {
				return nil, err
			}
			var contToken string
			if artistRadio.Continuation != nil {
				contToken = artistRadio.Continuation.Token
			}
			return &RadioData{
				Items:        artistRadio.Items,
				Continuation: contToken,
			}, nil
		}
	}

	bodyParams := map[string]any{
		"enablePersistentPlaylistPanel": true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
		"playlistId":                    videoIdToRadio(songID, filters),
		"isAudioOnly":                   true,
		"watchEndpointMusicSupportedConfigs": map[string]any{
			"watchEndpointMusicConfig": map[string]any{
				"hasPersistentPlaylistPanel": true,
				"musicVideoType":             "MUSIC_VIDEO_TYPE_ATV",
			},
		},
	}
	if continuation != nil {
		bodyParams["continuation"] = *continuation
	}

	var radioContents []ResponseRadioItem
	var nextContinuation string
	var outFilters [][]RadioBuilderModifier

	if continuation == nil {
		var resp YoutubeiNextResponse
		err := c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), bodyParams, false, &resp)
		if err != nil {
			return nil, err
		}

		defer func() { recover() }()
		renderer := resp.Contents.SingleColumnMusicWatchNextResultsRenderer.TabbedRenderer.WatchNextTabbedResultsRenderer.Tabs[0].TabRenderer.Content.MusicQueueRenderer
		if renderer.Content != nil {
			radioContents = renderer.Content.PlaylistPanelRenderer.Contents
			if len(renderer.Content.PlaylistPanelRenderer.Continuations) > 0 {
				nextContinuation = renderer.Content.PlaylistPanelRenderer.Continuations[0].GetToken()
			}
		}

		if renderer.SubHeaderChipCloud != nil {
			for _, chip := range renderer.SubHeaderChipCloud.ChipCloudRenderer.Chips {
				if playlistID := chip.getPlaylistID(); playlistID != "" {
					if list := radioToFilters(playlistID, songID); len(list) > 0 {
						outFilters = append(outFilters, list)
					}
				}
			}
		}
	} else {
		var resp YoutubeiNextContinuationResponse
		err := c.doInnerTube(ctx, "next", GetContextWebRemix(c.hl), bodyParams, false, &resp)
		if err != nil {
			return nil, err
		}
		radioContents = resp.ContinuationContents.PlaylistPanelContinuation.Contents
		if len(resp.ContinuationContents.PlaylistPanelContinuation.Continuations) > 0 {
			nextContinuation = resp.ContinuationContents.PlaylistPanelContinuation.Continuations[0].GetToken()
		}
	}

	var songs []Song
	for _, item := range radioContents {
		renderer := item.GetRenderer()
		if renderer == nil {
			continue
		}

		var duration int64
		if renderer.LengthText != nil {
			duration = parseDurationMs(renderer.LengthText.FirstText())
		}

		songs = append(songs, Song{
			ID:         CleanSongID(renderer.VideoID),
			Name:       renderer.Title.FirstText(),
			DurationMs: duration,
			Artists:    renderer.GetArtists(),
			Album:      renderer.GetAlbum(),
			Thumbnail:  NewThumbnailProvider(renderer.Thumbnail.Thumbnail.Thumbnails),
		})
	}

	return &RadioData{
		Items:        songs,
		Continuation: nextContinuation,
		Filters:      outFilters,
	}, nil
}

func (c RadioChip) getPlaylistID() string {
	return c.ChipCloudChipRenderer.NavigationEndpoint.QueueUpdateCommand.FetchContentsCommand.WatchEndpoint.PlaylistID
}

func videoIdToRadio(songID string, filters []RadioBuilderModifier) string {
	var nonInternalFilters []RadioBuilderModifier
	for _, f := range filters {
		if f != ModifierArtist {
			nonInternalFilters = append(nonInternalFilters, f)
		}
	}

	if len(nonInternalFilters) == 0 {
		return "RDAMVM" + songID
	}

	var ret strings.Builder
	ret.WriteString("RDAT")
	for _, f := range nonInternalFilters {
		ret.WriteString(string(f))
	}
	ret.WriteString("v")
	ret.WriteString(songID)
	return ret.String()
}

func radioToFilters(radio string, songID string) []RadioBuilderModifier {
	if !strings.HasPrefix(radio, "RDAT") {
		return nil
	}

	modifierString := radio[4 : len(radio)-len(songID)]
	var ret []RadioBuilderModifier

	i := 0
	for i+1 < len(modifierString) {
		mod := ModifierFromString(modifierString[i : i+2])
		if mod != "" {
			ret = append(ret, mod)
		}
		i += 2
	}
	return ret
}
