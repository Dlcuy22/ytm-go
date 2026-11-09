// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Verify lyric sheet decoding: the timed mobile shape, the untimed mobile
//   shape, the plaintext web shelf, the unavailable message, and the tab
//   lookup the lyrics browse ID comes from.
//
// Key Components:
//   - TestLyricSheet*: decoding each response shape
//   - TestTabBrowseID*: page-type tab selection
//   - TestGetContextIOSMusic: the preset that returns timed lyrics
//   - TestGetSongLyricsLive: opt-in network check, skipped by default
//
// Dependencies:
//   - context
//   - encoding/json
//   - errors
//   - os
//   - testing
//
// Error Types:
//   - None
//
package ytm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// timedLyricsBody is a trimmed real response from the iOS Music profile: the
// element renderer wrapper with a timed model inside it.
const timedLyricsBody = `{
  "contents": {
    "elementRenderer": {
      "newElement": {
        "type": {
          "componentType": {
            "model": {
              "timedLyricsModel": {
                "lyricsData": {
                  "timedLyricsData": [
                    {"lyricLine": "♪", "cueRange": {"startTimeMilliseconds": "0", "endTimeMilliseconds": "29210", "metadata": {"id": "0"}}},
                    {"lyricLine": "first line", "cueRange": {"startTimeMilliseconds": "29210", "endTimeMilliseconds": "33290", "metadata": {"id": "1"}}},
                    {"lyricLine": "second line", "cueRange": {"startTimeMilliseconds": "33290", "endTimeMilliseconds": "38530", "metadata": {"id": "2"}}}
                  ]
                }
              }
            }
          }
        }
      }
    }
  }
}`

// untimedLyricsBody is the same wrapper with the cue ranges removed. The
// endpoint does this for tracks whose lyrics it has not timed.
const untimedLyricsBody = `{
  "contents": {
    "elementRenderer": {
      "newElement": {
        "type": {
          "componentType": {
            "model": {
              "timedLyricsModel": {
                "lyricsData": {
                  "timedLyricsData": [
                    {"lyricLine": "first line"},
                    {"lyricLine": "second line"}
                  ]
                }
              }
            }
          }
        }
      }
    }
  }
}`

// plainLyricsBody is the desktop web shape: a description shelf.
const plainLyricsBody = `{
  "contents": {
    "sectionListRenderer": {
      "contents": [
        {
          "musicDescriptionShelfRenderer": {
            "description": {"runs": [{"text": "first line\nsecond line"}]}
          }
        }
      ]
    }
  }
}`

// unavailableLyricsBody is what a track without lyrics is answered with.
const unavailableLyricsBody = `{
  "contents": {
    "messageRenderer": {
      "text": {"runs": [{"text": "Lyrics not available"}]}
    }
  }
}`

func decodeLyricSheet(t *testing.T, body string) (*Lyrics, error) {
	t.Helper()
	var resp lyricBrowseResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	return resp.sheet()
}

// TestLyricSheetTimed pins the mobile shape: cue ranges are decoded from
// strings and the sheet reports itself synced.
func TestLyricSheetTimed(t *testing.T) {
	sheet, err := decodeLyricSheet(t, timedLyricsBody)
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	if !sheet.Synced {
		t.Error("a sheet with every cue range must be synced")
	}
	if len(sheet.Lines) != 3 {
		t.Fatalf("lines: got %d, want 3", len(sheet.Lines))
	}
	if sheet.Lines[1].Text != "first line" {
		t.Errorf("line text: got %q", sheet.Lines[1].Text)
	}
	if sheet.Lines[1].StartMs != 29210 || sheet.Lines[1].EndMs != 33290 {
		t.Errorf("cue: got %d-%d", sheet.Lines[1].StartMs, sheet.Lines[1].EndMs)
	}
	if got := sheet.PlainText(); got != "first line\nsecond line" {
		t.Errorf("plain text: got %q", got)
	}
}

// TestLyricSheetUntimed pins the mixed shape: lines without cue ranges are
// still readable, but the sheet must not claim to be synced.
func TestLyricSheetUntimed(t *testing.T) {
	sheet, err := decodeLyricSheet(t, untimedLyricsBody)
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	if sheet.Synced {
		t.Error("a sheet without cue ranges must not be synced")
	}
	if len(sheet.Lines) != 2 {
		t.Fatalf("lines: got %d, want 2", len(sheet.Lines))
	}
	if sheet.Lines[0].StartMs != 0 {
		t.Errorf("untimed line should stay at zero, got %d", sheet.Lines[0].StartMs)
	}
}

// TestLyricSheetPlainShelf pins the desktop web fallback.
func TestLyricSheetPlainShelf(t *testing.T) {
	sheet, err := decodeLyricSheet(t, plainLyricsBody)
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	if sheet.Synced {
		t.Error("a description shelf has no timing")
	}
	if len(sheet.Lines) != 2 || sheet.Lines[1].Text != "second line" {
		t.Errorf("shelf lines: got %+v", sheet.Lines)
	}
}

// TestLyricSheetPlainShelfMultipleRuns checks that a shelf whose text arrived
// in several runs keeps all of them instead of stopping at the first.
func TestLyricSheetPlainShelfMultipleRuns(t *testing.T) {
	body := `{"contents":{"sectionListRenderer":{"contents":[{"musicDescriptionShelfRenderer":{"description":{"runs":[{"text":"first line\n"},{"text":"second line"}]}}}]}}}`
	sheet, err := decodeLyricSheet(t, body)
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	if got := sheet.PlainText(); got != "first line\nsecond line" {
		t.Errorf("plain text: got %q", got)
	}
}

