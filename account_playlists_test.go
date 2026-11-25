package ytm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestCreatePlaylistOptionsAndSanitization(t *testing.T) {
	var gotBodies []map[string]any
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/playlist/create" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotBodies = append(gotBodies, decodeBody(t, r))
		io.WriteString(w, `{"status":"STATUS_SUCCEEDED","playlistId":"PLnew123"}`)
	})
	defer server.Close()
	client.writeMinGap = 0

	ctx := context.Background()
	id, err := client.CreatePlaylist(ctx, CreatePlaylistOptions{
		Title:    "  <My> Playlist  ",
		VideoIDs: []string{"vid1", "vid2"},
	})
	if err != nil {
		t.Fatalf("CreatePlaylist default: %v", err)
	}
	if id != "PLnew123" {
		t.Fatalf("playlistId: got %q, want PLnew123", id)
	}
	if gotBodies[0]["title"] != "My Playlist" {
		t.Errorf("sanitized title: got %v, want %q", gotBodies[0]["title"], "My Playlist")
	}
	if gotBodies[0]["privacyStatus"] != "PRIVATE" {
		t.Errorf("default privacy: got %v, want PRIVATE", gotBodies[0]["privacyStatus"])
	}
	if vids, _ := gotBodies[0]["videoIds"].([]any); len(vids) != 2 || vids[0] != "vid1" || vids[1] != "vid2" {
		t.Errorf("videoIds: got %v", gotBodies[0]["videoIds"])
	}

	if _, err := client.CreatePlaylist(ctx, CreatePlaylistOptions{
		Title:   "Public List",
		Privacy: PlaylistPublic,
	}); err != nil {
		t.Fatalf("CreatePlaylist public: %v", err)
	}
	if gotBodies[1]["privacyStatus"] != "PUBLIC" {
		t.Errorf("explicit privacy: got %v, want PUBLIC", gotBodies[1]["privacyStatus"])
	}

	if _, err := client.CreatePlaylist(ctx, CreatePlaylistOptions{Title: "<<<>>>"}); err == nil {
		t.Fatal("expected error for empty title after sanitization")
	}
}

func TestCreatePlaylistRejectedStatus(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":"STATUS_FAILED"}`)
	})
	defer server.Close()
	client.writeMinGap = 0

	if _, err := client.CreatePlaylist(context.Background(), CreatePlaylistOptions{Title: "Test"}); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("got %v, want ErrEditRejected", err)
	}
}

func TestAddSongsToPlaylistOptsDedupeModes(t *testing.T) {
	var seenOptions []string
	var modeToReturn string

	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := decodeBody(t, r)
		actions, _ := body["actions"].([]any)
		act, _ := actions[0].(map[string]any)
		seenOptions = append(seenOptions, act["dedupeOption"].(string))

		switch modeToReturn {
		case "dup":
			io.WriteString(w, `{"status":"STATUS_FAILED","actions":[{"confirmDialogRenderer":{"confirmButton":{"serviceEndpoint":{"playlistEditEndpoint":{"actions":[{"dedupeOption":"DEDUPE_OPTION_SKIP"}]}}}}}]}`)
		case "fail":
			io.WriteString(w, `{"status":"STATUS_FAILED"}`)
		default:
			io.WriteString(w, `{"status":"STATUS_SUCCEEDED"}`)
		}
	})
	defer server.Close()
	client.writeMinGap = 0

	ctx := context.Background()
	if err := client.AddSongsToPlaylistOpts(ctx, "VLPL1", []string{"v1"}, DedupeAsk); err != nil {
		t.Fatalf("DedupeAsk ok: %v", err)
	}
	if err := client.AddSongsToPlaylistOpts(ctx, "PL1", []string{"v1"}, DedupeDrop); err != nil {
		t.Fatalf("DedupeDrop ok: %v", err)
	}
	if err := client.AddSongsToPlaylist(ctx, "PL1", []string{"v1"}); err != nil {
		t.Fatalf("AddSongsToPlaylist ok: %v", err)
	}

	wantOpts := []string{"DEDUPE_OPTION_CHECK", "DEDUPE_OPTION_DROP_DUPLICATE", "DEDUPE_OPTION_SKIP"}
	for i, w := range wantOpts {
		if seenOptions[i] != w {
			t.Errorf("mode[%d]: got %q, want %q", i, seenOptions[i], w)
		}
	}

	modeToReturn = "dup"
	if err := client.AddSongsToPlaylistOpts(ctx, "PL1", []string{"v1"}, DedupeAsk); !errors.Is(err, ErrDuplicates) {
		t.Fatalf("expected ErrDuplicates, got %v", err)
	}

	modeToReturn = "fail"
	if err := client.AddSongsToPlaylistOpts(ctx, "PL1", []string{"v1"}, DedupeDrop); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("expected ErrEditRejected, got %v", err)
	}
}

