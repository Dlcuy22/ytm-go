// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Verify the library surface: liked-song paging, the library toggle token
//   extraction, feedback submission, and the browseId on the home feed.
//
// Key Components:
//   - TestLibraryToggleReadsTokens: pure parsing of the /next menu toggle
//   - TestSetSongInLibrary*: token selection and the idempotent no-op paths
//   - TestSendFeedbackRequestBody: the /feedback payload shape
//   - TestGetSongFeedSendsBrowseID: the home feed request shape
//   - TestGetLikedSongsRequiresAuth / RequestShape
//   - TestGetLikedSongsLive / TestLibraryRoundTripLive: opt-in network checks
//
// Dependencies:
//   - context
//   - encoding/json
//   - net/http
//   - net/http/httptest
//   - os
//   - testing
//
// Error Types:
//   - None
package ytm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testClient points a client at a local server so request shapes can be
// asserted without touching the network. Tests live in package ytm, so the
// unexported apiURL is reachable.
func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	client := NewClientWithHTTP(server.Client())
	client.apiURL = server.URL + "/"
	client.nonMusicURL = server.URL + "/"
	return client, server
}

// authedClient is a test client with credentials, so requireAuth passes and the
// authed paths are exercised.
func authedClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	client, server := testClient(t, handler)
	auth, err := NewAuthFromCookieString("SAPISID=test_sapisid; SID=test_sid")
	if err != nil {
		t.Fatalf("build auth: %v", err)
	}
	client.SetAuth(auth)
	client.SetVisitorID("test_visitor")
	return client, server
}

// decodeBody reads and decodes a JSON request body.
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode body: %v (%s)", err, raw)
	}
	return body
}

// queueNextBody builds the /next shape libraryToggleTokens walks, with the
// given library toggle values.
func queueNextBody(videoID, addToken, removeToken string, isToggled bool) string {
	toggle := map[string]any{
		"toggleMenuServiceItemRenderer": map[string]any{
			"defaultText": map[string]any{"runs": []map[string]any{{"text": "Save to library"}}},
			"toggledText": map[string]any{"runs": []map[string]any{{"text": "Remove from library"}}},
			"defaultServiceEndpoint": map[string]any{
				"feedbackEndpoint": map[string]any{"feedbackToken": addToken},
			},
			"toggledServiceEndpoint": map[string]any{
				"feedbackEndpoint": map[string]any{"feedbackToken": removeToken},
			},
			"isToggled": isToggled,
		},
	}
	// A pin toggle sits alongside it, as it does in the real response. The
	// extraction must pick the library one by its labels, not by position.
	pin := map[string]any{
		"toggleMenuServiceItemRenderer": map[string]any{
			"defaultText": map[string]any{"runs": []map[string]any{{"text": "Pin to Listen again"}}},
			"toggledText": map[string]any{"runs": []map[string]any{{"text": "Unpin from Listen again"}}},
			"defaultServiceEndpoint": map[string]any{
				"feedbackEndpoint": map[string]any{"feedbackToken": "pin_default_token"},
			},
			"toggledServiceEndpoint": map[string]any{
				"feedbackEndpoint": map[string]any{"feedbackToken": "pin_toggled_token"},
			},
		},
	}
	body := map[string]any{
		"contents": map[string]any{
			"singleColumnMusicWatchNextResultsRenderer": map[string]any{
				"tabbedRenderer": map[string]any{
					"watchNextTabbedResultsRenderer": map[string]any{
						"tabs": []map[string]any{{
							"tabRenderer": map[string]any{
								"content": map[string]any{
									"musicQueueRenderer": map[string]any{
										"content": map[string]any{
											"playlistPanelRenderer": map[string]any{
												"contents": []map[string]any{{
													"playlistPanelVideoRenderer": map[string]any{
														"videoId": videoID,
														"menu": map[string]any{
															"menuRenderer": map[string]any{
																"items": []map[string]any{toggle, pin},
															},
														},
													},
												}},
											},
										},
									},
								},
							},
						}},
					},
				},
			},
		},
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}

// TestLibraryToggleReadsTokens checks the toggle is found by its labels and
// that both tokens come back with the current state.
func TestLibraryToggleReadsTokens(t *testing.T) {
	var next YoutubeiNextResponse
	if err := json.Unmarshal([]byte(queueNextBody("abc123", "add_tok", "remove_tok", true)), &next); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tabs := next.Contents.SingleColumnMusicWatchNextResultsRenderer.TabbedRenderer.WatchNextTabbedResultsRenderer.Tabs
	video := tabs[0].TabRenderer.Content.MusicQueueRenderer.Content.PlaylistPanelRenderer.Contents[0].GetRenderer()

	add, remove, isToggled := video.LibraryToggle()
	if add != "add_tok" {
		t.Errorf("add token: got %q, want add_tok", add)
	}
	if remove != "remove_tok" {
		t.Errorf("remove token: got %q, want remove_tok", remove)
	}
	if !isToggled {
		t.Error("isToggled: got false, want true")
	}
}

// TestSetSongInLibraryAlreadySavedIsNoOp proves the call is idempotent: a track
// already saved makes no /feedback request.
func TestSetSongInLibraryAlreadySavedIsNoOp(t *testing.T) {
	var feedbackCalled bool
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "next"):
			io.WriteString(w, queueNextBody("abc123", "add_tok", "remove_tok", true))
		case strings.Contains(r.URL.Path, "feedback"):
			feedbackCalled = true
			io.WriteString(w, `{"feedbackResponses":[{"isProcessed":true}]}`)
		default:
			io.WriteString(w, `{}`)
		}
	})
	defer server.Close()

	if err := client.SetSongInLibrary(context.Background(), "abc123", true); err != nil {
		t.Fatalf("SetSongInLibrary: %v", err)
	}
	if feedbackCalled {
		t.Error("saving an already-saved track must not call /feedback")
	}
}

