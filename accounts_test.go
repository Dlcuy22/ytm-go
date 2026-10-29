// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//
//	Verify the account surface and the signed-in detection that guards it.
//
// Key Components:
//   - TestGetAccountsReadsChannelTokens: channel list parsing from a real capture
//   - TestGetAccountInfoReadsEmail: the email only lives in the account header
//   - TestClassifyAuthState: the three body verdicts
//   - TestLibraryReaderReturnsSessionExpired: a dead session is not an empty list
//   - TestAuthUserHeader: the session value reaches the request header
//   - TestSetChannel: the three channel fields move together
//   - TestParsePlaylistResponseLogsPanic: a broken shape is visible, not silent
//
// Dependencies:
//   - bytes
//   - context
//   - encoding/json
//   - log/slog
//   - net/http
//   - os
//   - strings
//   - testing
//
// Error Types:
//   - None
package ytm

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
)

// serveFixture answers every request with a testdata file.
func serveFixture(t *testing.T, name string) http.HandlerFunc {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}
}

func TestGetAccountsReadsChannelTokens(t *testing.T) {
	client, server := authedClient(t, serveFixture(t, "accounts_list.json"))
	defer server.Close()

	accounts, err := client.GetAccounts(context.Background())
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}

	got := accounts[0]
	if got.Name != "Test User" {
		t.Errorf("Name = %q, want %q", got.Name, "Test User")
	}
	if got.ChannelHandle != "@TestUser" {
		t.Errorf("ChannelHandle = %q, want %q", got.ChannelHandle, "@TestUser")
	}
	if !got.IsSelected {
		t.Error("IsSelected = false, want true for the active channel")
	}
	// The listing reports the cache key without its "UC" prefix.
	if got.PageID != "UCabcdefghijklmnopqrstuv" {
		t.Errorf("PageID = %q, want the UC-prefixed channel ID", got.PageID)
	}
	// authUser is only in the sign-in URL, which is the one place it appears.
	if got.AuthUser != "3" {
		t.Errorf("AuthUser = %q, want %q", got.AuthUser, "3")
	}
	if got.DataSyncID != "111111111111111111||" {
		t.Errorf("DataSyncID = %q, want the datasync token", got.DataSyncID)
	}
	if got.ThumbnailURL == "" {
		t.Error("ThumbnailURL is empty")
	}
}

func TestGetAccountInfoReadsEmail(t *testing.T) {
	client, server := authedClient(t, serveFixture(t, "accounts_list.json"))
	defer server.Close()

	info, err := client.GetAccountInfo(context.Background())
	if err != nil {
		t.Fatalf("GetAccountInfo: %v", err)
	}
	if info.Name != "Test User" {
		t.Errorf("Name = %q", info.Name)
	}
	// The email is not in the channel item; it only appears in the Google
	// account header, so a parser that reads only the items gets it wrong.
	if info.Email != "test@example.invalid" {
		t.Errorf("Email = %q, want %q", info.Email, "test@example.invalid")
	}
	if info.ChannelHandle != "@TestUser" {
		t.Errorf("ChannelHandle = %q", info.ChannelHandle)
	}
}

func TestClassifyAuthState(t *testing.T) {
	cases := []struct {
		name string
		body string
		want authState
	}{
		{
			name: "signed in",
			body: `{"responseContext":{"serviceTrackingParams":[{"service":"GFEEDBACK","params":[{"key":"logged_in","value":"1"}]}]}}`,
			want: authStateSignedIn,
		},
		{
			name: "signed out",
			body: `{"responseContext":{"serviceTrackingParams":[{"service":"GFEEDBACK","params":[{"key":"logged_in","value":"0"}]}]}}`,
			want: authStateSignedOut,
		},
		{
			// Whitespace between the two JSON fields must not defeat the match.
			name: "signed in with pretty printing",
			body: "{\n \"params\": [\n  {\n   \"key\": \"logged_in\",\n   \"value\": \"1\"\n  }\n ]\n}",
			want: authStateSignedIn,
		},
		{
			name: "no tracking params but a sign-in endpoint",
			body: `{"header":{"signInEndpoint":{"hack":true}}}`,
			want: authStateSignedOut,
		},
		{
			name: "no marker at all",
			body: `{"responseContext":{"visitorData":"x"}}`,
			want: authStateUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyAuthState([]byte(tc.body)); got != tc.want {
				t.Errorf("classifyAuthState = %v, want %v", got, tc.want)
			}
		})
	}
}

