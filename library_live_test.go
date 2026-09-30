// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Opt-in live checks for the library surface, using real credentials read from
//   ~/.ytm-cookie. Skipped unless YTM_LIVE=1.
//
// The credential file uses the labelled export format:
//
//	***INNERTUBE COOKIE*** ...
//	***VISITOR DATA*** ...
//	***DATASYNC ID*** =...
//	***AUTH USER*** =...
//
// No test here prints a secret, and the write test restores the state it found.
//
// Key Components:
//   - loadLiveClient: builds a Client from the credential file
//   - TestGetLikedSongsLive: first page plus one continuation page
//   - TestGetSongFeedLive: the home feed carries real rows
//   - TestLibraryRoundTripLive: remove then re-add, ending where it started
//
// Dependencies:
//   - bufio
//   - context
//   - os
//   - strings
//   - testing
//
// Error Types:
//   - None
package ytm

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
)

// liveCreds is the parsed credential file. It is never logged.
type liveCreds struct {
	cookie  string
	visitor string
	name    string
}

func loadLiveCreds(t *testing.T) liveCreds {
	t.Helper()
	path := os.Getenv("YTM_COOKIE_FILE")
	if path == "" {
		path = os.ExpandEnv("$HOME/.ytm-cookie")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Skipf("no credential file at %s: %v", path, err)
	}
	defer file.Close()

	creds := liveCreds{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	for scanner.Scan() {
		line := scanner.Text()
		open := strings.Index(line, "***")
		if open < 0 {
			continue
		}
		rest := line[open+3:]
		closeIdx := strings.Index(rest, "***")
		if closeIdx < 0 {
			continue
		}
		label := strings.TrimSpace(rest[:closeIdx])
		value := strings.TrimSpace(rest[closeIdx+3:])
		switch label {
		case "INNERTUBE COOKIE":
			creds.cookie = value
		case "VISITOR DATA":
			creds.visitor = value
		case "ACCOUNT NAME":
			creds.name = strings.TrimPrefix(value, "=")
		}
	}
	if creds.cookie == "" {
		t.Skip("credential file has no INNERTUBE COOKIE line")
	}
	return creds
}

func liveClient(t *testing.T) (*Client, liveCreds) {
	t.Helper()
	creds := loadLiveCreds(t)
	auth, err := NewAuthFromCookieString(creds.cookie)
	if err != nil {
		t.Fatalf("cookie is missing SAPISID: %v", err)
	}
	client := NewClient()
	client.SetAuth(auth)
	if creds.visitor != "" {
		client.SetVisitorID(creds.visitor)
	}
	return client, creds
}

// TestGetLikedSongsLive reads the first library page and one continuation page.
func TestGetLikedSongsLive(t *testing.T) {
	if os.Getenv("YTM_LIVE") == "" {
		t.Skip("set YTM_LIVE=1 to run live checks")
	}
	client, creds := liveClient(t)
	ctx := context.Background()

	songs, cont, err := client.GetLikedSongs(ctx)
	if err != nil {
		t.Fatalf("GetLikedSongs: %v", err)
	}
	t.Logf("account %q: first page returned %d songs, continuation=%v",
		creds.name, len(songs), cont != nil)
	if len(songs) == 0 {
		t.Fatal("expected at least one liked song")
	}
	for i, s := range songs[:min(3, len(songs))] {
		t.Logf("  [%d] %s  %s  (%dms)", i, s.ID, s.Name, s.DurationMs)
	}
	if songs[0].ID == "" {
		t.Error("first song has no ID")
	}
	if songs[0].Name == "" {
		t.Error("first song has no name")
	}

	if cont == nil {
		t.Log("library fits on one page; nothing to page")
		return
	}
	page2, cont2, err := client.GetLikedSongsContinuation(ctx, cont.Token)
	if err != nil {
		t.Fatalf("GetLikedSongsContinuation: %v", err)
	}
	t.Logf("second page returned %d songs, continuation=%v", len(page2), cont2 != nil)
	if len(page2) == 0 {
		t.Error("a non-nil continuation should yield more songs")
	}
	if len(page2) > 0 && len(songs) > 0 && page2[0].ID == songs[0].ID {
		t.Error("second page repeated the first page")
	}
}

// TestGetSongFeedLive checks the home feed carries real rows and chips.
func TestGetSongFeedLive(t *testing.T) {
	if os.Getenv("YTM_LIVE") == "" {
		t.Skip("set YTM_LIVE=1 to run live checks")
	}
	client, _ := liveClient(t)

	feed, err := client.GetSongFeed(context.Background(), 3, nil, nil)
	if err != nil {
		t.Fatalf("GetSongFeed: %v", err)
	}
	t.Logf("home feed: %d layouts, %d chips, continuation=%v",
		len(feed.Layouts), len(feed.FilterChips), feed.Continuation != "")
	if len(feed.Layouts) == 0 {
		t.Error("home feed returned no layouts")
	}
	for i, l := range feed.Layouts[:min(4, len(feed.Layouts))] {
		t.Logf("  [%d] %q (%d items)", i, l.Title, len(l.Items))
	}
	if len(feed.FilterChips) > 0 {
		chips := feed.FilterChips[:min(4, len(feed.FilterChips))]
		for _, ch := range chips {
			t.Logf("  chip %q params=%d chars", ch.Text, len(ch.Params))
		}
	}
}

// TestLibraryRoundTripLive exercises the write path and restores the original
// state. It removes the song, verifies it is gone, re-adds it, and verifies it
// is back. If the song started absent, it adds then removes instead.
//
// This is a real mutation of the account library, which is why it only runs
// when YTM_LIVE=1 is set explicitly.
func TestLibraryRoundTripLive(t *testing.T) {
	if os.Getenv("YTM_LIVE") == "" {
		t.Skip("set YTM_LIVE=1 to run live checks")
	}
	songID := os.Getenv("YTM_SONG")
	if songID == "" {
		songID = "gnkOESS2qs8"
	}
	client, _ := liveClient(t)
	ctx := context.Background()

	startSaved, err := client.songInLibrary(ctx, songID)
	if err != nil {
		t.Fatalf("read initial state: %v", err)
	}
	t.Logf("initial state: %s saved=%v", songID, startSaved)

	// Flip to the opposite state and confirm.
	if err := client.SetSongInLibrary(ctx, songID, !startSaved); err != nil {
		t.Fatalf("flip to %v: %v", !startSaved, err)
	}
	afterFlip, err := client.songInLibrary(ctx, songID)
	if err != nil {
		t.Fatalf("read flipped state: %v", err)
	}
	if afterFlip != !startSaved {
		t.Fatalf("flip failed: want saved=%v, got saved=%v", !startSaved, afterFlip)
	}
	t.Logf("flipped to saved=%v", afterFlip)

	// Restore the original state.
	if err := client.SetSongInLibrary(ctx, songID, startSaved); err != nil {
		t.Fatalf("restore to %v: %v", startSaved, err)
	}
	afterRestore, err := client.songInLibrary(ctx, songID)
	if err != nil {
		t.Fatalf("read restored state: %v", err)
	}
	if afterRestore != startSaved {
		t.Fatalf("restore failed: want saved=%v, got saved=%v", startSaved, afterRestore)
	}
	t.Logf("restored to saved=%v", afterRestore)

	// Idempotence: setting the state it already has must not error.
	if err := client.SetSongInLibrary(ctx, songID, startSaved); err != nil {
		t.Fatalf("idempotent set: %v", err)
	}
}

// songInLibrary reports whether a track is currently saved, by reading the
// library toggle from /next. It is a test helper rather than public API because
// SetSongInLibrary already makes the common case a single call.
func (c *Client) songInLibrary(ctx context.Context, songID string) (bool, error) {
	_, _, isToggled, err := c.libraryToggleTokens(ctx, CleanSongID(songID))
	return isToggled, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
