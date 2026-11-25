package ytm

import (
	"encoding/json"
	"testing"
)

func TestMusicCarouselShelfNumItemsPerColumnStringAndInt(t *testing.T) {
	var fromStr MusicCarouselShelfRenderer
	if err := json.Unmarshal([]byte(`{"numItemsPerColumn":"4","contents":[]}`), &fromStr); err != nil {
		t.Fatalf("unmarshal string numItemsPerColumn: %v", err)
	}
	if fromStr.NumItemsPerColumn == nil || *fromStr.NumItemsPerColumn != 4 {
		t.Fatalf("got %v, want 4", fromStr.NumItemsPerColumn)
	}

	var fromInt MusicCarouselShelfRenderer
	if err := json.Unmarshal([]byte(`{"numItemsPerColumn":3,"contents":[]}`), &fromInt); err != nil {
		t.Fatalf("unmarshal int numItemsPerColumn: %v", err)
	}
	if fromInt.NumItemsPerColumn == nil || *fromInt.NumItemsPerColumn != 3 {
		t.Fatalf("got %v, want 3", fromInt.NumItemsPerColumn)
	}
}

func TestYoutubeiShelfArtistSkipsNilNavigationEndpoint(t *testing.T) {
	shelf := YoutubeiShelf{
		MusicResponsiveHeaderRenderer: &MusicResponsiveHeaderRenderer{
			StraplineTextOne: &TextRuns{
				Runs: []TextRun{
					{Text: " • "},
					{
						Text: "Avenged Sevenfold",
						NavigationEndpoint: &NavigationEndpoint{
							BrowseEndpoint: &BrowseEndpoint{
								BrowseID: "UC1234",
								BrowseEndpointContextSupportedConfigs: &BrowseEndpointContextSupportedConfigs{
									BrowseEndpointContextMusicConfig: BrowseEndpointContextMusicConfig{
										PageType: "MUSIC_PAGE_TYPE_ARTIST",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	got := shelf.Artist()
	if got == nil {
		t.Fatal("expected artist, got nil")
	}
	if got.ID != "UC1234" || got.Name != "Avenged Sevenfold" {
		t.Fatalf("got %+v, want ID=UC1234 Name=Avenged Sevenfold", got)
	}
}

func TestMusicTwoRowItemRendererParsesAlbumYear(t *testing.T) {
	item := &MusicTwoRowItemRenderer{
		Title: TextRuns{Runs: []TextRun{{Text: "Under Mentality"}}},
		Subtitle: &TextRuns{
			Runs: []TextRun{
				{Text: "Album"},
				{Text: " • "},
				{Text: "2023"},
			},
		},
		NavigationEndpoint: NavigationEndpoint{
			BrowseEndpoint: &BrowseEndpoint{
				BrowseID: "MPREb_test",
				BrowseEndpointContextSupportedConfigs: &BrowseEndpointContextSupportedConfigs{
					BrowseEndpointContextMusicConfig: BrowseEndpointContextMusicConfig{
						PageType: "MUSIC_PAGE_TYPE_ALBUM",
					},
				},
			},
		},
	}

	got, ok := item.toMediaItem().(*Playlist)
	if !ok || got == nil {
		t.Fatalf("expected *Playlist, got %T", item.toMediaItem())
	}
	if got.Year != 2023 {
		t.Fatalf("Year = %d, want 2023", got.Year)
	}
}
