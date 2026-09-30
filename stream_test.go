// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Verify stream resolution: codec selection ranking, format parsing, and
//   expiry handling, plus an opt-in live check against a real track.
//
// Key Components:
//   - TestSelectAudioFormat*: the ranking that implements codec preference
//   - TestToStreamFormat*: raw format parsing and rejection rules
//   - TestFlexInt*: the tolerant numeric unmarshalling
//   - TestGetStreamLive: opt-in network check, skipped by default
//
// Dependencies:
//   - context
//   - os
//   - testing
//   - time
//
// Error Types:
//   - None
package ytm

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// sampleFormats mirrors the audio formats the VISIONOS profile returns for a
// typical music track, in the order the endpoint lists them: ascending bitrate,
// AAC and Opus interleaved.
func sampleFormats() []StreamFormat {
	return []StreamFormat{
		{Itag: 139, MimeType: `audio/mp4; codecs="mp4a.40.5"`, Codec: CodecAAC, Bitrate: 50521, SampleRate: 22050, Channels: 2},
		{Itag: 249, MimeType: `audio/webm; codecs="opus"`, Codec: CodecOpus, Bitrate: 51984, SampleRate: 48000, Channels: 2},
		{Itag: 250, MimeType: `audio/webm; codecs="opus"`, Codec: CodecOpus, Bitrate: 69201, SampleRate: 48000, Channels: 2},
		{Itag: 140, MimeType: `audio/mp4; codecs="mp4a.40.2"`, Codec: CodecAAC, Bitrate: 131269, SampleRate: 44100, Channels: 2},
		{Itag: 251, MimeType: `audio/webm; codecs="opus"`, Codec: CodecOpus, Bitrate: 135891, SampleRate: 48000, Channels: 2},
	}
}

// TestSelectAudioFormatDefaultsToOpus pins the default: no options means the
// highest-bitrate Opus rung, which is itag 251 for music.
func TestSelectAudioFormatDefaultsToOpus(t *testing.T) {
	got := selectAudioFormat(sampleFormats(), StreamOptions{})
	if len(got) == 0 {
		t.Fatal("no formats selected")
	}
	if got[0].Itag != 251 {
		t.Errorf("default should pick itag 251, got %d", got[0].Itag)
	}
	if got[0].Codec != CodecOpus {
		t.Errorf("default should pick opus, got %q", got[0].Codec)
	}
}

// TestSelectAudioFormatAACWinsWhenAsked proves an explicit codec preference
// overrides the default, even though AAC is the lower bitrate here.
func TestSelectAudioFormatAACWinsWhenAsked(t *testing.T) {
	got := selectAudioFormat(sampleFormats(), StreamOptions{Codec: CodecAAC})
	if len(got) == 0 {
		t.Fatal("no formats selected")
	}
	if got[0].Itag != 140 {
		t.Errorf("AAC preference should pick itag 140, got %d", got[0].Itag)
	}
}

// TestSelectAudioFormatBitrateCapKeepsCodec verifies a cap narrows within the
// codec family instead of silently switching families.
func TestSelectAudioFormatBitrateCapKeepsCodec(t *testing.T) {
	got := selectAudioFormat(sampleFormats(), StreamOptions{Codec: CodecOpus, MaxBitrate: 100000})
	if len(got) == 0 {
		t.Fatal("no formats selected")
	}
	if got[0].Itag != 250 {
		t.Errorf("cap should pick the 69 kbps opus rung (250), got %d", got[0].Itag)
	}
}

// TestSelectAudioFormatIgnoresImpossibleCap documents the deliberate fallback:
// a cap that excludes everything is dropped rather than returning no stream.
func TestSelectAudioFormatIgnoresImpossibleCap(t *testing.T) {
	got := selectAudioFormat(sampleFormats(), StreamOptions{Codec: CodecOpus, MaxBitrate: 1000})
	if len(got) == 0 {
		t.Fatal("an impossible cap must not empty the result")
	}
	if got[0].Itag != 251 {
		t.Errorf("cap should be ignored, expected 251, got %d", got[0].Itag)
	}
}

