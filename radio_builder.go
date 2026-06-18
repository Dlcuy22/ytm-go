// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement RadioBuilder endpoints and token generation.
//
// Key Components:
//   - RadioBuilderArtist: artist seed structure used to build a custom radio
//   - GetRadioBuilderArtists: retrieves seed list of artists
//   - BuildRadioToken: encodes builder artists and modifiers into custom VLRDAT radio token
//   - GetBuiltRadio: retrieves custom playlist from token
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

// RadioBuilderArtist represents an artist entity in the custom radio builder.
type RadioBuilderArtist struct {
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	Thumbnail Thumbnail `json:"thumbnail"`
}

/*
GetRadioBuilderArtists retrieves the list of seed artists configured for custom radio generation.

    params:
          ctx: execution context
    returns:
          []RadioBuilderArtist: list of seed options
          error: network or parsing error
*/
func (c *Client) GetRadioBuilderArtists(ctx context.Context) ([]RadioBuilderArtist, error) {
	var resp struct {
		Contents struct {
			SingleColumnBrowseResultsRenderer struct {
				Tabs []struct {
					TabRenderer struct {
						Content struct {
							SectionListRenderer struct {
								Contents []struct {
									ItemSectionRenderer struct {
										Contents []struct {
											ElementRenderer struct {
												NewElement struct {
													Type struct {
														ComponentType struct {
															Model struct {
																MusicRadioBuilderModel struct {
																	SeedItems []struct {
																		ItemEntityKey  string `json:"itemEntityKey"`
																		MusicThumbnail struct {
																			Image struct {
																				Sources []Thumbnail `json:"sources"`
																			} `json:"image"`
																		} `json:"musicThumbnail"`
																		Title string `json:"title"`
																	} `json:"seedItems"`
																} `json:"musicRadioBuilderModel"`
															} `json:"model"`
														} `json:"componentType"`
													} `json:"type"`
												} `json:"newElement"`
											} `json:"elementRenderer"`
										} `json:"contents"`
									} `json:"itemSectionRenderer"`
								} `json:"contents"`
							} `json:"sectionListRenderer"`
						} `json:"content"`
					} `json:"tabRenderer"`
				} `json:"tabs"`
			} `json:"singleColumnBrowseResultsRenderer"`
		} `json:"contents"`
		FrameworkUpdates struct {
			EntityBatchUpdate struct {
				Mutations []struct {
					EntityKey string `json:"entityKey"`
					Payload   struct {
						MusicFormBooleanChoice *struct {
							OpaqueToken string `json:"opaqueToken"`
						} `json:"musicFormBooleanChoice"`
					} `json:"payload"`
				} `json:"mutations"`
			} `json:"entityBatchUpdate"`
		} `json:"frameworkUpdates"`
	}

	err := c.doInnerTube(ctx, "browse", GetContextAndroid(c.hl), map[string]any{
		"browseId": "FEmusic_radio_builder",
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	defer func() { recover() }()
	seedItems := resp.Contents.SingleColumnBrowseResultsRenderer.Tabs[0].TabRenderer.Content.SectionListRenderer.Contents[0].ItemSectionRenderer.Contents[0].ElementRenderer.NewElement.Type.ComponentType.Model.MusicRadioBuilderModel.SeedItems
	mutations := resp.FrameworkUpdates.EntityBatchUpdate.Mutations

	var artists []RadioBuilderArtist
	limit := len(seedItems)
	if len(mutations) < limit {
		limit = len(mutations)
	}

	for i := 0; i < limit; i++ {
		item := seedItems[i]
		mutation := mutations[i]
		if mutation.Payload.MusicFormBooleanChoice != nil {
			var thumb Thumbnail
			if len(item.MusicThumbnail.Image.Sources) > 0 {
				thumb = item.MusicThumbnail.Image.Sources[0]
			}
			artists = append(artists, RadioBuilderArtist{
				Name:      item.Title,
				Token:     mutation.Payload.MusicFormBooleanChoice.OpaqueToken,
				Thumbnail: thumb,
			})
		}
	}

	return artists, nil
}

/*
BuildRadioToken encodes selected artists and modifiers into custom VLRDAT radio token.

    params:
          artists: seed choices
          modifiers: selected variety or style filters
    returns:
          string: constructed token
*/
func BuildRadioToken(artists []RadioBuilderArtist, modifiers []RadioBuilderModifier) string {
	if len(artists) == 0 {
		return ""
	}
	radioToken := "VLRDAT"

	var filterB, filterA, selType, variety string
	for _, m := range modifiers {
		switch m {
		case ModifierPumpUp, ModifierChill, ModifierUpbeat, ModifierDownbeat, ModifierFocus:
			filterB = string(m)
		case ModifierPopular, ModifierHidden, ModifierNew:
			filterA = string(m)
		case ModifierFamiliar, ModifierDiscover:
			selType = string(m)
		case ModifierLowVariety, ModifierHighVariety:
			variety = string(m)
		}
	}

	for _, val := range []string{filterB, filterA, selType, variety} {
		if val != "" {
			radioToken += val
		}
	}

	modifierAdded := filterB != "" || filterA != "" || selType != "" || variety != ""

	for i, artist := range artists {
		token := strings.TrimPrefix(artist.Token, "RDAT")
		if len(token) > 0 && token[0] == 'a' && i != 0 {
			token = "I" + token[1:]
		}

		if len(artists) == 1 && !modifierAdded {
			// keep full token
		} else if i+1 == len(artists) {
			idx := strings.LastIndex(token, "E")
			if idx != -1 {
				token = token[:idx+1]
			}
		} else {
			idx := strings.LastIndex(token, "E")
			if idx != -1 {
				token = token[:idx]
			}
		}

		radioToken += token
	}

	return radioToken
}

/*
GetBuiltRadio retrieves custom playlist contents from VLRDAT token.

    params:
          ctx: execution context
          radioToken: custom VLRDAT token
    returns:
          *Playlist: custom generated radio playlist
          error: network or parsing error
*/
func (c *Client) GetBuiltRadio(ctx context.Context, radioToken string) (*Playlist, error) {
	if !strings.HasPrefix(radioToken, "VLRDAT") || !strings.Contains(radioToken, "E") {
		return nil, fmt.Errorf("invalid radio token")
	}

	playlist, err := c.LoadPlaylist(ctx, radioToken, nil, nil, nil, false)
	if err != nil {
		return nil, err
	}

	playlist.Type = PlaylistTypeRadio
	return playlist, nil
}