// signedOutBody is the smallest body that reports a dead session.
const signedOutBody = `{"responseContext":{"serviceTrackingParams":[{"service":"GFEEDBACK","params":[{"key":"logged_in","value":"0"}]}]},"contents":{}}`

func TestLibraryReaderReturnsSessionExpired(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(signedOutBody))
	})
	defer server.Close()

	ctx := context.Background()

	if _, _, err := client.GetLikedSongs(ctx); err != ErrSessionExpired {
		t.Errorf("GetLikedSongs error = %v, want ErrSessionExpired", err)
	}
	if _, err := client.GetLikedAlbums(ctx); err != ErrSessionExpired {
		t.Errorf("GetLikedAlbums error = %v, want ErrSessionExpired", err)
	}
	if _, err := client.GetLikedArtists(ctx); err != ErrSessionExpired {
		t.Errorf("GetLikedArtists error = %v, want ErrSessionExpired", err)
	}
	if _, err := client.GetAccountPlaylists(ctx); err != ErrSessionExpired {
		t.Errorf("GetAccountPlaylists error = %v, want ErrSessionExpired", err)
	}
	if _, _, err := client.GetLibraryPlaylists(ctx); err != ErrSessionExpired {
		t.Errorf("GetLibraryPlaylists error = %v, want ErrSessionExpired", err)
	}
	if _, err := client.GetAccounts(ctx); err != ErrSessionExpired {
		t.Errorf("GetAccounts error = %v, want ErrSessionExpired", err)
	}
}