// TestSelectAudioFormatDoesNotMutateInput guards the copy: the caller's slice
// order is part of its contract with the endpoint, and reordering it in place
// would be a surprising side effect.
func TestSelectAudioFormatDoesNotMutateInput(t *testing.T) {
	in := sampleFormats()
	firstBefore := in[0].Itag
	_ = selectAudioFormat(in, StreamOptions{})
	if in[0].Itag != firstBefore {
		t.Errorf("input slice was reordered: first is now %d", in[0].Itag)
	}
}

// TestSelectAudioFormatRanksUnknownCodecLast checks that a codec outside the
// known families does not outrank a known one under CodecAuto.
func TestSelectAudioFormatRanksUnknownCodecLast(t *testing.T) {
	formats := []StreamFormat{
		{Itag: 1, Codec: AudioCodec("vorbis"), Bitrate: 999999, SampleRate: 48000},
		{Itag: 251, Codec: CodecOpus, Bitrate: 135891, SampleRate: 48000},
	}
	got := selectAudioFormat(formats, StreamOptions{})
	if got[0].Itag != 251 {
		t.Errorf("known codec should outrank unknown, got itag %d", got[0].Itag)
	}
}

// TestToStreamFormatRejectsCipher proves a signatureCipher format is dropped
// rather than offered as if its URL were usable.
func TestToStreamFormatRejectsCipher(t *testing.T) {
	_, ok := toStreamFormat(playerFormat{
		Itag:            251,
		MimeType:        `audio/webm; codecs="opus"`,
		SignatureCipher: "url=https%3A%2F%2Fexample&s=abc",
	})
	if ok {
		t.Error("a ciphered format must not be returned as playable")
	}
}

// TestToStreamFormatRejectsVideo checks that a muxed A/V format is filtered out
// by its width even when the MIME type is video.
func TestToStreamFormatRejectsVideo(t *testing.T) {
	if _, ok := toStreamFormat(playerFormat{
		Itag: 18, MimeType: "video/mp4", Width: 640, URL: "https://example",
	}); ok {
		t.Error("a video format must not be returned as audio")
	}
}

// TestToStreamFormatParsesFields checks the field mapping and the string
// contentLength conversion.
func TestToStreamFormatParsesFields(t *testing.T) {
	got, ok := toStreamFormat(playerFormat{
		Itag:          251,
		URL:           "https://rr1---sn-x.googlevideo.com/videoplayback?expire=1790777639&itag=251",
		MimeType:      `audio/webm; codecs="opus"`,
		Bitrate:       135891,
		ContentLength: 3672328,
		AudioSampleRt: 48000,
		AudioChannels: 2,
	})
	if !ok {
		t.Fatal("valid audio format was rejected")
	}
	if got.Codec != CodecOpus {
		t.Errorf("codec: got %q", got.Codec)
	}
	if got.ContentLength != 3672328 {
		t.Errorf("content length: got %d", got.ContentLength)
	}
	if got.ExpiresAt.Unix() != 1790777639 {
		t.Errorf("expiry: got %v", got.ExpiresAt)
	}
}

// TestFlexIntAcceptsStringAndNumber is the regression guard for the real
// response shape: audioSampleRate arrives as a string, bitrate as a number, and
// a plain int field makes the whole response fail to unmarshal.
func TestFlexIntAcceptsStringAndNumber(t *testing.T) {
	var f playerFormat
	raw := `{"itag":251,"bitrate":135891,"audioSampleRate":"48000","audioChannels":2,"contentLength":"3672328"}`
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(f.Itag) != 251 {
		t.Errorf("itag: got %d", f.Itag)
	}
	if int(f.Bitrate) != 135891 {
		t.Errorf("bitrate: got %d", f.Bitrate)
	}
	if int(f.AudioSampleRt) != 48000 {
		t.Errorf("audioSampleRate from string: got %d", f.AudioSampleRt)
	}
	if int64(f.ContentLength) != 3672328 {
		t.Errorf("contentLength from string: got %d", f.ContentLength)
	}
}

