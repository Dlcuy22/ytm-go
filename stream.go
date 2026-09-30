// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Resolve playable audio streams for a track without shelling out to yt-dlp.
//
// Key Components:
//   - StreamFormat: one selectable audio representation from /player
//   - StreamOptions / AudioCodec: how a caller asks for a codec
//   - GetStream / GetStreamWithOptions: /player against the VISIONOS profile
//   - selectAudioFormat: the ranking that implements the codec preference
//
// Dependencies:
//   - context
//   - fmt
//   - net/url
//   - sort
//   - strconv
//   - strings
//   - time
//
// Error Types:
//   - ErrStreamNotFound: no audio format survived selection
//   - ErrContentNotAvailable: the track is not playable for this client
package ytm

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AudioCodec names a container/codec family a caller can ask for. The values
// are deliberately coarse: a caller picks a family, and selectAudioFormat
// picks the best bitrate inside it.
type AudioCodec string

const (
	// CodecAuto prefers Opus in WebM and falls back to AAC in MP4. It is the
	// zero value, so an unset StreamOptions means "best available".
	CodecAuto AudioCodec = ""
	// CodecOpus is audio/webm; codecs="opus". The default in practice: 48 kHz
	// stereo at the highest bitrate YouTube offers for music.
	CodecOpus AudioCodec = "opus"
	// CodecAAC is audio/mp4; codecs="mp4a.*", the AAC family.
	CodecAAC AudioCodec = "aac"
)

// StreamOptions tunes stream resolution.
type StreamOptions struct {
	// Codec selects the container/codec family. Empty means CodecAuto.
	Codec AudioCodec

	// MaxBitrate caps the chosen format in bits per second. Zero means no cap.
	// It is applied after the codec filter, so a cap never silently switches
	// the codec family.
	MaxBitrate int

	// ClientName overrides the impersonated playback client. Empty uses
	// VISIONOS, which is the profile that returns direct media URLs. It exists
	// so a caller can benchmark an alternative without editing this package.
	ClientName string
}

// StreamFormat is one audio representation from a /player response.
//
// It is a flat value type on purpose: it is serialised into caches and handed
// to a downloader, so it must not point back into the response tree.
type StreamFormat struct {
	// Itag is YouTube's format identifier (251 is the usual Opus music rung).
	Itag int `json:"itag"`
	// MimeType is the full MIME type, codecs parameter included, exactly as the
	// endpoint reported it.
	MimeType string `json:"mime_type"`
	// Codec is the coarse family derived from MimeType: "opus" or "aac".
	Codec AudioCodec `json:"codec"`
	// Bitrate is the average bitrate in bits per second.
	Bitrate int `json:"bitrate"`
	// SampleRate and Channels describe the decoded audio.
	SampleRate int `json:"sample_rate,omitempty"`
	Channels   int `json:"channels,omitempty"`
	// ContentLength is the byte size of the representation, or 0 when unknown.
	ContentLength int64 `json:"content_length,omitempty"`
	// URL is the signed media URL. It is already authorised for this client:
	// no signature cipher and no PO token apply.
	//
	// One gotcha travels with this URL: googlevideo throttles a GET that has no
	// Range header to roughly 32 KiB/s. Always request it with a Range, even
	// for the whole file ("bytes=0-"), which measured ~80x faster on the same
	// track. See cmd/e2e for the measured numbers.
	URL string `json:"url"`
	// ExpiresAt is when URL stops working. Signed URLs are also bound to the
	// requesting IP, so a cached StreamFormat is only reusable from the same
	// host and before this instant.
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// Expired reports whether the signed URL is past its expiry. An unknown expiry
// (zero time) is treated as not expired, because the endpoint did not say.
func (f StreamFormat) Expired(now time.Time) bool {
	return !f.ExpiresAt.IsZero() && now.After(f.ExpiresAt)
}

// Streams is the resolved playback information for one track.
type Streams struct {
	// VideoID is the track the formats belong to.
	VideoID string `json:"video_id"`
	// Title, Author and DurationMs come from videoDetails and are convenient
	// for a UI that has only an ID to start from.
	Title      string `json:"title,omitempty"`
	Author     string `json:"author,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	// HLSManifestURL is set when the endpoint offered an HLS master playlist.
	// It is a fallback transport, not the default: the direct URL is cheaper.
	HLSManifestURL string `json:"hls_manifest_url,omitempty"`
	// Formats are every audio format the endpoint returned, best first under
	// the requested preference. The selected one is Formats[0] when non-empty.
	Formats []StreamFormat `json:"formats,omitempty"`
}

// Best returns the format selection settled on, which is the first entry.
func (s *Streams) Best() (StreamFormat, bool) {
	if s == nil || len(s.Formats) == 0 {
		return StreamFormat{}, false
	}
	return s.Formats[0], true
}

// playerResponse is the slice of the /player body streaming needs. Everything
// else the endpoint sends is ignored, which keeps this decoupled from the
// catalogue models.
type playerResponse struct {
	PlayabilityStatus struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"playabilityStatus"`
	StreamingData struct {
		Formats         []playerFormat `json:"formats"`
		AdaptiveFormats []playerFormat `json:"adaptiveFormats"`
		HLSManifestURL  string         `json:"hlsManifestUrl"`
	} `json:"streamingData"`
	VideoDetails struct {
		VideoID       string `json:"videoId"`
		Title         string `json:"title"`
		Author        string `json:"author"`
		LengthSeconds string `json:"lengthSeconds"`
	} `json:"videoDetails"`
}

// playerFormat is one entry of formats/adaptiveFormats. Only the fields stream
// selection reads are declared.
//
// The numeric fields use flexInt/flexInt64 because YouTube is inconsistent
// about their JSON type: audioSampleRate arrives as a string on some formats
// and a number on others, and contentLength is a string on every format. A
// plain int field makes the whole response fail to unmarshal.
type playerFormat struct {
	Itag          flexInt   `json:"itag"`
	URL           string    `json:"url"`
	MimeType      string    `json:"mimeType"`
	Bitrate       flexInt   `json:"bitrate"`
	ContentLength flexInt64 `json:"contentLength"`
	AudioSampleRt flexInt   `json:"audioSampleRate"`
	AudioChannels flexInt   `json:"audioChannels"`
	// Width is present only on video formats, which is how audio is told apart
	// when the MIME type is ambiguous.
	Width flexInt `json:"width"`
	// SignatureCipher means the URL is not directly usable. This package does
	// not implement the cipher, so such a format is dropped rather than
	// returned half-usable.
	SignatureCipher string `json:"signatureCipher"`
}

// flexInt is an int that accepts a JSON number or a numeric string. An absent
// or unparseable value becomes 0, which callers already treat as unknown.
type flexInt int

func (f *flexInt) UnmarshalJSON(data []byte) error {
	n, err := parseFlexibleInt(data)
	if err != nil {
		return nil // unknown, not fatal: the field is advisory
	}
	*f = flexInt(n)
	return nil
}

// flexInt64 is flexInt for 64-bit values such as contentLength.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	n, err := parseFlexibleInt(data)
	if err != nil {
		return nil
	}
	*f = flexInt64(n)
	return nil
}

