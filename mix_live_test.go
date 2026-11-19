// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Opt-in live check for mix loading against the real watch-next endpoint.
//
// Key Components:
//   - TestGetMixLive: the mix behind a real YouTube share id, plus one page
//
// Dependencies:
//   - context
//   - os
//   - testing
//
// Error Types:
//   - None
package ytm

import (
	"context"
	"os"
	"testing"
)

// TestGetMixLive resolves the mix a YouTube share URL names. It is skipped
// unless YTM_LIVE=1, because it needs the network and the mix itself rotates.
func TestGetMixLive(t *testing.T) {
	if os.Getenv("YTM_LIVE") == "" {
		t.Skip("set YTM_LIVE=1 to run live checks")
	}

	client := NewClient()
	ctx := context.Background()

	playlist, err := client.GetMix(ctx, "RDLoK17z6xDwI", nil)
	if err != nil {
		t.Fatalf("GetMix: %v", err)
	}
	if len(playlist.Items) == 0 {
		t.Fatal("mix has no items")
	}
	if first := playlist.Items[0]; first.ID != "LoK17z6xDwI" || first.Name == "" {
		t.Fatalf("first = %+v, want the seed track", first)
	}
	if playlist.Continuation == nil {
		t.Fatal("mix has no continuation, want a radio token")
	}

	next, err := client.GetMix(ctx, "RDLoK17z6xDwI", playlist.Continuation)
	if err != nil {
		t.Fatalf("GetMix continuation: %v", err)
	}
	if len(next.Items) == 0 {
		t.Fatal("continuation page has no items")
	}
}
