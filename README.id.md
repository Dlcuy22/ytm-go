# ytm-go

*Baca dokumentasi ini dalam bahasa lain: [English](README.md).*

`ytm-go` adalah library Go port dari [ytm-kt](https://gitlab.com/syk.sh/ytm-kt). Library ini mendukung berbagai fitur pencarian dan ekstraksi metadata dari YouTube Music.

## Fitur Utama

- **Pencarian & Autocomplete**: Mencari lagu, album, playlist, artis, dan saran pencarian real-time.
- **Ekstraksi Metadata Lengkap**: Mengambil informasi track (judul, artis, durasi, thumbnail, eksplisit status).
- **Profil Artis**: Mengambil informasi detail artis beserta visual layout rilis album/lagu populer mereka.
- **Playlist & Album**: Membaca daftar lagu lengkap (tracklist) dari playlist publik maupun album resmi.
- **Lirik Lagu**: Mengambil teks lirik lagu plaintext menggunakan ID lirik.
- **Rekomendasi (Radio)**: Membangun daftar putar rekomendasi (radio) secara dinamis.

---

## Getting Started

Tidak seperti `ytm-kt` asli yang menggunakan `NewPipeExtractor` untuk format decoding/downloading media, `ytm-go` sepenuhnya mendelegasikan proses pengunduhan media ke **`yt-dlp`** melalui local Go bindings **[go-ytdlp](https://github.com/lrstanley/go-ytdlp)**. 

Ini memisahkan logika parsing catalog (di `ytm-go`) dengan logika download yang rentan berubah karena proteksi signature cipher YouTube.

### Cara Penggunaan Paling Simple

Berikut adalah contoh untuk mencari sebuah lagu dan mengunduhnya ke format Ogg/Opus secara langsung:

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

	// 1. Inisialisasi client ytm-go
	client := ytm.NewClient()

	// 2. Cari lagu di YouTube Music
	fmt.Println("Mencari lagu 'MURI MURI EVOLUTION'...")
	results, err := client.Search(ctx, "MURI MURI EVOLUTION - NANAOAKARI", "", false)
	if err != nil {
		log.Fatalf("Pencarian gagal: %v", err)
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
		log.Fatalf("Lagu tidak ditemukan di hasil pencarian.")
	}
	fmt.Printf("Lagu Ditemukan: %s (ID: %s)\n", targetSong.Name, targetSong.ID)

	// 3. Mengunduh & mengubah ke format Opus menggunakan go-ytdlp
	fmt.Println("Memastikan dependensi runtime (yt-dlp, ffmpeg) terinstall...")
	ytdlp.MustInstallAll(ctx)

	fmt.Println("Mengunduh media...")
	dl := ytdlp.New().
		Format("bestaudio").
		ExtractAudio().
		AudioFormat("opus").
		AudioQuality("0").
		Output("./temp_song.opus").
		NoPlaylist()

	_, err = dl.Run(ctx, "https://www.youtube.com/watch?v="+targetSong.ID)
	if err != nil {
		log.Fatalf("Proses unduh gagal: %v", err)
	}

	fmt.Println("Lagu berhasil diunduh ke: ./temp_song.opus")
}
```

---

## Dokumentasi API Utama

### `NewClient()`
Membuat instance client baru untuk berkomunikasi dengan InnerTube API.

### `client.Search(ctx, query, params, authRequired)`
Mencari item di katalog musik berdasarkan string query.
- **Returns**: `*SearchResults, error`

### `client.GetSearchSuggestions(ctx, query)`
Mendapatkan autocomplete saran pencarian yang sesuai dengan input pengguna.
- **Returns**: `[]string, error`

### `client.LoadArtist(ctx, artistID)`
Memuat informasi profil lengkap milik musisi atau channel, termasuk jumlah subscriber dan rilis musik terpopuler.
- **Returns**: `*Artist, error`

### `client.LoadPlaylist(ctx, playlistID, continuation, browseParams, playlistURL, useNonMusicAPI)`
Memuat daftar tracklist dari sebuah playlist, album, atau radio feed.
- **Returns**: `*Playlist, error`

### `client.GetSongLyrics(ctx, lyricsBrowseID)`
Mengambil teks lirik (plaintext) dari track terkait menggunakan ID lirik.
- **Returns**: `string, error`