func TestEditAndDeleteRejectFailedStatus(t *testing.T) {
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":"STATUS_FAILED"}`)
	})
	defer server.Close()
	client.writeMinGap = 0

	ctx := context.Background()
	if err := client.EditPlaylist(ctx, "PL1", []PlaylistEditAction{{Action: PlaylistEditActionSetName, PlaylistName: "X"}}); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("EditPlaylist: got %v, want ErrEditRejected", err)
	}
	if err := client.DeleteAccountPlaylist(ctx, "PL1"); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("DeleteAccountPlaylist: got %v, want ErrEditRejected", err)
	}
}

func TestOwnershipHelpersAndParsePlaylistItems(t *testing.T) {
	if !(Playlist{OwnerID: "UC123"}).Owned("UC123") {
		t.Error("expected matching channel ID to be owned")
	}
	if (Playlist{OwnerID: "UC123"}).Owned("") || (Playlist{}).Owned("UC123") {
		t.Error("expected empty channel/owner ID not to be owned")
	}

	editable := map[string]bool{
		"PLfJnkCDFqhzg":       true,
		"VLPLfJnkCDFqhzg":     true,
		"LM":                  false,
		"VLLM":                false,
		"OLAK5uy_abc":         false,
		"RDCLAK5uy_abc":       false,
		"MPREb_abc":           false,
		"SE":                  false,
		"FEmusic_liked":       false,
		"MPSPabc":             false,
		"LPabc":               false,
		"LM:123":              false,
		"liked":               false,
		"downloaded":          false,
		"":                    false,
	}
	for id, want := range editable {
		if got := IsEditablePlaylistID(id); got != want {
			t.Errorf("IsEditablePlaylistID(%q) = %v, want %v", id, got, want)
		}
	}

	raw := `[
		{
			"musicTwoRowItemRenderer": {
				"title": {"runs": [{"text": "My Playlist"}]},
				"navigationEndpoint": {"browseEndpoint": {
					"browseId": "VLPL_OWNED",
					"browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_PLAYLIST"}}
				}},
				"menu": {"menuRenderer": {"items": [
					{"menuNavigationItemRenderer": {"icon": {"iconType": "DELETE"}}}
				]}}
			}
		},
		{
			"musicTwoRowItemRenderer": {
				"title": {"runs": [{"text": "Saved Playlist"}]},
				"navigationEndpoint": {"browseEndpoint": {
					"browseId": "VLPL_FOREIGN",
					"browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_PLAYLIST"}}
				}},
				"menu": {"menuRenderer": {"items": [
					{"menuServiceItemRenderer": {"icon": {"iconType": "REMOVE_FROM_LIBRARY"}}}
				]}}
			}
		}
	]`
	var items []YoutubeiShelfContentsItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("unmarshal items: %v", err)
	}
	auth := &AuthState{ChannelID: "UC_ME"}
	pls := parsePlaylistItems(items, DefaultHL, auth)
	if len(pls) != 2 {
		t.Fatalf("got %d playlists, want 2", len(pls))
	}
	if !pls[0].Owned("UC_ME") {
		t.Errorf("owned playlist had OwnerID=%q, want UC_ME", pls[0].OwnerID)
	}
	if pls[1].Owned("UC_ME") {
		t.Errorf("foreign playlist unexpectedly marked owned: %+v", pls[1])
	}
}
