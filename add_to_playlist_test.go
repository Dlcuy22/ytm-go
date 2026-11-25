package ytm

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
)

func TestGetAddToPlaylistTargetsFixtureAndRequest(t *testing.T) {
	fixture, err := os.ReadFile("testdata/add_to_playlist.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	clk := newFakeClock()
	var gotPath string
	var gotBody map[string]any
	var gotCookie bool

	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCookie = r.Header.Get("Cookie") != ""
		gotBody = decodeBody(t, r)
		w.Write(fixture)
	})
	defer server.Close()
	client.nowFn = clk.Now
	client.sleepFn = clk.Sleep

	targets, err := client.GetAddToPlaylistTargets(context.Background(), "MPEDdQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("GetAddToPlaylistTargets: %v", err)
	}
	if gotPath != "/playlist/get_add_to_playlist" {
		t.Errorf("path: got %q, want /playlist/get_add_to_playlist", gotPath)
	}
	if !gotCookie {
		t.Error("expected Cookie header")
	}
	if len(clk.Sleeps()) != 0 {
		t.Errorf("read endpoint must not trigger write pacing, slept %v", clk.Sleeps())
	}
	if vids, _ := gotBody["videoIds"].([]any); len(vids) != 1 || vids[0] != "dQw4w9WgXcQ" {
		t.Errorf("videoIds: got %v, want [dQw4w9WgXcQ]", gotBody["videoIds"])
	}

	want := []AddToPlaylistTarget{
		{
			PlaylistID: "LM",
			Title:      "Liked music",
			Subtitle:   "14 songs",
			Thumbnail:  "https://example.invalid/thumb/lm-192.png",
		},
		{
			PlaylistID: "PLtestPlaylist01",
			Title:      "Road Trip Mix",
			Subtitle:   "9 tracks",
			Thumbnail:  "https://example.invalid/thumb/pl1-88.jpg",
		},
		{
			PlaylistID: "PLtestPlaylist02",
			Title:      "Late Night",
			Subtitle:   "3 tracks",
			Thumbnail:  "",
		},
	}
	if len(targets) != len(want) {
		t.Fatalf("got %d targets, want %d: %+v", len(targets), len(want), targets)
	}
	for i := range want {
		if targets[i] != want[i] {
			t.Errorf("target[%d]: got %+v, want %+v", i, targets[i], want[i])
		}
	}
}

func TestGetAddToPlaylistTargetsValidation(t *testing.T) {
	anon := NewClient()
	if _, err := anon.GetAddToPlaylistTargets(context.Background(), "vid1"); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("unauthed: got %v, want ErrLoginRequired", err)
	}

	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()
	if _, err := client.GetAddToPlaylistTargets(context.Background(), "   "); err == nil {
		t.Fatal("expected error for empty videoID")
	}
}