func TestSignedInUnknownIsNotDead(t *testing.T) {
	// An endpoint that never echoes the viewer state must not be read as a dead
	// session, or every continuation would fail.
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"visitorData":"x"}`))
	})
	defer server.Close()

	client.ensureVisitorID(context.Background())
	if !client.SignedIn() {
		t.Error("SignedIn() = false for a body with no marker")
	}
	if client.AuthKnown() {
		t.Error("AuthKnown() = true for a body with no marker")
	}
}

func TestAuthUserHeader(t *testing.T) {
	var got string
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Goog-AuthUser")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(signedOutBody))
	})
	defer server.Close()

	// The constructor defaults to the first channel.
	client.GetAccounts(context.Background())
	if got != "0" {
		t.Errorf("default X-Goog-AuthUser = %q, want %q", got, "0")
	}

	client.auth.AuthUser = "3"
	client.GetAccounts(context.Background())
	if got != "3" {
		t.Errorf("X-Goog-AuthUser = %q, want %q", got, "3")
	}
}

func TestSetChannel(t *testing.T) {
	auth, err := NewAuthFromCookieString("SAPISID=test_sapisid")
	if err != nil {
		t.Fatalf("build auth: %v", err)
	}
	if auth.AuthUser != "0" {
		t.Errorf("constructor AuthUser = %q, want %q", auth.AuthUser, "0")
	}

	auth.SetChannel("UCbrand", "123||", "2")
	if auth.ChannelID != "UCbrand" || auth.DataSyncID != "123||" || auth.AuthUser != "2" {
		t.Errorf("SetChannel did not set all three: %+v", auth)
	}

	// An empty value means "leave this one alone", which is what a listing with a
	// missing token needs.
	auth.SetChannel("", "", "5")
	if auth.ChannelID != "UCbrand" || auth.DataSyncID != "123||" || auth.AuthUser != "5" {
		t.Errorf("SetChannel overwrote a field it was told to leave: %+v", auth)
	}
}

func TestGetLibraryPlaylists(t *testing.T) {
	client, server := authedClient(t, serveFixture(t, "library_playlists.json"))
	defer server.Close()
	client.auth.ChannelID = "test_channel"

	playlists, cont, err := client.GetLibraryPlaylists(context.Background())
	if err != nil {
		t.Fatalf("GetLibraryPlaylists: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("no playlists parsed from the fixture")
	}
	if cont != nil {
		t.Errorf("continuation = %+v, want nil: the fixture is a single page", cont)
	}

	// The page also holds a "New playlist" button, which carries no browse
	// endpoint and must be skipped rather than parsed into an empty playlist.
	for _, pl := range playlists {
		if pl.ID == "" {
			t.Errorf("parsed a playlist with an empty ID: %+v", pl)
		}
	}

	// Built-in playlists belong to the session channel.
	var sawOwned bool
	for _, pl := range playlists {
		if pl.ID == "VLLM" || pl.ID == "LM" {
			sawOwned = true
			if pl.OwnerID != "test_channel" {
				t.Errorf("owned playlist OwnerID = %q, want the session channel", pl.OwnerID)
			}
		}
	}
	if !sawOwned {
		t.Error("fixture should contain a built-in playlist")
	}
}

func TestGetLikedAlbumsEmptyState(t *testing.T) {
	// The empty page uses an itemSectionRenderer with a "No albums yet" message,
	// not a grid. It must yield nothing and no error, not a panic.
	client, server := authedClient(t, serveFixture(t, "library_albums_empty.json"))
	defer server.Close()

	albums, err := client.GetLikedAlbums(context.Background())
	if err != nil {
		t.Fatalf("GetLikedAlbums: %v", err)
	}
	if len(albums) != 0 {
		t.Errorf("got %d albums from the empty page, want 0", len(albums))
	}
}

func TestGetLikedArtistsStripsPrefix(t *testing.T) {
	client, server := authedClient(t, serveFixture(t, "library_artists.json"))
	defer server.Close()

	artists, err := client.GetLikedArtists(context.Background())
	if err != nil {
		t.Fatalf("GetLikedArtists: %v", err)
	}
	if len(artists) == 0 {
		t.Fatal("no artists parsed from the fixture")
	}
	for _, a := range artists {
		if strings.HasPrefix(a.ID, "MPLA") {
			t.Errorf("artist ID still carries the MPLA prefix: %q", a.ID)
		}
	}
}

func TestContinuationRequiresToken(t *testing.T) {
	client, server := authedClient(t, serveFixture(t, "library_playlists.json"))
	defer server.Close()

	if _, _, err := client.GetLibraryPlaylistsContinuation(context.Background(), ""); err == nil {
		t.Error("empty token should be rejected")
	}
	if _, _, err := client.GetLikedAlbumsContinuation(context.Background(), ""); err == nil {
		t.Error("empty token should be rejected")
	}
	if _, _, err := client.GetLikedArtistsContinuation(context.Background(), ""); err == nil {
		t.Error("empty token should be rejected")
	}
}

func TestParsePlaylistResponseLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// A radio parse against an empty response indexes through nil renderers. The
	// parser must return nil and record why, instead of taking the caller down.
	got := parsePlaylistResponse(logger, "MPTRx", &YoutubeiBrowseResponse{}, "en", true)
	if got != nil {
		t.Errorf("got %+v, want nil", got)
	}
	if !strings.Contains(buf.String(), "recovered parse panic") {
		t.Errorf("no panic was logged; buffer = %q", buf.String())
	}
	if !strings.Contains(buf.String(), "parsePlaylistResponse") {
		t.Errorf("log line does not name the operation; buffer = %q", buf.String())
	}
}

func TestClientSetLoggerNilIsIgnored(t *testing.T) {
	client := NewClient()
	client.SetLogger(nil)
	// Must not panic and must still resolve a usable logger.
	client.logPanic("test", "boom")
}

func TestAccountsListRequestShape(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody = decodeBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"responseContext":{"serviceTrackingParams":[{"service":"GFEEDBACK","params":[{"key":"logged_in","value":"1"}]}]}}`))
	})
	defer server.Close()

	if _, err := client.GetAccounts(context.Background()); err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/account/accounts_list") {
		t.Errorf("path = %q", gotPath)
	}
	// The endpoint answers a bare "OK" when a requestType is sent, so the body
	// must carry the context alone.
	if _, ok := gotBody["requestType"]; ok {
		t.Errorf("requestType must not be sent; body = %v", gotBody)
	}
	if _, ok := gotBody["context"]; !ok {
		t.Errorf("body has no context; body = %v", gotBody)
	}
}