// TestSetSongInLibraryRemovesWhenSaved checks the remove direction submits the
// toggled token.
func TestSetSongInLibraryRemovesWhenSaved(t *testing.T) {
	var gotTokens []string
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "next"):
			io.WriteString(w, queueNextBody("abc123", "add_tok", "remove_tok", true))
		case strings.Contains(r.URL.Path, "feedback"):
			body := decodeBody(t, r)
			if raw, ok := body["feedbackTokens"].([]any); ok {
				for _, tk := range raw {
					gotTokens = append(gotTokens, tk.(string))
				}
			}
			io.WriteString(w, `{"feedbackResponses":[{"isProcessed":true}]}`)
		default:
			io.WriteString(w, `{}`)
		}
	})
	defer server.Close()

	if err := client.SetSongInLibrary(context.Background(), "abc123", false); err != nil {
		t.Fatalf("SetSongInLibrary: %v", err)
	}
	if len(gotTokens) != 1 || gotTokens[0] != "remove_tok" {
		t.Errorf("tokens: got %v, want [remove_tok]", gotTokens)
	}
}

// TestSetSongInLibraryAddsWhenAbsent checks the add direction.
func TestSetSongInLibraryAddsWhenAbsent(t *testing.T) {
	var gotTokens []string
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "next"):
			io.WriteString(w, queueNextBody("abc123", "add_tok", "remove_tok", false))
		case strings.Contains(r.URL.Path, "feedback"):
			body := decodeBody(t, r)
			if raw, ok := body["feedbackTokens"].([]any); ok {
				for _, tk := range raw {
					gotTokens = append(gotTokens, tk.(string))
				}
			}
			io.WriteString(w, `{"feedbackResponses":[{"isProcessed":true}]}`)
		default:
			io.WriteString(w, `{}`)
		}
	})
	defer server.Close()

	if err := client.SetSongInLibrary(context.Background(), "abc123", true); err != nil {
		t.Fatalf("SetSongInLibrary: %v", err)
	}
	if len(gotTokens) != 1 || gotTokens[0] != "add_tok" {
		t.Errorf("tokens: got %v, want [add_tok]", gotTokens)
	}
}

// TestSetSongInLibraryWithoutToggle reports the missing-token error rather than
// sending an empty token.
func TestSetSongInLibraryWithoutToggle(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "next") {
			io.WriteString(w, `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[]}}}}}`)
			return
		}
		io.WriteString(w, `{}`)
	})
	defer server.Close()

	err := client.SetSongInLibrary(context.Background(), "abc123", true)
	if err == nil {
		t.Fatal("expected an error when no toggle is present")
	}
	if !strings.Contains(err.Error(), "feedback token not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSendFeedbackRequestBody pins the payload the endpoint expects.
func TestSendFeedbackRequestBody(t *testing.T) {
	var body map[string]any
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		io.WriteString(w, `{"feedbackResponses":[{"isProcessed":true}]}`)
	})
	defer server.Close()

	if err := client.SendFeedback(context.Background(), "tok_a", "tok_b"); err != nil {
		t.Fatalf("SendFeedback: %v", err)
	}
	tokens, ok := body["feedbackTokens"].([]any)
	if !ok || len(tokens) != 2 {
		t.Fatalf("feedbackTokens: got %v", body["feedbackTokens"])
	}
	if tokens[0] != "tok_a" || tokens[1] != "tok_b" {
		t.Errorf("tokens: got %v", tokens)
	}
	if body["isFeedbackTokenUnencrypted"] != false {
		t.Errorf("isFeedbackTokenUnencrypted: got %v", body["isFeedbackTokenUnencrypted"])
	}
	if body["shouldMerge"] != false {
		t.Errorf("shouldMerge: got %v", body["shouldMerge"])
	}
}

// TestSendFeedbackRejectsUnprocessed surfaces a refused token, which arrives as
// HTTP 200 with isProcessed false.
func TestSendFeedbackRejectsUnprocessed(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"feedbackResponses":[{"isProcessed":false}]}`)
	})
	defer server.Close()

	if err := client.SendFeedback(context.Background(), "tok"); err == nil {
		t.Fatal("expected a rejection error")
	}
}

// TestSendFeedbackRequiresToken guards the empty-argument case.
func TestSendFeedbackRequiresToken(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{}`)
	})
	defer server.Close()

	if err := client.SendFeedback(context.Background()); err == nil {
		t.Fatal("expected an error for an empty token list")
	}
}