// TestFlexIntToleratesMissingAndNull keeps the zero-value contract: an absent
// or null numeric field is unknown, not a parse failure.
func TestFlexIntToleratesMissingAndNull(t *testing.T) {
	var f playerFormat
	if err := json.Unmarshal([]byte(`{"itag":251,"audioSampleRate":null}`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if int(f.AudioSampleRt) != 0 {
		t.Errorf("null sample rate should be 0, got %d", f.AudioSampleRt)
	}
	if int(f.Bitrate) != 0 {
		t.Errorf("absent bitrate should be 0, got %d", f.Bitrate)
	}
}

// TestExpiryOfHandlesMissingParam keeps the zero-time contract: an unknown
// expiry must not look expired.
func TestExpiryOfHandlesMissingParam(t *testing.T) {
	if !expiryOf("https://example/videoplayback?itag=251").IsZero() {
		t.Error("a URL without expire should yield the zero time")
	}
	if !expiryOf("://not a url").IsZero() {
		t.Error("an unparseable URL should yield the zero time")
	}
}

// TestStreamFormatExpired pins both halves of Expired, including that an
// unknown expiry is never treated as expired.
func TestStreamFormatExpired(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"future", now.Add(time.Hour), false},
		{"past", now.Add(-time.Hour), true},
		{"unknown", time.Time{}, false},
	}
	for _, c := range cases {
		f := StreamFormat{ExpiresAt: c.at}
		if got := f.Expired(now); got != c.want {
			t.Errorf("%s: Expired = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestCodecOf classifies the two real MIME shapes and a codecless one.
func TestCodecOf(t *testing.T) {
	cases := map[string]AudioCodec{
		`audio/webm; codecs="opus"`:     CodecOpus,
		`audio/mp4; codecs="mp4a.40.2"`: CodecAAC,
		`audio/mp4; codecs="mp4a.40.5"`: CodecAAC,
		`audio/ogg; codecs="vorbis"`:    AudioCodec(`audio/ogg; codecs="vorbis"`),
	}
	for mime, want := range cases {
		if got := codecOf(mime); got != want {
			t.Errorf("codecOf(%q) = %q, want %q", mime, got, want)
		}
	}
}

// TestGetStreamLive exercises the real endpoint. It is skipped unless
// YTM_LIVE=1 is set, because it needs network access and the profile can be
// rotated by YouTube without notice.
//
//	YTM_LIVE=1 go test -run TestGetStreamLive -v
func TestGetStreamLive(t *testing.T) {
	if os.Getenv("YTM_LIVE") == "" {
		t.Skip("set YTM_LIVE=1 to run the live endpoint check")
	}
	songID := os.Getenv("YTM_SONG")
	if songID == "" {
		songID = "gnkOESS2qs8" // TUYU - それでも雨は降るんだね
	}

	client := NewClient()
	streams, err := client.GetStream(context.Background(), songID)
	if err != nil {
		t.Fatalf("GetStream(%s): %v", songID, err)
	}
	if streams.VideoID != songID {
		t.Errorf("video id: got %q, want %q", streams.VideoID, songID)
	}
	if streams.DurationMs <= 0 {
		t.Errorf("duration should be positive, got %d", streams.DurationMs)
	}

	best, ok := streams.Best()
	if !ok {
		t.Fatal("no best format")
	}
	t.Logf("title=%q author=%q duration=%dms", streams.Title, streams.Author, streams.DurationMs)
	t.Logf("best: itag=%d codec=%s bitrate=%d sr=%d ch=%d len=%d",
		best.Itag, best.Codec, best.Bitrate, best.SampleRate, best.Channels, best.ContentLength)
	t.Logf("url: %.80s...", best.URL)

	if best.Codec != CodecOpus {
		t.Errorf("default codec: got %q, want opus", best.Codec)
	}
	if best.URL == "" {
		t.Error("best format has no URL")
	}
	if best.ContentLength <= 0 {
		t.Error("best format has no content length")
	}
	if best.ExpiresAt.IsZero() {
		t.Error("best format has no expiry")
	}
	if streams.HLSManifestURL == "" {
		t.Log("note: endpoint returned no HLS manifest for this track")
	}
}
