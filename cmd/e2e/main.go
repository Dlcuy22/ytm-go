// Command e2e prints the resolved stream for one track, then downloads it.
//
// It is the manual proof that a resolved URL is playable end to end: the
// written file should decode as Opus and its byte count should match the
// content length the API reported.
//
//	go run ./cmd/e2e -id gnkOESS2qs8 -out /tmp/track.webm
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dlcuy22/ytm-go"
)

func main() {
	songID := flag.String("id", "gnkOESS2qs8", "YouTube track ID")
	out := flag.String("out", "/tmp/track.webm", "output file")
	urlOnly := flag.Bool("url", false, "print the URL and exit without downloading")
	flag.Parse()

	client := ytm.NewClient()
	streams, err := client.GetStream(context.Background(), *songID)
	if err != nil {
		log.Fatalf("GetStream: %v", err)
	}
	best, ok := streams.Best()
	if !ok {
		log.Fatal("no playable format")
	}

	fmt.Printf("track:   %s - %s (%dms)\n", streams.Author, streams.Title, streams.DurationMs)
	fmt.Printf("format:  itag=%d codec=%s bitrate=%d sr=%d ch=%d len=%d\n",
		best.Itag, best.Codec, best.Bitrate, best.SampleRate, best.Channels, best.ContentLength)
	fmt.Printf("expires: %s\n", best.ExpiresAt.Format(time.RFC3339))

	if *urlOnly {
		fmt.Println(best.URL)
		return
	}

	req, err := http.NewRequest(http.MethodGet, best.URL, nil)
	if err != nil {
		log.Fatal(err)
	}
	// The signed URL is bound to the requesting client's user agent.
	req.Header.Set("User-Agent", ytm.GetContextVisionOS("en").UserAgent)
	req.Header.Set("Accept", "*/*")
	// The Range header is not optional. googlevideo throttles an un-ranged GET
	// to roughly 32 KiB/s, while the same request with `bytes=0-` runs at full
	// speed: measured 112 s versus 1.4 s for one 3.6 MB track. Any downloader
	// built on this API must send it.
	req.Header.Set("Range", "bytes=0-")

	started := time.Now()
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		log.Fatalf("fetch: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	n, err := io.Copy(file, resp.Body)
	if err != nil {
		log.Fatalf("copy after %d bytes: %v", n, err)
	}
	elapsed := time.Since(started)
	fmt.Printf("wrote %d bytes in %s (%.0f KiB/s)\n", n, elapsed.Round(time.Millisecond),
		float64(n)/elapsed.Seconds()/1024)
	if best.ContentLength > 0 && n != best.ContentLength {
		log.Fatalf("byte count %d does not match reported content length %d", n, best.ContentLength)
	}
	fmt.Printf("output: %s\n", *out)
}