func TestAccountsListRejectsMalformedBody(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"actions":[{"getMultiPageMenuAction":{"menu":{"multiPageMenuRenderer":{"sections":[{"accountSectionListRenderer":{"contents":[{"accountItemSectionRenderer":{"contents":[{"accountItem":{"accountName":{"runs":[{"text":"Only Name"}]},"isSelected":true}}]}}]}}]}}}}]}`))
	})
	defer server.Close()

	// A listing without identity tokens still yields the name, but no channel ID,
	// auth user or datasync token. Those must stay empty rather than be invented.
	accounts, err := client.GetAccounts(context.Background())
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}
	if accounts[0].Name != "Only Name" {
		t.Errorf("Name = %q", accounts[0].Name)
	}
	if accounts[0].PageID != "" || accounts[0].AuthUser != "" || accounts[0].DataSyncID != "" {
		t.Errorf("invented a token from nothing: %+v", accounts[0])
	}
}

func TestAuthUserFromSigninURL(t *testing.T) {
	cases := map[string]string{
		"/signin?action_handle_signin=true&authuser=3&next=%2F": "3",
		"/signin?authuser=0":   "0",
		"authuser=12":          "12",
		"/signin?next=%2F":     "",
		"/signin?authuser=abc": "",
	}
	for in, want := range cases {
		if got := authUserFromSigninURL(in); got != want {
			t.Errorf("authUserFromSigninURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChannelIDFromCacheKey(t *testing.T) {
	if got := channelIDFromCacheKey("abcdefghijklmnopqrstuv"); got != "UCabcdefghijklmnopqrstuv" {
		t.Errorf("got %q", got)
	}
	// Already prefixed values must not get a second prefix.
	if got := channelIDFromCacheKey("UCabcdefghijklmnopqrstuv"); got != "UCabcdefghijklmnopqrstuv" {
		t.Errorf("got %q", got)
	}
	if got := channelIDFromCacheKey(""); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestAccountTextHandlesBothShapes(t *testing.T) {
	// The same endpoint returns runs on one host and simpleText on another.
	runs := accountText{Runs: []TextRun{{Text: "A"}, {Text: "B"}}}
	if got := runs.value(); got != "AB" {
		t.Errorf("runs form = %q, want %q", got, "AB")
	}
	simple := accountText{SimpleText: "Only"}
	if got := simple.value(); got != "Only" {
		t.Errorf("simpleText form = %q, want %q", got, "Only")
	}
}

func TestLibrarySectionListToleratesTwoColumn(t *testing.T) {
	// Reading SingleColumnBrowseResultsRenderer.Tabs without a nil check panics
	// when the response only has the two-column form.
	br := &YoutubeiBrowseResponse{}
	if got := br.librarySectionList(); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
	if got := br.libraryShelves(); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
	if got := br.libraryContinuation(); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestSignedInOnResponse(t *testing.T) {
	var br YoutubeiBrowseResponse
	if err := json.Unmarshal([]byte(signedOutBody), &br); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// The response has no client-side verdict until the dispatcher sets one.
	if !br.SignedIn() {
		t.Error("SignedIn() = false before any verdict was recorded")
	}
	br.setAuthState(authStateSignedOut)
	if br.SignedIn() {
		t.Error("SignedIn() = true after a signed-out verdict")
	}
}