// parseFlexibleInt accepts a bare number, a quoted number, or null.
func parseFlexibleInt(data []byte) (int64, error) {
	s := strings.TrimSpace(string(data))
	if s == "" || s == "null" {
		return 0, fmt.Errorf("empty")
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	return strconv.ParseInt(s, 10, 64)
}

/*
GetStream resolves the playable audio stream for a song ID using the default
preference: the highest-bitrate Opus format in WebM.

    params:
          ctx: execution context
          songID: YouTube track ID (an "MPED" prefix is stripped)
    returns:
          *Streams: resolved formats, best first
          error: network, parsing, or availability error
*/
func (c *Client) GetStream(ctx context.Context, songID string) (*Streams, error) {
	return c.GetStreamWithOptions(ctx, songID, StreamOptions{})
}

/*
GetStreamWithOptions resolves playable audio streams for a song ID.

The request is made with the VISIONOS client context, which is the profile that
currently answers with plain media URLs. It is anonymous by design: the profile
cannot log in, so this call never sends the account cookie and works the same
whether or not SetAuth was called.

    params:
          ctx: execution context
          songID: YouTube track ID (an "MPED" prefix is stripped)
          opts: codec preference and bitrate cap
    returns:
          *Streams: resolved formats, best first
          error: network, parsing, or availability error
*/
func (c *Client) GetStreamWithOptions(ctx context.Context, songID string, opts StreamOptions) (*Streams, error) {
	songID = CleanSongID(songID)
	if songID == "" {
		return nil, fmt.Errorf("ytm: GetStream requires a song ID")
	}

	clientCtx := GetContextVisionOS(c.hl)
	if opts.ClientName != "" {
		clientCtx.ClientName = opts.ClientName
		clientCtx.ClientID = ""
	}

	var resp playerResponse
	// authed=false is deliberate: VISIONOS is a playback-only profile that does
	// not accept a login, and sending the cookie would not help.
	err := c.doInnerTube(ctx, "player", clientCtx, map[string]any{
		"videoId": songID,
	}, false, &resp)
	if err != nil {
		return nil, err
	}

	status := resp.PlayabilityStatus.Status
	if status != "OK" {
		// UNPLAYABLE and LOGIN_REQUIRED are the two ways a track is refused.
		// The reason string is the only useful detail the endpoint gives.
		if resp.PlayabilityStatus.Reason != "" {
			return nil, fmt.Errorf("%w: %s: %s", ErrContentNotAvailable, status, resp.PlayabilityStatus.Reason)
		}
		return nil, fmt.Errorf("%w: %s", ErrContentNotAvailable, status)
	}

	out := &Streams{
		VideoID:        firstNonEmpty(resp.VideoDetails.VideoID, songID),
		Title:          resp.VideoDetails.Title,
		Author:         resp.VideoDetails.Author,
		HLSManifestURL: resp.StreamingData.HLSManifestURL,
	}
	if secs, err := strconv.ParseInt(resp.VideoDetails.LengthSeconds, 10, 64); err == nil {
		out.DurationMs = secs * 1000
	}

	all := make([]playerFormat, 0, len(resp.StreamingData.Formats)+len(resp.StreamingData.AdaptiveFormats))
	all = append(all, resp.StreamingData.Formats...)
	all = append(all, resp.StreamingData.AdaptiveFormats...)

	candidates := make([]StreamFormat, 0, len(all))
	for _, f := range all {
		if sf, ok := toStreamFormat(f); ok {
			candidates = append(candidates, sf)
		}
	}
	if len(candidates) == 0 {
		return nil, ErrStreamNotFound
	}

	out.Formats = selectAudioFormat(candidates, opts)
	if len(out.Formats) == 0 {
		return nil, ErrStreamNotFound
	}

	return out, nil
}

// toStreamFormat converts one raw format to a StreamFormat, reporting false for
// entries that are not directly playable audio.
func toStreamFormat(f playerFormat) (StreamFormat, bool) {
	if f.URL == "" || f.SignatureCipher != "" {
		// A ciphered format needs the JS player to unwrap; this package does not
		// do that, so it is not offered as if it were ready to use.
		return StreamFormat{}, false
	}
	if !strings.HasPrefix(f.MimeType, "audio/") {
		return StreamFormat{}, false
	}
	if f.Width != 0 {
		// A muxed A/V format is not what a music player wants.
		return StreamFormat{}, false
	}

	sf := StreamFormat{
		Itag:          int(f.Itag),
		MimeType:      f.MimeType,
		Codec:         codecOf(f.MimeType),
		Bitrate:       int(f.Bitrate),
		SampleRate:    int(f.AudioSampleRt),
		Channels:      int(f.AudioChannels),
		ContentLength: int64(f.ContentLength),
		URL:           f.URL,
		ExpiresAt:     expiryOf(f.URL),
	}
	return sf, true
}

// codecOf maps a MIME type to the coarse family a caller asks for.
func codecOf(mimeType string) AudioCodec {
	lower := strings.ToLower(mimeType)
	switch {
	case strings.Contains(lower, "opus"):
		return CodecOpus
	case strings.Contains(lower, "mp4a"), strings.Contains(lower, "aac"):
		return CodecAAC
	default:
		return AudioCodec(lower)
	}
}

// expiryOf reads the expire parameter out of a signed media URL. It returns the
// zero time when the parameter is absent or unparseable.
func expiryOf(raw string) time.Time {
	u, err := url.Parse(raw)
	if err != nil {
		return time.Time{}
	}
	value := u.Query().Get("expire")
	if value == "" {
		return time.Time{}
	}
	secs, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(secs, 0)
}

// selectAudioFormat orders candidates by the requested preference. It always
// returns a copy: the caller's slice is not reordered.
//
// Ranking, in order:
//  1. codec family, by the preference in opts
//  2. bitrate, descending
//  3. sample rate, descending, as a tie-break between equal bitrates
//
// A MaxBitrate cap is applied as a filter before ranking, but only if it leaves
// at least one format; otherwise the cap is ignored rather than failing the
// call, because a playable stream beats no stream.
func selectAudioFormat(candidates []StreamFormat, opts StreamOptions) []StreamFormat {
	pool := make([]StreamFormat, len(candidates))
	copy(pool, candidates)

	if opts.MaxBitrate > 0 {
		capped := make([]StreamFormat, 0, len(pool))
		for _, f := range pool {
			if f.Bitrate > 0 && f.Bitrate <= opts.MaxBitrate {
				capped = append(capped, f)
			}
		}
		if len(capped) > 0 {
			pool = capped
		}
	}

	rank := codecRank(opts.Codec)
	sort.SliceStable(pool, func(i, j int) bool {
		ri, rj := rank(pool[i].Codec), rank(pool[j].Codec)
		if ri != rj {
			return ri < rj
		}
		if pool[i].Bitrate != pool[j].Bitrate {
			return pool[i].Bitrate > pool[j].Bitrate
		}
		if pool[i].SampleRate != pool[j].SampleRate {
			return pool[i].SampleRate > pool[j].SampleRate
		}
		return pool[i].Itag < pool[j].Itag
	})
	return pool
}

// codecRank returns a score for one codec under a preference. A lower score is
// better, so the sort is ascending.
func codecRank(want AudioCodec) func(AudioCodec) int {
	order := []AudioCodec{CodecOpus, CodecAAC}
	switch want {
	case CodecOpus, CodecAAC:
		order = []AudioCodec{want}
	default:
		// CodecAuto: Opus first because it is the higher-quality music encode
		// and decodes natively at 48 kHz in this project.
	}
	return func(got AudioCodec) int {
		for i, c := range order {
			if got == c {
				return i
			}
		}
		return len(order)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
