// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Implement lyrics loading, timed and plaintext.
//
// Key Components:
//   - GetSongLyricsTimed: requests and decodes the lyric sheet for a lyrics browse ID
//   - GetSongLyrics: the plaintext view of the same sheet
//   - LyricLine / Lyrics: the decoded sheet, with cue ranges when the endpoint has them
//
// Dependencies:
//   - context
//   - errors
//   - fmt
//   - strings
//
// Error Types:
//   - ErrLyricsNotFound: the response carried no lyric sheet at all
//   - ErrLyricsNotAvailable: the endpoint answered that lyrics are unavailable
//
package ytm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// LyricLine is one line of a lyric sheet.
//
// StartMs and EndMs are zero when the endpoint returned the line without a cue
// range, which happens for tracks whose lyrics it has not timed. A line that
// carries only text is still a usable lyric line: it just cannot be followed.
type LyricLine struct {
	Text    string `json:"text"`
	StartMs int64  `json:"start_ms,omitempty"`
	EndMs   int64  `json:"end_ms,omitempty"`
}

// Lyrics is the lyric sheet resolved for one track.
//
// Synced reports that every line carries a cue range. It is false for a sheet
// that was only available as plain text, and for a timed sheet where the
// endpoint left a line untimed.
type Lyrics struct {
	Lines  []LyricLine `json:"lines,omitempty"`
	Synced bool        `json:"synced"`
}

// PlainText joins the lines the way a plain lyric sheet reads. Blank lines and
// the instrumental markers the endpoint emits are dropped.
func (l *Lyrics) PlainText() string {
	if l == nil {
		return ""
	}
	texts := make([]string, 0, len(l.Lines))
	for _, line := range l.Lines {
		if line.Text == "" || line.Text == "♪" {
			continue
		}
		texts = append(texts, line.Text)
	}
	return strings.Join(texts, "\n")
}

// lyricBrowseResponse is the slice of a lyrics browse body that both the mobile
// and the web client shapes can fill.
//
// The two shapes share nothing but the wrapper: the web client answers with a
// section list and a description shelf, while the mobile clients answer with an
// element renderer carrying a timedLyricsModel. Both can also answer with a
// message renderer when the track has no lyrics.
type lyricBrowseResponse struct {
	Contents struct {
		SectionListRenderer *struct {
			Contents []struct {
				MusicDescriptionShelfRenderer *struct {
					Description TextRuns `json:"description"`
				} `json:"musicDescriptionShelfRenderer"`
			} `json:"contents"`
		} `json:"sectionListRenderer"`

		ElementRenderer *struct {
			NewElement struct {
				Type struct {
					ComponentType struct {
						Model struct {
							TimedLyricsModel  *timedLyricsModel `json:"timedLyricsModel"`
							MusicMessageModel *struct {
								Text string `json:"text"`
							} `json:"musicMessageModel"`
						} `json:"model"`
					} `json:"componentType"`
				} `json:"type"`
			} `json:"newElement"`
		} `json:"elementRenderer"`

		MessageRenderer *struct {
			Text TextRuns `json:"text"`
		} `json:"messageRenderer"`
	} `json:"contents"`
}

// timedLyricsModel is the mobile lyric model: one entry per line.
type timedLyricsModel struct {
	LyricsData struct {
		TimedLyricsData []timedLyricLine `json:"timedLyricsData"`
	} `json:"lyricsData"`
}

// timedLyricLine is one entry of timedLyricsData. The cue bounds arrive as
// strings, which is why they are decoded through flexInt.
type timedLyricLine struct {
	LyricLine string `json:"lyricLine"`
	CueRange  *struct {
		StartTimeMilliseconds flexInt `json:"startTimeMilliseconds"`
		EndTimeMilliseconds   flexInt `json:"endTimeMilliseconds"`
	} `json:"cueRange"`
}

