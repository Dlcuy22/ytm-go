// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Verify mix loading: request shape, seed handling, queue parsing, and paging.
//
// Key Components:
//   - TestGetMixReadsQueuePage: the seed request and the song fields it yields
//   - TestGetMixOmitsSeedForOtherMixIDs: an id with no single seed sends none
//   - TestGetMixFollowsContinuation: a token page asks by continuation alone
//   - TestGetMixRejectsQueuelessResponse: a browse-shaped answer is an error
//   - TestMixVideoID: which ids carry a seed video
//
// Dependencies:
//   - context
//   - net/http
//   - net/http/httptest
//   - testing
//
// Error Types:
//   - None
package ytm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mixClient points a client at a local server with a visitor id set, so the
// request is the one the endpoints see and no bootstrap call is made.
func mixClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	client, server := testClient(t, handler)
	client.SetVisitorID("test_visitor")

	return client, server
}

func TestGetMixReadsQueuePage(t *testing.T) {
	var body map[string]any
	client, server := mixClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/next" {
			t.Errorf("path = %s, want /next", r.URL.Path)
		}
		body = decodeBody(t, r)
		serveFixture(t, "mix.json")(w, r)
	})
	defer server.Close()

	playlist, err := client.GetMix(context.Background(), "RDLoK17z6xDwI", nil)
	if err != nil {
		t.Fatalf("GetMix: %v", err)
	}
	if body["playlistId"] != "RDLoK17z6xDwI" || body["videoId"] != "LoK17z6xDwI" {
		t.Fatalf("body = %v, want the mix id and its seed video", body)
	}
	if len(playlist.Items) != 3 {
		t.Fatalf("items = %d, want 3 (the wrapped renderer included)", len(playlist.Items))
	}

	first := playlist.Items[0]
	if first.ID != "LoK17z6xDwI" || first.Name != "風薫る空の下" {
		t.Fatalf("first = %q %q, want the seed track", first.ID, first.Name)
	}
	if len(first.Artists) != 1 || first.Artists[0].Name != "TUYU" {
		t.Fatalf("artists = %+v, want TUYU", first.Artists)
	}
	if first.DurationMs != 203000 {
		t.Fatalf("duration = %d, want 203000", first.DurationMs)
	}
	if first.Thumbnail == nil || first.Thumbnail.GetThumbnailURL(ThumbnailQualityHigh) == "" {
		t.Fatalf("thumbnail = %+v, want a resolvable still", first.Thumbnail)
	}

	cont := playlist.Continuation
	if cont == nil || cont.Type != ContinuationMix || cont.ItemID != "RDLoK17z6xDwI" || cont.Token == "" {
		t.Fatalf("continuation = %+v, want a mix token for the playlist", cont)
	}
}

func TestGetMixOmitsSeedForOtherMixIDs(t *testing.T) {
	var body map[string]any
	client, server := mixClient(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		serveFixture(t, "mix.json")(w, r)
	})
	defer server.Close()

	// An automix id names its seed inside a longer marker, not as the whole
	// suffix, so the queue has to come from the playlist id alone.
	if _, err := client.GetMix(context.Background(), "RDAMVMabcdefghijk", nil); err != nil {
		t.Fatalf("GetMix: %v", err)
	}
	if _, ok := body["videoId"]; ok {
		t.Fatalf("body = %v, want no seed video", body)
	}
}

func TestGetMixFollowsContinuation(t *testing.T) {
	calls := 0
	token := "mix_page_2"
	client, server := mixClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		body := decodeBody(t, r)
		if body["continuation"] != token {
			t.Errorf("call %d body = %v, want the continuation token alone", calls, body)
		}
		if _, ok := body["playlistId"]; ok {
			t.Errorf("call %d sent the playlist id on a continuation", calls)
		}
		serveFixture(t, "mix_continuation.json")(w, r)
	})
	defer server.Close()

	playlist, err := client.GetMix(context.Background(), "RDLoK17z6xDwI", &BuiltInContinuation{
		Token:  token,
		Type:   ContinuationMix,
		ItemID: "RDLoK17z6xDwI",
	})
	if err != nil {
		t.Fatalf("GetMix continuation: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if len(playlist.Items) != 2 || playlist.Items[0].ID != "K8GZ8SoNfmE" {
		t.Fatalf("items = %+v, want the continuation page", playlist.Items)
	}
	if playlist.Continuation == nil || playlist.Continuation.Token == "" {
		t.Fatalf("continuation = %+v, want the next token", playlist.Continuation)
	}
}

func TestGetMixRejectsQueuelessResponse(t *testing.T) {
	client, server := mixClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"contents":{}}`))
	})
	defer server.Close()

	if _, err := client.GetMix(context.Background(), "RDLoK17z6xDwI", nil); err == nil {
		t.Fatal("GetMix on a response with no queue must fail")
	}
}

func TestGetMixRejectsOtherPlaylistIDs(t *testing.T) {
	client, server := mixClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("a plain playlist id must not reach the network")
	})
	defer server.Close()

	if _, err := client.GetMix(context.Background(), "PLabcdefghijk", nil); err == nil {
		t.Fatal("GetMix on a plain playlist id must fail")
	}
}

func TestIsMixID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"RDLoK17z6xDwI", true},
		{"RDAMVMabcdefghijk", true},
		{"  RDEMabcdefghijk  ", true},
		{"PLabcdefghijk", false},
		{"MPREb_album", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsMixID(c.in); got != c.want {
			t.Errorf("IsMixID(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMixVideoID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"RDLoK17z6xDwI", "LoK17z6xDwI"},
		{"RDAMVMabcdefghijk", ""},
		{"RDCLAK5uy_lspe1yM2aG", ""},
		{"RDATrXvabcdefghijk", ""},
		{"PLabcdefghijk", ""},
		{"RDshort", ""},
		{"RDabcdefghij!", ""},
	}
	for _, c := range cases {
		if got := mixVideoID(c.in); got != c.want {
			t.Errorf("mixVideoID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
