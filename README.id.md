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
- **Pustaka Akun**: Membaca playlist, lagu disukai, album dan artis milik akun, lengkap dengan paging.
- **Akun & Channel**: Mendaftar channel milik satu akun Google, membaca nama dan email, serta berpindah channel aktif.

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

### Autentikasi

`ytm-go` tidak menyediakan alur login. Library ini menerima header `Cookie:` hasil
tangkap dari browser yang sudah login, lalu memutarnya ulang:

```go
auth, err := ytm.NewAuthFromCookieString(cookieHeader)
if err != nil {
    return err // header tidak membawa SAPISID
}
client.SetAuth(auth)
client.SetVisitorID(visitorData) // opsional, menghemat satu request tiap start
```

Header yang dikirim harus utuh, bukan satu field saja. `SAPISID` sendirian hanya
menjadi benih hash `Authorization`; identitas sesi ada di `SID` atau
`__Secure-3PSID`. Header berisi `SAPISID` tanpa keduanya akan dijawab halaman
signed-out untuk setiap request.

`auth.SetChannel(pageID, dataSyncID, authUser)` mengarahkan sesi ke salah satu
channel milik akun yang punya beberapa channel. Cookie tetap sama; hanya header
akun yang berubah. `DataSyncID` hanya untuk pembukuan dan tidak pernah dikirim
dalam request.

### `client.GetAccounts(ctx)`
Mendaftar channel milik akun Google yang sedang login.
- **Returns**: `[]Account, error`

### `client.GetAccountInfo(ctx)`
Membaca nama tampilan, email, handle channel dan avatar akun yang sedang login.
- **Returns**: `*AccountInfo, error`

### `client.GetLikedSongs(ctx)` / `client.GetLikedSongsContinuation(ctx, token)`
Memuat lagu disukai akun secara bertahap (paging).
- **Returns**: `[]Song, *BuiltInContinuation, error`

### `client.GetLibraryPlaylists(ctx)` / `client.GetLibraryPlaylistsContinuation(ctx, token)`
Memuat playlist milik akun secara bertahap (paging).
- **Returns**: `[]Playlist, *BuiltInContinuation, error`

### `client.GetLikedAlbumsContinuation(ctx, token)`
Memuat album disukai secara bertahap. `GetLikedAlbums` mengambil halaman pertama.
- **Returns**: `[]Playlist, *BuiltInContinuation, error`

### `client.GetLikedArtistsContinuation(ctx, token)`
Memuat artis secara bertahap. `GetLikedArtists` mengambil halaman pertama.
- **Returns**: `[]Artist, *BuiltInContinuation, error`

### `client.SignedIn()`
Menandakan apakah respons terakhir dibuat untuk penonton yang sudah login.
- **Returns**: `bool`

Sesi mati **bukan** error HTTP: server menjawab 200 dengan halaman signed-out.
Karena itu reader ber-auth mengembalikan `ErrSessionExpired` dan bukan slice
kosong, supaya sesi rusak terlihat dan tidak tampak seperti pustaka kosong.
