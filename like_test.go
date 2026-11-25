package ytm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestSetPlaylistSavedRequestShape(t *testing.T) {
	var paths []string
	var targets []string

	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		body := decodeBody(t, r)
		target, _ := body["target"].(map[string]any)
		pid, _ := target["playlistId"].(string)
		targets = append(targets, pid)
		io.WriteString(w, `{"status":"STATUS_SUCCEEDED"}`)
	})
	defer server.Close()
	client.writeMinGap = 0

	ctx := context.Background()
	if err := client.SetPlaylistSaved(ctx, "VLPLabc123", true); err != nil {
		t.Fatalf("save VL playlist: %v", err)
	}
	if err := client.SetPlaylistSaved(ctx, "MPSPOLAK5uy_xyz", false); err != nil {
		t.Fatalf("unsave MPSP playlist: %v", err)
	}

	if len(paths) != 2 || paths[0] != "/like/like" || paths[1] != "/like/removelike" {
		t.Fatalf("paths: got %v, want [/like/like /like/removelike]", paths)
	}
	if targets[0] != "PLabc123" || targets[1] != "OLAK5uy_xyz" {
		t.Fatalf("playlistIds: got %v, want [PLabc123 OLAK5uy_xyz]", targets)
	}
}

func TestSetPlaylistSavedValidationAndStatus(t *testing.T) {
	anon := NewClient()
	if err := anon.SetPlaylistSaved(context.Background(), "PLabc", true); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("unauthed: got %v, want ErrLoginRequired", err)
	}

	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":"STATUS_FAILED"}`)
	})
	defer server.Close()
	client.writeMinGap = 0

	if err := client.SetPlaylistSaved(context.Background(), "VL", true); err == nil {
		t.Fatal("expected error for empty cleaned playlist id")
	}
	if err := client.SetPlaylistSaved(context.Background(), "PLabc", true); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("rejected save: got %v, want ErrEditRejected", err)
	}
	if err := client.SetSongLiked(context.Background(), "vid1", SongLikedStatusLiked); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("rejected like: got %v, want ErrEditRejected", err)
	}
}
