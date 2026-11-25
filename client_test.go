package ytm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 11, 26, 6, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(_ context.Context, d time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	return nil
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func (c *fakeClock) Sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.sleeps...)
}

func TestWritePacingAndReadBypass(t *testing.T) {
	clk := newFakeClock()
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":"STATUS_SUCCEEDED"}`)
	})
	defer server.Close()
	client.nowFn = clk.Now
	client.sleepFn = clk.Sleep

	ctx := context.Background()
	var resp writeResponse

	// First write has no prior write, so no gap wait.
	if err := client.doInnerTube(ctx, "like/like", GetContextWebRemix(DefaultHL), nil, true, &resp); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	// Read path (including playlist/get_add_to_playlist) never sleeps for write pacing.
	if err := client.doInnerTube(ctx, "playlist/get_add_to_playlist", GetContextWebRemix(DefaultHL), nil, true, &resp); err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(clk.Sleeps()) != 0 {
		t.Fatalf("expected 0 sleeps after read, got %v", clk.Sleeps())
	}

	// Second write 100ms later waits the remaining 400ms.
	clk.Advance(100 * time.Millisecond)
	if err := client.doInnerTube(ctx, "browse/edit_playlist", GetContextWebRemix(DefaultHL), nil, true, &resp); err != nil {
		t.Fatalf("write 2: %v", err)
	}
	sleeps := clk.Sleeps()
	if len(sleeps) != 1 || sleeps[0] != 400*time.Millisecond {
		t.Fatalf("write 2 sleep: got %v, want [400ms]", sleeps)
	}

	// Write after full gap does not sleep again.
	clk.Advance(600 * time.Millisecond)
	if err := client.doInnerTube(ctx, "playlist/delete", GetContextWebRemix(DefaultHL), nil, true, &resp); err != nil {
		t.Fatalf("write 3: %v", err)
	}
	if len(clk.Sleeps()) != 1 {
		t.Fatalf("write 3 unexpected sleeps: %v", clk.Sleeps())
	}
}

func TestRefusalSingle403RetriesOnce(t *testing.T) {
	clk := newFakeClock()
	var calls atomic.Int32
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"error":{"code":403}}`)
			return
		}
		io.WriteString(w, `{"status":"STATUS_SUCCEEDED"}`)
	})
	defer server.Close()
	client.nowFn = clk.Now
	client.sleepFn = clk.Sleep
	client.writeMinGap = 0

	var resp writeResponse
	if err := client.doInnerTube(context.Background(), "browse", GetContextWebRemix(DefaultHL), nil, true, &resp); err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", calls.Load())
	}
	if sleeps := clk.Sleeps(); len(sleeps) != 1 || sleeps[0] != refusalRetryDelay {
		t.Fatalf("expected 250ms retry sleep, got %v", sleeps)
	}
	if client.refusals != 0 {
		t.Fatalf("expected refusals reset to 0, got %d", client.refusals)
	}
}

func TestRefusalBackoffAndReset(t *testing.T) {
	clk := newFakeClock()
	var failMode atomic.Bool
	failMode.Store(true)

	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		if failMode.Load() {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"error":{"code":403}}`)
			return
		}
		io.WriteString(w, `{"status":"STATUS_SUCCEEDED"}`)
	})
	defer server.Close()
	client.nowFn = clk.Now
	client.sleepFn = clk.Sleep
	client.writeMinGap = 0

	ctx := context.Background()
	var resp writeResponse

	// Each call does 2 attempts (403 -> sleep 250ms -> 403).
	// Call 1: refusals 1, 2
	err := client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp)
	if err == nil || errors.Is(err, ErrRateLimited) {
		t.Fatalf("call 1: got %v, want plain InnerTubeError", err)
	}
	// Call 2: refusals 3, 4
	err = client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp)
	if err == nil || errors.Is(err, ErrRateLimited) {
		t.Fatalf("call 2: got %v, want plain InnerTubeError", err)
	}
	// Call 3: refusals 5, 6 -> ErrRateLimited and pausedUntil = +5s
	err = client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("call 3: got %v, want ErrRateLimited", err)
	}

	// Call 4 must wait 5s before attempting, then its two 403s (7, 8) extend pause to 20s.
	before := len(clk.Sleeps())
	err = client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("call 4: got %v, want ErrRateLimited", err)
	}
	call4Sleeps := clk.Sleeps()[before:]
	if len(call4Sleeps) != 2 || call4Sleeps[0] != 5*time.Second || call4Sleeps[1] != refusalRetryDelay {
		t.Fatalf("call 4 sleeps: got %v, want [5s 250ms]", call4Sleeps)
	}

	// Call 5 hits pause from refusal 8 (20s), then 9 -> 40s, 10 -> 40s (capped).
	before = len(clk.Sleeps())
	_ = client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp)
	call5Sleeps := clk.Sleeps()[before:]
	if len(call5Sleeps) < 1 || call5Sleeps[0] != 20*time.Second {
		t.Fatalf("call 5 first sleep: got %v, want 20s", call5Sleeps)
	}

	// Call 6 hits cap of 40s, then succeeds once failMode turns false, resetting refusals.
	failMode.Store(false)
	before = len(clk.Sleeps())
	if err := client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp); err != nil {
		t.Fatalf("call 6: %v", err)
	}
	call6Sleeps := clk.Sleeps()[before:]
	if len(call6Sleeps) != 1 || call6Sleeps[0] != 40*time.Second {
		t.Fatalf("call 6 sleeps: got %v, want [40s]", call6Sleeps)
	}
	if client.refusals != 0 || !client.pausedUntil.IsZero() {
		t.Fatalf("expected reset after success, got refusals=%d pausedUntil=%v", client.refusals, client.pausedUntil)
	}
}

func TestSleepHonorsContextCancel(t *testing.T) {
	clk := newFakeClock()
	client, server := authedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer server.Close()
	client.nowFn = clk.Now
	// Leave sleepFn nil so real timer cancellation runs.
	client.pausedUntil = clk.Now().Add(30 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var resp writeResponse
	err := client.doInnerTube(ctx, "browse", GetContextWebRemix(DefaultHL), nil, true, &resp)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestWriteResponseStatusAndDedupe(t *testing.T) {
	if err := (writeResponse{Status: "STATUS_SUCCEEDED"}).checkStatus(); err != nil {
		t.Fatalf("unexpected error for SUCCEEDED: %v", err)
	}
	if err := (writeResponse{}).checkStatus(); err != nil {
		t.Fatalf("unexpected error for empty status: %v", err)
	}
	if err := (writeResponse{Status: "STATUS_FAILED"}).checkStatus(); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("got %v, want ErrEditRejected", err)
	}
	dupResp := writeResponse{
		Status:  "STATUS_FAILED",
		Actions: []json.RawMessage{json.RawMessage(`{"confirmDialog":{"dedupeOption":"DEDUPE_OPTION_SKIP"}}`)},
	}
	if !dupResp.asksAboutDuplicates() {
		t.Fatal("expected asksAboutDuplicates to return true")
	}
}