/*
GetSongLyricsTimed retrieves the lyric sheet for the specified lyrics browse ID.

The request is made with the iOS Music client, which is the profile the endpoint
answers with timedLyricsModel: one entry per line, each with a cue range in
milliseconds. The desktop web client is answered with a plaintext description
shelf instead, and that is used as the fallback when the mobile profile returns
nothing usable, so a stale client version degrades to unsynced lyrics rather
than to an error.

    params:
          ctx: execution context
          lyricsID: lyrics browse ID from Song.LyricsBrowseID
    returns:
          *Lyrics: lines, with cue ranges when the endpoint had them
          error: network, parsing, or availability error
*/
func (c *Client) GetSongLyricsTimed(ctx context.Context, lyricsID string) (*Lyrics, error) {
	if lyricsID == "" {
		return nil, fmt.Errorf("%w: empty lyrics ID", ErrLyricsNotFound)
	}

	timed, timedErr := c.fetchLyrics(ctx, lyricsID, GetContextIOSMusic(c.hl))
	if timedErr == nil {
		return timed, nil
	}
	if errors.Is(timedErr, ErrLyricsNotAvailable) {
		// Availability is content-level, so the web shelf would say the same
		// thing. Do not spend a second request to hear it again.
		return nil, timedErr
	}

	// The mobile profile can be refused outright when YouTube retires its client
	// version. The web shelf is plain text, but plain text beats no lyrics.
	plain, plainErr := c.fetchLyrics(ctx, lyricsID, GetContextWebRemix(c.hl))
	if plainErr == nil {
		return plain, nil
	}

	// Prefer the mobile failure: it is the profile the sheet normally comes from.
	return nil, timedErr
}

/*
GetSongLyrics retrieves the plaintext lyrics content for the specified lyrics browse ID.

Timed lines are joined into the same plaintext a plain sheet would give, so a
caller that only needs text does not have to know which shape the sheet had.

    params:
          ctx: execution context
          lyricsID: lyrics browse ID
    returns:
          string: plaintext lyrics
          error: network, parsing, or availability error
*/
func (c *Client) GetSongLyrics(ctx context.Context, lyricsID string) (string, error) {
	sheet, err := c.GetSongLyricsTimed(ctx, lyricsID)
	if err != nil {
		return "", err
	}
	return sheet.PlainText(), nil
}

// fetchLyrics runs the browse call with one client profile and decodes whatever
// sheet shape came back.
func (c *Client) fetchLyrics(ctx context.Context, lyricsID string, clientCtx ClientContext) (*Lyrics, error) {
	var resp lyricBrowseResponse

	err := c.doInnerTube(ctx, "browse", clientCtx, map[string]any{
		"browseId": lyricsID,
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	return resp.sheet()
}

// sheet decodes whichever lyric shape the response carries.
func (r lyricBrowseResponse) sheet() (*Lyrics, error) {
	if el := r.Contents.ElementRenderer; el != nil {
		model := el.NewElement.Type.ComponentType.Model

		if msg := model.MusicMessageModel; msg != nil && msg.Text != "" {
			return nil, fmt.Errorf("%w: %s", ErrLyricsNotAvailable, msg.Text)
		}

		if timed := model.TimedLyricsModel; timed != nil && len(timed.LyricsData.TimedLyricsData) > 0 {
			return timed.sheet(), nil
		}
	}

	if section := r.Contents.SectionListRenderer; section != nil {
		for _, entry := range section.Contents {
			if shelf := entry.MusicDescriptionShelfRenderer; shelf != nil {
				return plainSheet(runsText(shelf.Description)), nil
			}
		}
	}

	if msg := r.Contents.MessageRenderer; msg != nil {
		if text := runsText(msg.Text); text != "" {
			return nil, fmt.Errorf("%w: %s", ErrLyricsNotAvailable, text)
		}
		return nil, ErrLyricsNotAvailable
	}

	return nil, ErrLyricsNotFound
}

// sheet converts the timed model, keeping the sheet unsynced when any line
// arrived without a cue range. That mix is real: some tracks are returned as
// timedLyricsData with text only.
func (t *timedLyricsModel) sheet() *Lyrics {
	out := &Lyrics{Synced: true}
	for _, entry := range t.LyricsData.TimedLyricsData {
		line := LyricLine{Text: strings.TrimSpace(entry.LyricLine)}
		if entry.CueRange == nil {
			out.Synced = false
		} else {
			line.StartMs = int64(entry.CueRange.StartTimeMilliseconds)
			line.EndMs = int64(entry.CueRange.EndTimeMilliseconds)
		}
		out.Lines = append(out.Lines, line)
	}
	if len(out.Lines) == 0 {
		out.Synced = false
	}
	return out
}

// plainSheet turns one block of plaintext into unsynced lines.
func plainSheet(text string) *Lyrics {
	out := &Lyrics{}
	for _, line := range strings.Split(text, "\n") {
		out.Lines = append(out.Lines, LyricLine{Text: strings.TrimSpace(line)})
	}
	return out
}

// runsText concatenates every run. FirstText stops at the first non-empty run,
// which is fine for a title but truncates a shelf whose lyric text is split.
func runsText(runs TextRuns) string {
	var b strings.Builder
	for _, r := range runs.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}