// TestLyricSheetUnavailable checks that an explicit unavailability is an
// error a caller can test for rather than an empty sheet.
func TestLyricSheetUnavailable(t *testing.T) {
	_, err := decodeLyricSheet(t, unavailableLyricsBody)
	if !errors.Is(err, ErrLyricsNotAvailable) {
		t.Errorf("got %v, want ErrLyricsNotAvailable", err)
	}
}

// TestLyricSheetUnknownShape checks that a body with no sheet at all is
// reported as missing rather than silently empty.
func TestLyricSheetUnknownShape(t *testing.T) {
	_, err := decodeLyricSheet(t, `{"contents":{}}`)
	if !errors.Is(err, ErrLyricsNotFound) {
		t.Errorf("got %v, want ErrLyricsNotFound", err)
	}
}

// TestGetSongLyricsTimedRejectsEmptyID checks the guard, which also keeps a
// song with no lyrics tab from costing a request.
func TestGetSongLyricsTimedRejectsEmptyID(t *testing.T) {
	_, err := NewClient().GetSongLyricsTimed(context.Background(), "")
	if !errors.Is(err, ErrLyricsNotFound) {
		t.Errorf("got %v, want ErrLyricsNotFound", err)
	}
}

// TestTabBrowseIDSelectsByPageType pins the lookup that replaced positional
// tab indexing: a podcast has no lyrics tab, and its third tab is not Related.
func TestTabBrowseIDSelectsByPageType(t *testing.T) {
	musicTrack := []Tab{
		musicTab("", ""),
		musicTab("MPLYt_abc", "MUSIC_PAGE_TYPE_TRACK_LYRICS"),
		musicTab("", ""),
		musicTab("MPTRt_abc", "MUSIC_PAGE_TYPE_TRACK_RELATED"),
	}
	if got := tabBrowseID(musicTrack, "MUSIC_PAGE_TYPE_TRACK_LYRICS"); got != "MPLYt_abc" {
		t.Errorf("lyrics tab: got %q", got)
	}
	if got := tabBrowseID(musicTrack, "MUSIC_PAGE_TYPE_TRACK_RELATED"); got != "MPTRt_abc" {
		t.Errorf("related tab: got %q", got)
	}

	podcast := []Tab{
		musicTab("", ""),
		musicTab("MPPEabc", "MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_DETAILS"),
		musicTab("COMMENTSabc", ""),
		musicTab("MPPRabc", "MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_RELATED"),
	}
	if got := tabBrowseID(podcast, "MUSIC_PAGE_TYPE_TRACK_LYRICS"); got != "" {
		t.Errorf("a podcast has no lyrics tab, got %q", got)
	}
}

// musicTab builds one watch-next tab with the page type the lookup reads.
func musicTab(browseID, pageType string) Tab {
	var tab Tab
	tab.TabRenderer.Endpoint = &TabRendererEndpoint{}
	tab.TabRenderer.Endpoint.BrowseEndpoint.BrowseID = browseID
	if pageType != "" {
		tab.TabRenderer.Endpoint.BrowseEndpoint.BrowseEndpointContextSupportedConfigs =
			&BrowseEndpointContextSupportedConfigs{
				BrowseEndpointContextMusicConfig: BrowseEndpointContextMusicConfig{PageType: pageType},
			}
	}
	return tab
}

// TestGetContextIOSMusic pins the profile: the client name and numeric id are
// what select the mobile lyric response, and the version is what the endpoint
// checks before answering at all.
func TestGetContextIOSMusic(t *testing.T) {
	ctx := GetContextIOSMusic("ja")
	if ctx.ClientName != "IOS_MUSIC" {
		t.Errorf("client name: got %q", ctx.ClientName)
	}
	if ctx.ClientID != "26" {
		t.Errorf("client id: got %q", ctx.ClientID)
	}
	if ctx.HL != "ja" {
		t.Errorf("hl: got %q", ctx.HL)
	}
	if ctx.ClientVersion == "" {
		t.Error("client version is required")
	}
}

// TestGetSongLyricsLive exercises the real endpoint. It is skipped unless
// YTM_LIVE=1 is set, because it needs network access and the client version can
// be retired by YouTube without notice.
//
//	YTM_LIVE=1 go test -run TestGetSongLyricsLive -v
func TestGetSongLyricsLive(t *testing.T) {
	if os.Getenv("YTM_LIVE") == "" {
		t.Skip("set YTM_LIVE=1 to run the live endpoint check")
	}
	songID := os.Getenv("YTM_SONG")
	if songID == "" {
		songID = "Kr4EQDVETuA" // Michael Jackson - Billie Jean
	}

	client := NewClient()
	ctx := context.Background()

	song, err := client.LoadSong(ctx, songID)
	if err != nil {
		t.Fatalf("LoadSong(%s): %v", songID, err)
	}
	if song.LyricsBrowseID == "" {
		t.Fatalf("song %s has no lyrics browse ID", songID)
	}

	sheet, err := client.GetSongLyricsTimed(ctx, song.LyricsBrowseID)
	if err != nil {
		t.Fatalf("GetSongLyricsTimed(%s): %v", song.LyricsBrowseID, err)
	}
	if len(sheet.Lines) == 0 {
		t.Fatal("no lyric lines")
	}
	t.Logf("synced=%v lines=%d first=%q", sheet.Synced, len(sheet.Lines), sheet.Lines[0].Text)

	if !sheet.Synced {
		t.Error("the mobile profile should return timed lyrics for this track")
	}
	for i := 1; i < len(sheet.Lines); i++ {
		if sheet.Lines[i].StartMs < sheet.Lines[i-1].StartMs {
			t.Errorf("cue ranges go backwards at line %d: %d after %d",
				i, sheet.Lines[i].StartMs, sheet.Lines[i-1].StartMs)
		}
	}
}
