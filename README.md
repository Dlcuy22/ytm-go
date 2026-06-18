# ytm-go

*Read this in other languages: [Bahasa Indonesia](README.id.md).*

`ytm-go` is a Go port of [ytm-kt](https://gitlab.com/syk.sh/ytm-kt). It provides search functions and metadata extraction from YouTube Music.

## Key Features

- **Search & Autocomplete**: Query songs, albums, playlists, artists, and fetch real-time autocomplete search suggestions.
- **Full Metadata Extraction**: Retrieve tracks' detailed info (title, artists, duration, thumbnail provider, explicit tags).
- **Artist Profiles**: Load artist details, channel statistics, and categorized visual shelves (singles, popular tracks, albums).
- **Playlists & Albums**: Read the complete tracklists and metadata for public playlists or official albums.
- **Lyrics Sheet Loading**: Fetch plaintext lyrics using a song's lyrics browse ID.
- **Radio Recommendations**: Build recommended radio mix tracklists dynamically.

---

## Getting Started

Unlike the original `ytm-kt` library which leverages `NewPipeExtractor` to resolve/download media formats, `ytm-go` delegates downloading entirely to **`yt-dlp`** via the local Go bindings **[go-ytdlp](https://github.com/lrstanley/go-ytdlp)**. 

This decouples the library's catalog parsing from the decryption/download routing (which frequently breaks due to YouTube's player signature revisions).

### Simple Usage Example

Here is a quick example of searching for a track and downloading it directly to Ogg/Opus format:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dlcuy22/ytm-go"
	"github.com/lrstanley/go-ytdlp"
)

func main() {
	ctx := context.Background()

	// 1. Initialize the client
	client := ytm.NewClient()

	// 2. Search for the song
	fmt.Println("Searching for 'MURI MURI EVOLUTION'...")
	results, err := client.Search(ctx, "MURI MURI EVOLUTION - NANAOAKARI", "", false)
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	var targetSong *ytm.Song
	for _, cat := range results.Categories {
		for _, item := range cat.Layout.Items {
			if s, ok := item.(*ytm.Song); ok {
				targetSong = s
				break
			}
		}
		if targetSong != nil {
			break
		}
	}

	if targetSong == nil {
		log.Fatalf("Song not found in search results.")
	}
	fmt.Printf("Found Song: %s (ID: %s)\n", targetSong.Name, targetSong.ID)

	// 3. Ensure runtime dependencies and download via go-ytdlp
	fmt.Println("Ensuring go-ytdlp dependencies are installed...")
	ytdlp.MustInstallAll(ctx)

	fmt.Println("Downloading audio and converting to Opus...")
	dl := ytdlp.New().
		Format("bestaudio").
		ExtractAudio().
		AudioFormat("opus").
		AudioQuality("0").
		Output("./temp_song.opus").
		NoPlaylist()

	_, err = dl.Run(ctx, "https://www.youtube.com/watch?v="+targetSong.ID)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}

	fmt.Println("Song successfully downloaded to: ./temp_song.opus")
}
```

---

## Core API Reference

### `NewClient()`
Creates a new `Client` instance configured with default connection settings.

### `client.Search(ctx, query, params, authRequired)`
Queries the YouTube Music catalog for tracks, albums, playlists, or artists matching the string query.
- **Returns**: `*SearchResults, error`

### `client.GetSearchSuggestions(ctx, query)`
Fetches live autocomplete suggestions for search terms.
- **Returns**: `[]string, error`

### `client.LoadArtist(ctx, artistID)`
Retrieves full details for an artist channel including layouts, popular tracks, and subscriber metrics.
- **Returns**: `*Artist, error`

### `client.LoadPlaylist(ctx, playlistID, continuation, browseParams, playlistURL, useNonMusicAPI)`
Retrieves detailed metadata and tracklistings for playlists or albums.
- **Returns**: `*Playlist, error`

### `client.GetSongLyrics(ctx, lyricsBrowseID)`
Retrieves the plaintext lyrics for a song.
- **Returns**: `string, error`
