// Command e2e prints the resolved stream for one track, then downloads it.
//
// It is the manual proof that a resolved URL is playable end to end: the
// written file should decode as Opus and its byte count should match the
// content length the API reported.
//
//	go run ./cmd/e2e -id gnkOESS2qs8 -out /tmp/track.webm
//
// With -video it resolves the picture as well and writes both files, because an
// adaptive video rendition carries no audio.
//
//	go run ./cmd/e2e -id BFSmN6fQ_04 -video -height 720 -out /tmp/track.webm
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dlcuy22/ytm-go"
)

func main() {
	songID := flag.String("id", "gnkOESS2qs8", "YouTube track ID")
	out := flag.String("out", "/tmp/track.webm", "output file for the audio track")
	urlOnly := flag.Bool("url", false, "print the URLs and exit without downloading")
	withVideo := flag.Bool("video", false, "also resolve and download the video rendition")
	height := flag.Int("height", 720, "cap the video resolution in pixels")
	codec := flag.String("codec", "", "prefer a video family: avc1, vp9 or av01")
	flag.Parse()

	client := ytm.NewClient()
	streams, err := client.GetStreamWithOptions(context.Background(), *songID,
		ytm.StreamOptions{Video: *withVideo, MaxHeight: *height, VideoCodec: ytm.VideoCodec(*codec)})
	if err != nil {
		log.Fatalf("GetStream: %v", err)
	}
	best, ok := streams.Best()
	if !ok {
		log.Fatal("no playable audio format")
	}

	fmt.Printf("track:   %s - %s (%dms)\n", streams.Author, streams.Title, streams.DurationMs)
	fmt.Printf("audio:   itag=%d codec=%s bitrate=%d sr=%d ch=%d len=%d\n",
		best.Itag, best.Codec, best.Bitrate, best.SampleRate, best.Channels, best.ContentLength)
	fmt.Printf("expires: %s\n", best.ExpiresAt.Format(time.RFC3339))

	var video ytm.VideoFormat
	if *withVideo {
		video, ok = streams.BestVideo()
		if !ok {
			log.Fatalf("no video rendition at or below %dpx", *height)
		}
		fmt.Printf("video:   itag=%d codec=%s %dx%d %dfps %s bitrate=%d len=%d muxed=%v\n",
			video.Itag, video.Codec, video.Width, video.Height, video.Fps,
			video.QualityLabel, video.Bitrate, video.ContentLength, video.Muxed)
	}

	if *urlOnly {
		fmt.Println(best.URL)
		if *withVideo {
			fmt.Println(video.URL)
		}
		return
	}

	if err := download(best.URL, *out, best.ContentLength); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("output:  %s\n", *out)

	if *withVideo {
		videoOut := strings.TrimSuffix(*out, filepath.Ext(*out)) + ".video" + extOf(video.MimeType)
		if err := download(video.URL, videoOut, video.ContentLength); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("output:  %s\n", videoOut)
		if _, needsAudio := streams.AudioFor(video); needsAudio {
			fmt.Printf("play:    mpv --audio-file=%s %s\n", *out, videoOut)
		} else {
			fmt.Printf("play:    mpv %s\n", videoOut)
		}
	}
}

// chunkSize is how much of a signed URL is fetched per request when saving to
// disk. It is 4 MiB because googlevideo throttles a single long transfer once
// it has served a few megabytes: measured on one 17.9 MB rendition, one
// `bytes=0-` request ran at 164 KiB/s (109 s) while 4 MiB requests ran at
// 2.9 MiB/s (6.2 s).
//
// This applies to saving a file. Playback does not need it: the server answers
// byte ranges, so a demuxer streams and seeks the URL directly.
const chunkSize = 4 << 20

// download writes one signed URL to disk and checks it against the length the
// API reported.
func download(url, path string, want int64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	started := time.Now()
	n, err := fetchChunked(url, file, want)
	if err != nil {
		return err
	}
	elapsed := time.Since(started)
	fmt.Printf("wrote %d bytes in %s (%.0f KiB/s)\n", n, elapsed.Round(time.Millisecond),
		float64(n)/elapsed.Seconds()/1024)
	if want > 0 && n != want {
		return fmt.Errorf("byte count %d does not match reported content length %d", n, want)
	}
	return nil
}

// fetchChunked copies the media into w a range at a time.
//
// The Range header is not optional. googlevideo throttles an un-ranged GET to
// roughly 32 KiB/s, and a single ranged request to a slow crawl after a few
// megabytes, so the transfer is split instead of streamed in one go.
func fetchChunked(url string, w io.Writer, total int64) (int64, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	var written int64

	for {
		offset := written
		end := offset + chunkSize - 1
		if total > 0 && end >= total {
			end = total - 1
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return written, err
		}
		// The signed URL is bound to the requesting client's user agent.
		req.Header.Set("User-Agent", ytm.GetContextVisionOS("en").UserAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, end))

		resp, err := client.Do(req)
		if err != nil {
			return written, fmt.Errorf("fetch at %d: %w", offset, err)
		}

		if resp.StatusCode/100 != 2 {
			resp.Body.Close()
			return written, fmt.Errorf("fetch at %d: HTTP %d", offset, resp.StatusCode)
		}

		// A server that ignores Range answers 200 with the whole file, which is
		// correct only for the first chunk; anything else would duplicate bytes.
		if resp.StatusCode == http.StatusOK && offset > 0 {
			resp.Body.Close()
			return written, fmt.Errorf("fetch at %d: server ignored the range", offset)
		}

		n, copyErr := io.Copy(w, resp.Body)
		resp.Body.Close()
		written += n
		if copyErr != nil {
			return written, fmt.Errorf("copy at %d: %w", offset, copyErr)
		}
		if n == 0 {
			break
		}
		if total > 0 && written >= total {
			break
		}
	}
	return written, nil
}

// extOf picks a file extension from a MIME type, so the written file opens in a
// player without being told the container.
func extOf(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "webm"):
		return ".webm"
	case strings.Contains(mimeType, "mp4"):
		return ".mp4"
	default:
		return ".bin"
	}
}