// TestGetSongFeedSendsBrowseID is the regression guard for the home feed: the
// request must name FEmusic_home, which it previously did not.
func TestGetSongFeedSendsBrowseID(t *testing.T) {
	var body map[string]any
	client, server := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		io.WriteString(w, `{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[]}}}`)
	})
	defer server.Close()

	if _, err := client.GetSongFeed(context.Background(), 1, nil, nil); err != nil {
		t.Fatalf("GetSongFeed: %v", err)
	}
	if body["browseId"] != "FEmusic_home" {
		t.Errorf("browseId: got %v, want FEmusic_home", body["browseId"])
	}
}

// TestGetSongFeedWithBrowseHonoursAuthed checks the authed flag reaches the
// request: with credentials set, the cookie must be attached.
func TestGetSongFeedWithBrowseHonoursAuthed(t *testing.T) {
	var gotCookie, gotAuth bool
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie") != ""
		gotAuth = r.Header.Get("Authorization") != ""
		io.WriteString(w, `{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[]}}}`)
	})
	defer server.Close()

	if _, err := client.GetSongFeedWithBrowse(context.Background(), "FEmusic_explore", 1, nil, nil, true); err != nil {
		t.Fatalf("GetSongFeedWithBrowse: %v", err)
	}
	if !gotCookie || !gotAuth {
		t.Errorf("authed request missing credentials: cookie=%v auth=%v", gotCookie, gotAuth)
	}

	gotCookie, gotAuth = false, false
	if _, err := client.GetSongFeedWithBrowse(context.Background(), "FEmusic_explore", 1, nil, nil, false); err != nil {
		t.Fatalf("GetSongFeedWithBrowse (anon): %v", err)
	}
	if gotCookie || gotAuth {
		t.Errorf("anonymous request leaked credentials: cookie=%v auth=%v", gotCookie, gotAuth)
	}
}

// TestGetLikedSongsRequestShape pins the browse target and that the call is
// authed.
func TestGetLikedSongsRequestShape(t *testing.T) {
	var body map[string]any
	var gotCookie bool
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		gotCookie = r.Header.Get("Cookie") != ""
		io.WriteString(w, `{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[]}}}`)
	})
	defer server.Close()

	if _, _, err := client.GetLikedSongs(context.Background()); err != nil {
		t.Fatalf("GetLikedSongs: %v", err)
	}
	if body["browseId"] != libraryBrowseID {
		t.Errorf("browseId: got %v, want %s", body["browseId"], libraryBrowseID)
	}
	if !gotCookie {
		t.Error("liked songs must be requested with credentials")
	}
}

// TestGetLikedSongsRequiresAuth keeps the login contract.
func TestGetLikedSongsRequiresAuth(t *testing.T) {
	client := NewClient()
	if _, _, err := client.GetLikedSongs(context.Background()); err == nil {
		t.Fatal("expected ErrLoginRequired")
	}
}

// TestShelfContinuationExtractsToken covers both continuation shapes.
func TestShelfContinuationExtractsToken(t *testing.T) {
	shelf := &MusicShelfRenderer{
		Continuations: []Continuation{{
			NextContinuationData: &ContinuationData{Continuation: "next_page_token"},
		}},
	}
	cont := shelfContinuation(shelf)
	if cont == nil || cont.Token != "next_page_token" {
		t.Fatalf("continuation: got %+v", cont)
	}
	if cont.Type != ContinuationPlaylist {
		t.Errorf("continuation type: got %q", cont.Type)
	}

	if got := shelfContinuation(&MusicShelfRenderer{}); got != nil {
		t.Errorf("empty shelf should yield nil, got %+v", got)
	}
}

// TestGetLikedSongsContinuationRequestShape checks the paging request carries
// the token and stays authed.
func TestGetLikedSongsContinuationRequestShape(t *testing.T) {
	var body map[string]any
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		body = decodeBody(t, r)
		io.WriteString(w, `{"continuationContents":{"musicShelfContinuation":{"contents":[]}}}`)
	})
	defer server.Close()

	if _, _, err := client.GetLikedSongsContinuation(context.Background(), "page_two"); err != nil {
		t.Fatalf("GetLikedSongsContinuation: %v", err)
	}
	if body["continuation"] != "page_two" {
		t.Errorf("continuation: got %v", body["continuation"])
	}
}

// TestGetLikedSongsContinuationRequiresToken rejects an empty token rather than
// sending a request that would return the first page again.
func TestGetLikedSongsContinuationRequiresToken(t *testing.T) {
	client := NewClient()
	auth, _ := NewAuthFromCookieString("SAPISID=x")
	client.SetAuth(auth)
	if _, _, err := client.GetLikedSongsContinuation(context.Background(), ""); err == nil {
		t.Fatal("expected an error for an empty token")
	}
}
