// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define the raw InnerTube response JSON structures and parsers.
//
// Key Components:
//   - YoutubeiBrowseResponse: represents InnerTube browse outcome JSON
//   - YoutubeiNextResponse: represents watch-next queue details
//   - Various visual renderers like MusicResponsiveListItemRenderer, MusicTwoRowItemRenderer, etc.
//
// Dependencies:
//   - strings
//   - strconv
//
// Error Types:
//   - None
//
package ytm

import (
	"strconv"
	"strings"
)

// TextRuns holds runs of formatted text.
type TextRuns struct {
	Runs []TextRun `json:"runs"`
}

func (tr TextRuns) FirstText() string {
	for _, r := range tr.Runs {
		if r.Text != " • " && r.Text != "" {
			return r.Text
		}
	}
	return ""
}

func (tr TextRuns) FirstTextOrNull() *string {
	txt := tr.FirstText()
	if txt == "" {
		return nil
	}
	return &txt
}

// TextRun holds a single segment of text.
type TextRun struct {
	Text               string              `json:"text"`
	NavigationEndpoint *NavigationEndpoint `json:"navigationEndpoint,omitempty"`
}

func (tr TextRun) PageType() string {
	if tr.NavigationEndpoint != nil && tr.NavigationEndpoint.BrowseEndpoint != nil {
		return tr.NavigationEndpoint.BrowseEndpoint.PageType()
	}
	return ""
}

// NavigationEndpoint represents an action navigation routing in InnerTube.
type NavigationEndpoint struct {
	WatchEndpoint               *WatchEndpoint               `json:"watchEndpoint,omitempty"`
	BrowseEndpoint              *BrowseEndpoint              `json:"browseEndpoint,omitempty"`
	SearchEndpoint              *SearchEndpoint              `json:"searchEndpoint,omitempty"`
	WatchPlaylistEndpoint       *WatchPlaylistEndpoint       `json:"watchPlaylistEndpoint,omitempty"`
	ChannelCreationFormEndpoint *ChannelCreationFormEndpoint `json:"channelCreationFormEndpoint,omitempty"`
	CommandMetadata             *CommandMetadata             `json:"commandMetadata,omitempty"`
	QueueUpdateCommand          *QueueUpdateCommand          `json:"queueUpdateCommand,omitempty"`
}

func (ne NavigationEndpoint) GetMediaItem() MediaItem {
	if ne.WatchEndpoint != nil {
		if ne.WatchEndpoint.VideoID != "" {
			return &Song{ID: CleanSongID(ne.WatchEndpoint.VideoID)}
		}
		if ne.WatchEndpoint.PlaylistID != "" {
			return &Playlist{ID: CleanPlaylistID(ne.WatchEndpoint.PlaylistID)}
		}
	}
	if ne.BrowseEndpoint != nil {
		return ne.BrowseEndpoint.GetMediaItem()
	}
	if ne.WatchPlaylistEndpoint != nil {
		return &Playlist{ID: CleanPlaylistID(ne.WatchPlaylistEndpoint.PlaylistID)}
	}
	return nil
}

// QueueUpdateCommand holds player queue additions.
type QueueUpdateCommand struct {
	FetchContentsCommand struct {
		WatchEndpoint WatchEndpoint `json:"watchEndpoint"`
	} `json:"fetchContentsCommand"`
}

// WatchEndpoint represents a video playback target.
type WatchEndpoint struct {
	VideoID    string `json:"videoId"`
	PlaylistID string `json:"playlistId,omitempty"`
}

// BrowseEndpointContextMusicConfig defines browse item pages.
type BrowseEndpointContextMusicConfig struct {
	PageType string `json:"pageType"`
}

// BrowseEndpointContextSupportedConfigs wraps page layout configurations.
type BrowseEndpointContextSupportedConfigs struct {
	BrowseEndpointContextMusicConfig BrowseEndpointContextMusicConfig `json:"browseEndpointContextMusicConfig"`
}

// BrowseEndpoint represents a browse navigation target.
type BrowseEndpoint struct {
	BrowseID                              string                                 `json:"browseId"`
	BrowseEndpointContextSupportedConfigs *BrowseEndpointContextSupportedConfigs `json:"browseEndpointContextSupportedConfigs,omitempty"`
	Params                                string                                 `json:"params,omitempty"`
}

func (be BrowseEndpoint) PageType() string {
	if be.BrowseEndpointContextSupportedConfigs != nil {
		return be.BrowseEndpointContextSupportedConfigs.BrowseEndpointContextMusicConfig.PageType
	}
	return ""
}

func (be BrowseEndpoint) GetMediaItemType() string {
	pt := be.PageType()
	if strings.HasPrefix(pt, "MUSIC_PAGE_TYPE_ARTIST") || pt == "MUSIC_PAGE_TYPE_USER_CHANNEL" || pt == "MUSIC_PAGE_TYPE_LIBRARY_ARTIST" {
		return "ARTIST"
	}
	if pt == "MUSIC_PAGE_TYPE_PLAYLIST" || pt == "MUSIC_PAGE_TYPE_ALBUM" || pt == "MUSIC_PAGE_TYPE_AUDIOBOOK" || pt == "MUSIC_PAGE_TYPE_PODCAST" || pt == "MUSIC_PAGE_TYPE_RADIO" {
		return "PLAYLIST"
	}
	if pt == "MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_TRACK_PAGE" || pt == "MUSIC_PAGE_TYPE_UNKNOWN" {
		return "SONG"
	}
	return ""
}

func (be BrowseEndpoint) GetMediaItem() MediaItem {
	t := be.GetMediaItemType()
	if be.BrowseID == "" {
		return nil
	}
	switch t {
	case "ARTIST":
		return &Artist{ID: be.BrowseID}
	case "PLAYLIST":
		return &Playlist{ID: CleanPlaylistID(be.BrowseID)}
	case "SONG":
		return &Song{ID: CleanSongID(be.BrowseID)}
	}
	return nil
}

// SearchEndpoint represents a search target.
type SearchEndpoint struct {
	Query  string `json:"query"`
	Params string `json:"params,omitempty"`
}

// WatchPlaylistEndpoint represents playing a playlist.
type WatchPlaylistEndpoint struct {
	PlaylistID string `json:"playlistId"`
	Params     string `json:"params,omitempty"`
}

// ChannelCreationFormEndpoint represents channel updates.
type ChannelCreationFormEndpoint struct {
	ChannelCreationToken string `json:"channelCreationToken"`
}

// CommandMetadata stores web-specific platform type.
type CommandMetadata struct {
	WebCommandMetadata struct {
		WebPageType string `json:"webPageType"`
	} `json:"webCommandMetadata"`
}

// MusicThumbnailRenderer is the standard thumbnail image container.
type MusicThumbnailRenderer struct {
	Thumbnail struct {
		Thumbnails []Thumbnail `json:"thumbnails"`
	} `json:"thumbnail"`
}

// ThumbnailRenderer wraps MusicThumbnailRenderer.
type ThumbnailRenderer struct {
	MusicThumbnailRenderer *MusicThumbnailRenderer `json:"musicThumbnailRenderer,omitempty"`
}

func (tr ThumbnailRenderer) ToThumbnailProvider() *ThumbnailProvider {
	if tr.MusicThumbnailRenderer == nil {
		return nil
	}
	return NewThumbnailProvider(tr.MusicThumbnailRenderer.Thumbnail.Thumbnails)
}

// MusicResponsiveListItemRenderer represents responsive items in lists.
type MusicResponsiveListItemRenderer struct {
	PlaylistItemData  *RendererPlaylistItemData `json:"playlistItemData,omitempty"`
	FlexColumns       []FlexColumn              `json:"flexColumns,omitempty"`
	FixedColumns      []FixedColumn             `json:"fixedColumns,omitempty"`
	Thumbnail         *ThumbnailRenderer        `json:"thumbnail,omitempty"`
	NavigationEndpoint *NavigationEndpoint       `json:"navigationEndpoint,omitempty"`
	Menu              *Menu                     `json:"menu,omitempty"`
	Index             *TextRuns                 `json:"index,omitempty"`
	Badges            []Badge                   `json:"badges,omitempty"`
}

type RendererPlaylistItemData struct {
	VideoID             string `json:"videoId"`
	PlaylistSetVideoID  string `json:"playlistSetVideoId,omitempty"`
}

type FlexColumn struct {
	MusicResponsiveListItemFlexColumnRenderer struct {
		Text *TextRuns `json:"text,omitempty"`
	} `json:"musicResponsiveListItemFlexColumnRenderer"`
}

type FixedColumn struct {
	MusicResponsiveListItemFixedColumnRenderer struct {
		Text *TextRuns `json:"text,omitempty"`
	} `json:"musicResponsiveListItemFixedColumnRenderer"`
}

type Badge struct {
	MusicInlineBadgeRenderer *struct {
		Icon *struct {
			IconType string `json:"iconType"`
		} `json:"icon"`
	} `json:"musicInlineBadgeRenderer"`
}

func (b Badge) IsExplicit() bool {
	return b.MusicInlineBadgeRenderer != nil && b.MusicInlineBadgeRenderer.Icon != nil && b.MusicInlineBadgeRenderer.Icon.IconType == "MUSIC_EXPLICIT_BADGE"
}

func (r *MusicResponsiveListItemRenderer) ParseItem(hl string) (MediaItem, string) {
	videoID := ""
	if r.PlaylistItemData != nil {
		videoID = r.PlaylistItemData.VideoID
	} else if r.NavigationEndpoint != nil && r.NavigationEndpoint.WatchEndpoint != nil {
		videoID = r.NavigationEndpoint.WatchEndpoint.VideoID
	}

	browseID := ""
	if r.NavigationEndpoint != nil && r.NavigationEndpoint.BrowseEndpoint != nil {
		browseID = r.NavigationEndpoint.BrowseEndpoint.BrowseID
	}

	if videoID == "" && r.Thumbnail != nil && r.Thumbnail.MusicThumbnailRenderer != nil {
		for _, t := range r.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails {
			if strings.HasPrefix(t.URL, "https://i.ytimg.com/vi/") {
				end := strings.Index(t.URL[23:], "/")
				if end != -1 {
					videoID = t.URL[23 : 23+end]
					break
				}
			}
		}
	}

	var title string
	if len(r.FlexColumns) > 0 && r.FlexColumns[0].MusicResponsiveListItemFlexColumnRenderer.Text != nil {
		title = r.FlexColumns[0].MusicResponsiveListItemFlexColumnRenderer.Text.FirstText()
	}

	var duration int64
	for _, col := range r.FixedColumns {
		if col.MusicResponsiveListItemFixedColumnRenderer.Text != nil {
			t := col.MusicResponsiveListItemFixedColumnRenderer.Text.FirstText()
			if parsed := parseDurationMs(t); parsed > 0 {
				duration = parsed
				break
			}
		}
	}

	thumbnailProvider := r.Thumbnail.ToThumbnailProvider()
	isExplicit := false
	for _, b := range r.Badges {
		if b.IsExplicit() {
			isExplicit = true
			break
		}
	}

	var artists []Artist
	var album *Playlist

	if r.Menu != nil {
		for _, item := range r.Menu.MenuRenderer.Items {
			if item.MenuNavigationItemRenderer != nil && item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint != nil {
				be := item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint
				if be.BrowseID != "" {
					mit := be.GetMediaItemType()
					if mit == "ARTIST" {
						artists = append(artists, Artist{ID: be.BrowseID})
					} else if mit == "PLAYLIST" && album == nil {
						album = &Playlist{ID: CleanPlaylistID(be.BrowseID)}
					}
				}
			}
		}
	}

	// Try extracting artists/album from flex columns
	if len(r.FlexColumns) > 1 && r.FlexColumns[1].MusicResponsiveListItemFlexColumnRenderer.Text != nil {
		flexText := r.FlexColumns[1].MusicResponsiveListItemFlexColumnRenderer.Text
		for _, run := range flexText.Runs {
			if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
				be := run.NavigationEndpoint.BrowseEndpoint
				if be.GetMediaItemType() == "ARTIST" {
					found := false
					for _, a := range artists {
						if a.ID == be.BrowseID {
							found = true
							break
						}
					}
					if !found {
						artists = append(artists, Artist{
							ID:   be.BrowseID,
							Name: run.Text,
						})
					}
				} else if be.GetMediaItemType() == "PLAYLIST" && album == nil {
					album = &Playlist{
						ID:   CleanPlaylistID(be.BrowseID),
						Name: run.Text,
					}
				}
			}
		}
	}

	if len(r.FlexColumns) > 1 && r.FlexColumns[1].MusicResponsiveListItemFlexColumnRenderer.Text != nil {
		name := r.FlexColumns[1].MusicResponsiveListItemFlexColumnRenderer.Text.FirstText()
		if name != "" {
			for i := range artists {
				if artists[i].Name == "" {
					artists[i].Name = name
				}
			}
			if len(artists) == 0 {
				artists = append(artists, Artist{Name: name})
			}
		}
	}

	var playlistSetVideoID string
	if r.PlaylistItemData != nil {
		playlistSetVideoID = r.PlaylistItemData.PlaylistSetVideoID
	}

	if videoID != "" {
		songType := SongTypeSong
		if len(r.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails) > 0 {
			t := r.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails[0]
			if t.Height != t.Width {
				songType = SongTypeVideo
			}
		}
		return &Song{
			ID:         CleanSongID(videoID),
			Name:       title,
			DurationMs: duration,
			Type:       songType,
			IsExplicit: isExplicit,
			Thumbnail:  thumbnailProvider,
			Artists:    artists,
			Album:      album,
		}, playlistSetVideoID
	}

	if browseID != "" {
		pt := r.NavigationEndpoint.BrowseEndpoint.PageType()
		mit := r.NavigationEndpoint.BrowseEndpoint.GetMediaItemType()
		if mit == "PLAYLIST" {
			var playlistType PlaylistType
			switch pt {
			case "MUSIC_PAGE_TYPE_PLAYLIST":
				playlistType = PlaylistTypePlaylist
			case "MUSIC_PAGE_TYPE_ALBUM":
				playlistType = PlaylistTypeAlbum
			case "MUSIC_PAGE_TYPE_AUDIOBOOK":
				playlistType = PlaylistTypeAudiobook
			case "MUSIC_PAGE_TYPE_PODCAST":
				playlistType = PlaylistTypePodcast
			case "MUSIC_PAGE_TYPE_RADIO":
				playlistType = PlaylistTypeRadio
			default:
				playlistType = PlaylistTypePlaylist
			}

			return &Playlist{
				ID:              CleanPlaylistID(browseID),
				Name:            title,
				Thumbnail:       thumbnailProvider,
				Type:            playlistType,
				Artists:         artists,
				TotalDurationMs: duration,
			}, playlistSetVideoID
		} else if mit == "ARTIST" {
			var art Artist
			if len(artists) > 0 {
				art = artists[0]
			}
			art.ID = browseID
			art.Name = title
			art.Thumbnail = thumbnailProvider
			return &art, playlistSetVideoID
		}
	}

	return nil, ""
}

// MusicTwoRowItemRenderer represents visual grid/row items.
type MusicTwoRowItemRenderer struct {
	NavigationEndpoint NavigationEndpoint `json:"navigationEndpoint"`
	Title              TextRuns           `json:"title"`
	Subtitle           *TextRuns          `json:"subtitle,omitempty"`
	ThumbnailRenderer  ThumbnailRenderer  `json:"thumbnailRenderer"`
	Menu               *Menu              `json:"menu,omitempty"`
	SubtitleBadges     []Badge            `json:"subtitleBadges,omitempty"`
}

func (r *MusicTwoRowItemRenderer) toMediaItem() MediaItem {
	var artists []Artist
	if r.Subtitle != nil {
		for _, run := range r.Subtitle.Runs {
			if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
				be := run.NavigationEndpoint.BrowseEndpoint
				if be.GetMediaItemType() == "ARTIST" {
					artists = append(artists, Artist{
						ID:   be.BrowseID,
						Name: run.Text,
					})
				}
			}
		}
	}

	thumbnailProvider := r.ThumbnailRenderer.ToThumbnailProvider()
	titleText := r.Title.FirstText()

	if r.NavigationEndpoint.WatchEndpoint != nil {
		var album *Playlist
		if r.Menu != nil {
			for _, item := range r.Menu.MenuRenderer.Items {
				if item.MenuNavigationItemRenderer != nil && item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint != nil {
					be := item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint
					if be.BrowseID != "" && be.GetMediaItemType() == "PLAYLIST" {
						album = &Playlist{ID: CleanPlaylistID(be.BrowseID)}
						break
					}
				}
			}
		}

		songType := SongTypeSong
		if len(r.ThumbnailRenderer.MusicThumbnailRenderer.Thumbnail.Thumbnails) > 0 {
			t := r.ThumbnailRenderer.MusicThumbnailRenderer.Thumbnail.Thumbnails[0]
			if t.Height != t.Width {
				songType = SongTypeVideo
			}
		}

		isExplicit := false
		for _, b := range r.SubtitleBadges {
			if b.IsExplicit() {
				isExplicit = true
				break
			}
		}

		return &Song{
			ID:         CleanSongID(r.NavigationEndpoint.WatchEndpoint.VideoID),
			Type:       songType,
			Name:       titleText,
			Thumbnail:  thumbnailProvider,
			Artists:    artists,
			IsExplicit: isExplicit,
			Album:      album,
		}
	}

	if r.NavigationEndpoint.WatchPlaylistEndpoint != nil {
		return &Playlist{
			ID:        CleanPlaylistID(r.NavigationEndpoint.WatchPlaylistEndpoint.PlaylistID),
			Type:      PlaylistTypeRadio,
			Name:      titleText,
			Thumbnail: thumbnailProvider,
		}
	}

	if r.NavigationEndpoint.BrowseEndpoint != nil {
		be := r.NavigationEndpoint.BrowseEndpoint
		browseID := be.BrowseID
		if browseID == "" {
			return nil
		}
		pt := be.PageType()
		mit := be.GetMediaItemType()

		switch mit {
		case "ARTIST":
			return &Artist{
				ID:        browseID,
				Name:      titleText,
				Thumbnail: thumbnailProvider,
			}
		case "PLAYLIST":
			var playlistType PlaylistType
			switch pt {
			case "MUSIC_PAGE_TYPE_PLAYLIST":
				playlistType = PlaylistTypePlaylist
			case "MUSIC_PAGE_TYPE_ALBUM":
				playlistType = PlaylistTypeAlbum
			case "MUSIC_PAGE_TYPE_AUDIOBOOK":
				playlistType = PlaylistTypeAudiobook
			case "MUSIC_PAGE_TYPE_PODCAST":
				playlistType = PlaylistTypePodcast
			case "MUSIC_PAGE_TYPE_RADIO":
				playlistType = PlaylistTypeRadio
			default:
				playlistType = PlaylistTypePlaylist
			}

			return &Playlist{
				ID:        CleanPlaylistID(browseID),
				Type:      playlistType,
				Artists:   artists,
				Name:      titleText,
				Thumbnail: thumbnailProvider,
			}
		case "SONG":
			return &Song{
				ID:        CleanSongID(browseID),
				Name:      titleText,
				Thumbnail: thumbnailProvider,
				Artists:   artists,
			}
		}
	}

	return nil
}

// MusicMultiRowListItemRenderer represents podcast show list items.
type MusicMultiRowListItemRenderer struct {
	Title       TextRuns           `json:"title"`
	Subtitle    TextRuns           `json:"subtitle"`
	Thumbnail   ThumbnailRenderer  `json:"thumbnail"`
	Menu        Menu               `json:"menu"`
	OnTap       *OnTap             `json:"onTap,omitempty"`
	SecondTitle *TextRuns          `json:"secondTitle,omitempty"`
}

type OnTap struct {
	WatchEndpoint struct {
		WatchEndpointMusicSupportedConfigs struct {
			WatchEndpointMusicConfig struct {
				MusicVideoType string `json:"musicVideoType"`
			} `json:"watchEndpointMusicConfig"`
		} `json:"watchEndpointMusicSupportedConfigs"`
	} `json:"watchEndpoint"`
}

func (r *MusicMultiRowListItemRenderer) toMediaItem(hl string) MediaItem {
	var album *Playlist
	if r.SecondTitle != nil && len(r.SecondTitle.Runs) > 0 {
		run := r.SecondTitle.Runs[0]
		if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
			album = &Playlist{
				ID:   CleanPlaylistID(run.NavigationEndpoint.BrowseEndpoint.BrowseID),
				Name: run.Text,
			}
		}
	}

	if album == nil {
		for _, item := range r.Menu.MenuRenderer.Items {
			if item.MenuNavigationItemRenderer != nil && item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint != nil {
				be := item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint
				if be.BrowseID != "" {
					if be.PageType() == "MUSIC_PAGE_TYPE_PODCAST_SHOW_DETAIL_PAGE" {
						album = &Playlist{
							ID:   CleanPlaylistID(be.BrowseID),
							Type: PlaylistTypePodcast,
						}
						break
					} else if be.GetMediaItemType() == "PLAYLIST" {
						album = &Playlist{
							ID: CleanPlaylistID(be.BrowseID),
						}
						break
					}
				}
			}
		}
	}

	var artists []Artist
	for _, run := range r.Subtitle.Runs {
		if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
			be := run.NavigationEndpoint.BrowseEndpoint
			if be.GetMediaItemType() == "ARTIST" {
				artists = append(artists, Artist{
					ID:   be.BrowseID,
					Name: run.Text,
				})
			}
		}
	}

	var duration int64
	if len(r.Subtitle.Runs) > 0 {
		t := r.Subtitle.Runs[len(r.Subtitle.Runs)-1].Text
		duration = parseDurationMs(t)
	}

	firstTitle := r.Title.Runs[0]
	songID := ""
	if firstTitle.NavigationEndpoint != nil && firstTitle.NavigationEndpoint.BrowseEndpoint != nil {
		songID = firstTitle.NavigationEndpoint.BrowseEndpoint.BrowseID
	}

	songType := SongTypeSong
	if r.OnTap != nil && r.OnTap.WatchEndpoint.WatchEndpointMusicSupportedConfigs.WatchEndpointMusicConfig.MusicVideoType == "MUSIC_VIDEO_TYPE_PODCAST_EPISODE" {
		songType = SongTypePodcast
	}

	return &Song{
		ID:         CleanSongID(songID),
		Name:       firstTitle.Text,
		Thumbnail:  r.Thumbnail.ToThumbnailProvider(),
		DurationMs: duration,
		Type:       songType,
		Artists:    artists,
		Album:      album,
	}
}

// ContinuationItemRenderer handles paginating items.
type ContinuationItemRenderer struct {
	ContinuationEndpoint struct {
		ContinuationCommand struct {
			Token string `json:"token"`
		} `json:"continuationCommand"`
	} `json:"continuationEndpoint"`
}

// YoutubeiShelfContentsItem holds the decoded row details inside shelves.
type YoutubeiShelfContentsItem struct {
	MusicTwoRowItemRenderer         *MusicTwoRowItemRenderer         `json:"musicTwoRowItemRenderer,omitempty"`
	MusicResponsiveListItemRenderer *MusicResponsiveListItemRenderer `json:"musicResponsiveListItemRenderer,omitempty"`
	MusicMultiRowListItemRenderer   *MusicMultiRowListItemRenderer   `json:"musicMultiRowListItemRenderer,omitempty"`
	ContinuationItemRenderer         *ContinuationItemRenderer         `json:"continuationItemRenderer,omitempty"`
}

func (i *YoutubeiShelfContentsItem) ParseItem(hl string) (MediaItem, string) {
	if i.MusicTwoRowItemRenderer != nil {
		return i.MusicTwoRowItemRenderer.toMediaItem(), ""
	}
	if i.MusicResponsiveListItemRenderer != nil {
		return i.MusicResponsiveListItemRenderer.ParseItem(hl)
	}
	if i.MusicMultiRowListItemRenderer != nil {
		return i.MusicMultiRowListItemRenderer.toMediaItem(hl), ""
	}
	return nil, ""
}

// GridRenderer represents visual grids.
type GridRenderer struct {
	Items  []YoutubeiShelfContentsItem `json:"items"`
	Header *struct {
		GridHeaderRenderer HeaderRenderer `json:"gridHeaderRenderer"`
	} `json:"header,omitempty"`
}

// HeaderRenderer represents general page or section titles.
type HeaderRenderer struct {
	Title               *TextRuns           `json:"title,omitempty"`
	Strapline           *TextRuns           `json:"strapline,omitempty"`
	SubscriptionButton  *SubscriptionButton `json:"subscriptionButton,omitempty"`
	PlayButton          *MoreContentButton  `json:"playButton,omitempty"`
	Description         *TextRuns           `json:"description,omitempty"`
	Thumbnail           *Thumbnails         `json:"thumbnail,omitempty"`
	ForegroundThumbnail *Thumbnails         `json:"foregroundThumbnail,omitempty"`
	Subtitle            *TextRuns           `json:"subtitle,omitempty"`
	SecondSubtitle      *TextRuns           `json:"secondSubtitle,omitempty"`
	MoreContentButton   *MoreContentButton  `json:"moreContentButton,omitempty"`
}

func (h *HeaderRenderer) GetThumbnails() []Thumbnail {
	if h != nil && h.Thumbnail != nil {
		return h.Thumbnail.GetThumbnails()
	}
	return nil
}

type Thumbnails struct {
	MusicThumbnailRenderer         *MusicThumbnailRenderer `json:"musicThumbnailRenderer,omitempty"`
	CroppedSquareThumbnailRenderer *MusicThumbnailRenderer `json:"croppedSquareThumbnailRenderer,omitempty"`
}

func (t Thumbnails) GetThumbnails() []Thumbnail {
	if t.MusicThumbnailRenderer != nil {
		return t.MusicThumbnailRenderer.Thumbnail.Thumbnails
	}
	if t.CroppedSquareThumbnailRenderer != nil {
		return t.CroppedSquareThumbnailRenderer.Thumbnail.Thumbnails
	}
	return nil
}

type SubscriptionButton struct {
	SubscribeButtonRenderer struct {
		Subscribed          bool     `json:"subscribed"`
		SubscriberCountText TextRuns `json:"subscriberCountText"`
		ChannelID           string   `json:"channelId"`
	} `json:"subscribeButtonRenderer"`
}

type MoreContentButton struct {
	ButtonRenderer struct {
		NavigationEndpoint NavigationEndpoint `json:"navigationEndpoint"`
	} `json:"buttonRenderer"`
}

type MusicDescriptionShelfRenderer struct {
	Description TextRuns        `json:"description"`
	Header      *TextRuns       `json:"header,omitempty"`
}

type MusicCardShelfHeaderBasicRenderer struct {
	Title *TextRuns `json:"title,omitempty"`
}

type MusicCardShelfRenderer struct {
	Thumbnail *ThumbnailRenderer `json:"thumbnail,omitempty"`
	Title     TextRuns           `json:"title"`
	Subtitle  TextRuns           `json:"subtitle"`
	Menu      Menu               `json:"menu"`
	Header    struct {
		MusicCardShelfHeaderBasicRenderer *MusicCardShelfHeaderBasicRenderer `json:"musicCardShelfHeaderBasicRenderer,omitempty"`
	} `json:"header"`
}

func (c *MusicCardShelfRenderer) GetMediaItem() MediaItem {
	titleText := c.Title.FirstText()
	var item MediaItem
	if len(c.Title.Runs) > 0 {
		run := c.Title.Runs[0]
		if run.NavigationEndpoint != nil {
			if run.NavigationEndpoint.WatchEndpoint != nil {
				item = &Song{
					ID:   CleanSongID(run.NavigationEndpoint.WatchEndpoint.VideoID),
					Name: titleText,
				}
			} else if run.NavigationEndpoint.BrowseEndpoint != nil {
				item = run.NavigationEndpoint.BrowseEndpoint.GetMediaItem()
			}
		}
	}
	if item == nil {
		return nil
	}

	var artists []Artist
	var album *Playlist
	for _, run := range c.Subtitle.Runs {
		if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
			be := run.NavigationEndpoint.BrowseEndpoint
			mit := be.GetMediaItemType()
			if mit == "ARTIST" {
				artists = append(artists, Artist{
					ID:   be.BrowseID,
					Name: run.Text,
				})
			} else if mit == "PLAYLIST" && be.PageType() == "MUSIC_PAGE_TYPE_ALBUM" {
				album = &Playlist{
					ID:   CleanPlaylistID(be.BrowseID),
					Name: run.Text,
					Type: PlaylistTypeAlbum,
				}
			}
		}
	}

	if len(artists) == 0 && len(c.Subtitle.Runs) > 0 {
		if text := c.Subtitle.FirstText(); text != "" {
			artists = append(artists, Artist{Name: text})
		}
	}

	thumbnailProvider := c.Thumbnail.ToThumbnailProvider()
	switch it := item.(type) {
	case *Song:
		it.Artists = artists
		it.Thumbnail = thumbnailProvider
		it.Album = album
		return it
	case *Playlist:
		it.Artists = artists
		it.Thumbnail = thumbnailProvider
		return it
	case *Artist:
		it.Thumbnail = thumbnailProvider
		return it
	}
	return item
}

type MusicResponsiveHeaderRenderer struct {
	Title           *TextRuns `json:"title,omitempty"`
	StraplineTextOne *TextRuns `json:"straplineTextOne,omitempty"`
	Description     *struct {
		MusicDescriptionShelfRenderer MusicDescriptionShelfRenderer `json:"musicDescriptionShelfRenderer"`
	} `json:"description,omitempty"`
	Thumbnail *struct {
		MusicThumbnailRenderer *MusicThumbnailRenderer `json:"musicThumbnailRenderer,omitempty"`
	} `json:"thumbnail,omitempty"`
}

type MusicEditablePlaylistDetailHeaderRenderer struct {
	Header struct {
		MusicResponsiveHeaderRenderer *MusicResponsiveHeaderRenderer `json:"musicResponsiveHeaderRenderer,omitempty"`
	} `json:"header"`
}

type Header struct {
	MusicCarouselShelfBasicHeaderRenderer     *HeaderRenderer                            `json:"musicCarouselShelfBasicHeaderRenderer,omitempty"`
	MusicImmersiveHeaderRenderer              *HeaderRenderer                            `json:"musicImmersiveHeaderRenderer,omitempty"`
	MusicVisualHeaderRenderer                 *HeaderRenderer                            `json:"musicVisualHeaderRenderer,omitempty"`
	MusicDetailHeaderRenderer                 *MusicDetailHeaderRenderer                 `json:"musicDetailHeaderRenderer,omitempty"`
	MusicEditablePlaylistDetailHeaderRenderer *MusicEditablePlaylistDetailHeaderRenderer `json:"musicEditablePlaylistDetailHeaderRenderer,omitempty"`
	MusicCardShelfHeaderBasicRenderer         *HeaderRenderer                            `json:"musicCardShelfHeaderBasicRenderer,omitempty"`
}

func (h Header) GetRenderer() *HeaderRenderer {
	if h.MusicCarouselShelfBasicHeaderRenderer != nil {
		return h.MusicCarouselShelfBasicHeaderRenderer
	}
	if h.MusicImmersiveHeaderRenderer != nil {
		return h.MusicImmersiveHeaderRenderer
	}
	if h.MusicVisualHeaderRenderer != nil {
		return h.MusicVisualHeaderRenderer
	}
	if h.MusicCardShelfHeaderBasicRenderer != nil {
		return h.MusicCardShelfHeaderBasicRenderer
	}
	if h.MusicDetailHeaderRenderer != nil {
		d := h.MusicDetailHeaderRenderer
		return &HeaderRenderer{
			Title:              d.Title,
			Strapline:          d.Strapline,
			SubscriptionButton: d.SubscriptionButton,
			PlayButton:         d.PlayButton,
			Description:        d.Description,
			Thumbnail:          d.Thumbnail,
			Subtitle:           d.Subtitle,
			SecondSubtitle:     d.SecondSubtitle,
			MoreContentButton:  d.MoreContentButton,
		}
	}
	if h.MusicEditablePlaylistDetailHeaderRenderer != nil && h.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer != nil {
		r := h.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer
		var thumbnails *Thumbnails
		if r.Thumbnail != nil && r.Thumbnail.MusicThumbnailRenderer != nil {
			thumbnails = &Thumbnails{MusicThumbnailRenderer: r.Thumbnail.MusicThumbnailRenderer}
		}
		var desc *TextRuns
		if r.Description != nil {
			desc = &r.Description.MusicDescriptionShelfRenderer.Description
		}
		return &HeaderRenderer{
			Title:       r.Title,
			Description: desc,
			Thumbnail:   thumbnails,
			Subtitle:    r.StraplineTextOne,
		}
	}
	return nil
}

type MusicDetailHeaderRenderer struct {
	Title              *TextRuns           `json:"title,omitempty"`
	Strapline          *TextRuns           `json:"strapline,omitempty"`
	SubscriptionButton *SubscriptionButton `json:"subscriptionButton,omitempty"`
	PlayButton         *MoreContentButton  `json:"playButton,omitempty"`
	Description        *TextRuns           `json:"description,omitempty"`
	Thumbnail          *Thumbnails         `json:"thumbnail,omitempty"`
	Subtitle           *TextRuns           `json:"subtitle,omitempty"`
	SecondSubtitle     *TextRuns           `json:"secondSubtitle,omitempty"`
	MoreContentButton  *MoreContentButton  `json:"moreContentButton,omitempty"`
	Menu               *Menu               `json:"menu,omitempty"`
}

type MusicCarouselShelfRenderer struct {
	Header            *Header                     `json:"header,omitempty"`
	Contents          []YoutubeiShelfContentsItem `json:"contents"`
	NumItemsPerColumn *int                        `json:"numItemsPerColumn,omitempty"`
}

type MusicShelfRenderer struct {
	Title          *TextRuns                   `json:"title,omitempty"`
	Contents       []YoutubeiShelfContentsItem `json:"contents,omitempty"`
	Continuations  []Continuation              `json:"continuations,omitempty"`
	BottomEndpoint *NavigationEndpoint         `json:"bottomEndpoint,omitempty"`
}

// YoutubeiShelf represents visual groups of items.
type YoutubeiShelf struct {
	MusicShelfRenderer          *MusicShelfRenderer          `json:"musicShelfRenderer,omitempty"`
	MusicCarouselShelfRenderer *MusicCarouselShelfRenderer `json:"musicCarouselShelfRenderer,omitempty"`
	MusicDescriptionShelfRenderer *MusicDescriptionShelfRenderer `json:"musicDescriptionShelfRenderer,omitempty"`
	MusicPlaylistShelfRenderer  *MusicShelfRenderer          `json:"musicPlaylistShelfRenderer,omitempty"`
	MusicCardShelfRenderer      *MusicCardShelfRenderer      `json:"musicCardShelfRenderer,omitempty"`
	MusicResponsiveHeaderRenderer *MusicResponsiveHeaderRenderer `json:"musicResponsiveHeaderRenderer,omitempty"`
	MusicEditablePlaylistDetailHeaderRenderer *MusicEditablePlaylistDetailHeaderRenderer `json:"musicEditablePlaylistDetailHeaderRenderer,omitempty"`
	GridRenderer                *GridRenderer                `json:"gridRenderer,omitempty"`
	ItemSectionRenderer         *ItemSectionRenderer         `json:"itemSectionRenderer,omitempty"`
	ContinuationItemRenderer    *ContinuationItemRenderer    `json:"continuationItemRenderer,omitempty"`
}

func (s YoutubeiShelf) Title() string {
	if s.MusicShelfRenderer != nil && s.MusicShelfRenderer.Title != nil {
		return s.MusicShelfRenderer.Title.FirstText()
	}
	if s.MusicCarouselShelfRenderer != nil && s.MusicCarouselShelfRenderer.Header != nil {
		if r := s.MusicCarouselShelfRenderer.Header.GetRenderer(); r != nil && r.Title != nil {
			return r.Title.FirstText()
		}
	}
	if s.MusicDescriptionShelfRenderer != nil && s.MusicDescriptionShelfRenderer.Header != nil {
		return s.MusicDescriptionShelfRenderer.Header.FirstText()
	}
	if s.MusicCardShelfRenderer != nil {
		return s.MusicCardShelfRenderer.Title.FirstText()
	}
	if s.GridRenderer != nil && s.GridRenderer.Header != nil {
		return s.GridRenderer.Header.GridHeaderRenderer.Title.FirstText()
	}
	if s.MusicResponsiveHeaderRenderer != nil && s.MusicResponsiveHeaderRenderer.Title != nil {
		return s.MusicResponsiveHeaderRenderer.Title.FirstText()
	}
	if s.MusicEditablePlaylistDetailHeaderRenderer != nil && s.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer != nil {
		r := s.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer
		if r.Title != nil {
			return r.Title.FirstText()
		}
	}
	return ""
}

func (s YoutubeiShelf) Description() string {
	if s.MusicDescriptionShelfRenderer != nil {
		return s.MusicDescriptionShelfRenderer.Description.FirstText()
	}
	if s.MusicResponsiveHeaderRenderer != nil && s.MusicResponsiveHeaderRenderer.Description != nil {
		return s.MusicResponsiveHeaderRenderer.Description.MusicDescriptionShelfRenderer.Description.FirstText()
	}
	if s.MusicEditablePlaylistDetailHeaderRenderer != nil && s.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer != nil {
		r := s.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer
		if r.Description != nil {
			return r.Description.MusicDescriptionShelfRenderer.Description.FirstText()
		}
	}
	return ""
}

func (s YoutubeiShelf) Thumbnails() []Thumbnail {
	if s.MusicResponsiveHeaderRenderer != nil && s.MusicResponsiveHeaderRenderer.Thumbnail != nil && s.MusicResponsiveHeaderRenderer.Thumbnail.MusicThumbnailRenderer != nil {
		return s.MusicResponsiveHeaderRenderer.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails
	}
	if s.MusicEditablePlaylistDetailHeaderRenderer != nil && s.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer != nil {
		r := s.MusicEditablePlaylistDetailHeaderRenderer.Header.MusicResponsiveHeaderRenderer
		if r.Thumbnail != nil && r.Thumbnail.MusicThumbnailRenderer != nil {
			return r.Thumbnail.MusicThumbnailRenderer.Thumbnail.Thumbnails
		}
	}
	return nil
}

func (s YoutubeiShelf) Artist() *Artist {
	if s.MusicResponsiveHeaderRenderer != nil && s.MusicResponsiveHeaderRenderer.StraplineTextOne != nil {
		for _, run := range s.MusicResponsiveHeaderRenderer.StraplineTextOne.Runs {
			if item := run.NavigationEndpoint.GetMediaItem(); item != nil {
				if a, ok := item.(*Artist); ok {
					a.Name = run.Text
					return a
				}
			}
		}
	}
	return nil
}

func (s YoutubeiShelf) GetRenderer() any {
	if s.MusicShelfRenderer != nil {
		return s.MusicShelfRenderer
	}
	if s.MusicCarouselShelfRenderer != nil {
		return s.MusicCarouselShelfRenderer
	}
	if s.MusicPlaylistShelfRenderer != nil {
		return s.MusicPlaylistShelfRenderer
	}
	if s.MusicCardShelfRenderer != nil {
		return s.MusicCardShelfRenderer
	}
	if s.GridRenderer != nil {
		return s.GridRenderer
	}
	if s.ItemSectionRenderer != nil {
		return s.ItemSectionRenderer
	}
	return nil
}

func (s YoutubeiShelf) GetHeader() *Header {
	if s.MusicCarouselShelfRenderer != nil {
		return s.MusicCarouselShelfRenderer.Header
	}
	if s.MusicCardShelfRenderer != nil {
		basic := s.MusicCardShelfRenderer.Header.MusicCardShelfHeaderBasicRenderer
		var hr *HeaderRenderer
		if basic != nil {
			hr = &HeaderRenderer{Title: basic.Title}
		}
		return &Header{MusicCardShelfHeaderBasicRenderer: hr}
	}
	return nil
}

func (s YoutubeiShelf) GetNavigationEndpoint() *NavigationEndpoint {
	header := s.GetHeader()
	if header != nil {
		r := header.GetRenderer()
		if r != nil && r.MoreContentButton != nil {
			return &r.MoreContentButton.ButtonRenderer.NavigationEndpoint
		}
	}
	if s.MusicShelfRenderer != nil && s.MusicShelfRenderer.BottomEndpoint != nil {
		return s.MusicShelfRenderer.BottomEndpoint
	}
	return nil
}

func (s YoutubeiShelf) GetMediaItems(hl string) []MediaItem {
	var items []MediaItem
	if s.MusicShelfRenderer != nil {
		for _, item := range s.MusicShelfRenderer.Contents {
			if parsed, _ := item.ParseItem(hl); parsed != nil {
				items = append(items, parsed)
			}
		}
	} else if s.MusicCarouselShelfRenderer != nil {
		for _, item := range s.MusicCarouselShelfRenderer.Contents {
			if parsed, _ := item.ParseItem(hl); parsed != nil {
				items = append(items, parsed)
			}
		}
	} else if s.MusicPlaylistShelfRenderer != nil {
		for _, item := range s.MusicPlaylistShelfRenderer.Contents {
			if parsed, _ := item.ParseItem(hl); parsed != nil {
				items = append(items, parsed)
			}
		}
	} else if s.GridRenderer != nil {
		for _, item := range s.GridRenderer.Items {
			if parsed, _ := item.ParseItem(hl); parsed != nil {
				items = append(items, parsed)
			}
		}
	} else if s.ItemSectionRenderer != nil {
		items = s.ItemSectionRenderer.GetMediaItems(hl)
	}
	return items
}

func (s YoutubeiShelf) GetMediaItemsAndSetIDs(hl string) []struct {
	Item  MediaItem
	SetID string
} {
	var results []struct {
		Item  MediaItem
		SetID string
	}
	var contents []YoutubeiShelfContentsItem
	if s.MusicShelfRenderer != nil {
		contents = s.MusicShelfRenderer.Contents
	} else if s.MusicCarouselShelfRenderer != nil {
		contents = s.MusicCarouselShelfRenderer.Contents
	} else if s.MusicPlaylistShelfRenderer != nil {
		contents = s.MusicPlaylistShelfRenderer.Contents
	} else if s.GridRenderer != nil {
		contents = s.GridRenderer.Items
	}

	if len(contents) > 0 {
		for _, item := range contents {
			if parsed, setID := item.ParseItem(hl); parsed != nil {
				results = append(results, struct {
					Item  MediaItem
					SetID string
				}{Item: parsed, SetID: setID})
			}
		}
	} else if s.ItemSectionRenderer != nil {
		for _, c := range s.ItemSectionRenderer.Contents {
			if c.MusicResponsiveListItemRenderer != nil {
				if parsed, setID := c.MusicResponsiveListItemRenderer.ParseItem(hl); parsed != nil {
					results = append(results, struct {
						Item  MediaItem
						SetID string
					}{Item: parsed, SetID: setID})
				}
			}
			if c.PlaylistVideoListRenderer != nil {
				for _, item := range c.PlaylistVideoListRenderer.Contents {
					if song := item.PlaylistVideoRenderer.GetSong(); song != nil {
						results = append(results, struct {
							Item  MediaItem
							SetID string
						}{Item: song, SetID: ""})
					}
				}
			}
			if c.VideoRenderer != nil {
				if song := c.VideoRenderer.GetSong(); song != nil {
					results = append(results, struct {
						Item  MediaItem
						SetID string
					}{Item: song, SetID: ""})
				}
			}
		}
	}
	return results
}

// ItemSectionRenderer represents a general section containing list rows.
type ItemSectionRenderer struct {
	Contents []ItemSectionRendererContent `json:"contents"`
}

func (r ItemSectionRenderer) GetMediaItems(hl string) []MediaItem {
	var items []MediaItem
	for _, c := range r.Contents {
		if c.MusicResponsiveListItemRenderer != nil {
			if parsed, _ := c.MusicResponsiveListItemRenderer.ParseItem(hl); parsed != nil {
				items = append(items, parsed)
			}
		}
		if c.PlaylistVideoListRenderer != nil {
			for _, item := range c.PlaylistVideoListRenderer.Contents {
				if song := item.PlaylistVideoRenderer.GetSong(); song != nil {
					items = append(items, song)
				}
			}
		}
		if c.VideoRenderer != nil {
			if song := c.VideoRenderer.GetSong(); song != nil {
				items = append(items, song)
			}
		}
	}
	return items
}

type ItemSectionRendererContent struct {
	DidYouMeanRenderer               *DidYouMeanRenderer               `json:"didYouMeanRenderer,omitempty"`
	MusicResponsiveListItemRenderer  *MusicResponsiveListItemRenderer  `json:"musicResponsiveListItemRenderer,omitempty"`
	PlaylistVideoListRenderer        *PlaylistVideoListRenderer        `json:"playlistVideoListRenderer,omitempty"`
	VideoRenderer                    *VideoRenderer                    `json:"videoRenderer,omitempty"`
}

type DidYouMeanRenderer struct {
	CorrectedQuery TextRuns `json:"correctedQuery"`
}

type PlaylistVideoListRenderer struct {
	Contents []PlaylistVideoItem `json:"contents"`
}

type PlaylistVideoItem struct {
	PlaylistVideoRenderer VideoRenderer `json:"playlistVideoRenderer"`
}

type VideoRenderer struct {
	VideoID         string             `json:"videoId"`
	Title           TextRuns           `json:"title"`
	ShortBylineText *TextRuns          `json:"shortBylineText,omitempty"`
	LongBylineText  *TextRuns          `json:"longBylineText,omitempty"`
	LengthSeconds   *string            `json:"lengthSeconds,omitempty"`
	IsPlayable      *bool              `json:"isPlayable,omitempty"`
	Thumbnail       *ThumbnailRenderer `json:"thumbnail,omitempty"`
}

func (vr VideoRenderer) GetSong() *Song {
	if vr.IsPlayable != nil && !*vr.IsPlayable {
		return nil
	}

	var artists []Artist
	var combinedRuns []TextRun
	if vr.ShortBylineText != nil {
		combinedRuns = append(combinedRuns, vr.ShortBylineText.Runs...)
	}
	if vr.LongBylineText != nil {
		combinedRuns = append(combinedRuns, vr.LongBylineText.Runs...)
	}

	for _, run := range combinedRuns {
		if run.NavigationEndpoint != nil && run.NavigationEndpoint.CommandMetadata.WebCommandMetadata.WebPageType == "WEB_PAGE_TYPE_CHANNEL" {
			if run.NavigationEndpoint.BrowseEndpoint != nil {
				artists = append(artists, Artist{
					ID:   run.NavigationEndpoint.BrowseEndpoint.BrowseID,
					Name: run.Text,
				})
			}
		}
	}

	var duration int64
	if vr.LengthSeconds != nil {
		if s, err := strconv.ParseInt(*vr.LengthSeconds, 10, 64); err == nil {
			duration = s * 1000
		}
	}

	return &Song{
		ID:         CleanSongID(vr.VideoID),
		Name:       vr.Title.FirstText(),
		DurationMs: duration,
		Thumbnail:  vr.Thumbnail.ToThumbnailProvider(),
		Artists:    artists,
	}
}

// YoutubeiBrowseResponse represents the complete page structure from browse endpoint.
type YoutubeiBrowseResponse struct {
	Contents                 *Contents                 `json:"contents,omitempty"`
	ContinuationContents     *ContinuationContents     `json:"continuationContents,omitempty"`
	Header                   *Header                   `json:"header,omitempty"`
	Microformat              *Microformat              `json:"microformat,omitempty"`
	OnResponseReceivedActions []OnResponseReceivedAction `json:"onResponseReceivedActions,omitempty"`
}

type Microformat struct {
	MicroformatDataRenderer struct {
		URLCanonical string `json:"urlCanonical"`
	} `json:"microformatDataRenderer"`
}

type Contents struct {
	SingleColumnBrowseResultsRenderer *SingleColumnBrowseResultsRenderer `json:"singleColumnBrowseResultsRenderer,omitempty"`
	TwoColumnBrowseResultsRenderer    *TwoColumnBrowseResultsRenderer    `json:"twoColumnBrowseResultsRenderer,omitempty"`
}

type SingleColumnBrowseResultsRenderer struct {
	Tabs []Tab `json:"tabs"`
}

type TwoColumnBrowseResultsRenderer struct {
	Tabs              []Tab              `json:"tabs"`
	SecondaryContents *SecondaryContents `json:"secondaryContents,omitempty"`
}

type SecondaryContents struct {
	SectionListRenderer SectionListRenderer `json:"sectionListRenderer"`
}

type Tab struct {
	TabRenderer struct {
		Content  *Content            `json:"content,omitempty"`
		Endpoint *TabRendererEndpoint `json:"endpoint,omitempty"`
	} `json:"tabRenderer"`
}

type TabRendererEndpoint struct {
	BrowseEndpoint BrowseEndpoint `json:"browseEndpoint"`
}

type Content struct {
	SectionListRenderer *SectionListRenderer `json:"sectionListRenderer,omitempty"`
	MusicQueueRenderer  *MusicQueueRenderer  `json:"musicQueueRenderer,omitempty"`
}

type SectionListRenderer struct {
	Contents      []YoutubeiShelf `json:"contents,omitempty"`
	Header        *HeaderChips    `json:"header,omitempty"`
	Continuations []Continuation  `json:"continuations,omitempty"`
}

type HeaderChips struct {
	ChipCloudRenderer struct {
		Chips []SearchChip `json:"chips"`
	} `json:"chipCloudRenderer"`
}

type SearchChip struct {
	ChipCloudChipRenderer struct {
		NavigationEndpoint NavigationEndpoint `json:"navigationEndpoint"`
		Text               *TextRuns          `json:"text,omitempty"`
	} `json:"chipCloudChipRenderer"`
}

type ContinuationContents struct {
	SectionListContinuation        *SectionListRenderer `json:"sectionListContinuation,omitempty"`
	MusicPlaylistShelfContinuation *MusicShelfRenderer  `json:"musicPlaylistShelfContinuation,omitempty"`
}

type OnResponseReceivedAction struct {
	AppendContinuationItemsAction *struct {
		ContinuationItems []YoutubeiShelfContentsItem `json:"continuationItems"`
	} `json:"appendContinuationItemsAction,omitempty"`
}

func (br *YoutubeiBrowseResponse) CToken() string {
	if br.ContinuationContents != nil && br.ContinuationContents.SectionListContinuation != nil {
		for _, c := range br.ContinuationContents.SectionListContinuation.Continuations {
			if token := c.GetToken(); token != "" {
				return token
			}
		}
	}
	if br.Contents != nil && br.Contents.SingleColumnBrowseResultsRenderer != nil {
		for _, tab := range br.Contents.SingleColumnBrowseResultsRenderer.Tabs {
			if tab.TabRenderer.Content != nil && tab.TabRenderer.Content.SectionListRenderer != nil {
				for _, c := range tab.TabRenderer.Content.SectionListRenderer.Continuations {
					if token := c.GetToken(); token != "" {
						return token
					}
				}
			}
		}
	}
	return ""
}

func (br *YoutubeiBrowseResponse) GetShelves(hasContinuation bool) []YoutubeiShelf {
	if hasContinuation {
		if br.ContinuationContents != nil && br.ContinuationContents.SectionListContinuation != nil {
			return br.ContinuationContents.SectionListContinuation.Contents
		}
		return nil
	}
	if br.Contents != nil && br.Contents.SingleColumnBrowseResultsRenderer != nil && len(br.Contents.SingleColumnBrowseResultsRenderer.Tabs) > 0 {
		tab := br.Contents.SingleColumnBrowseResultsRenderer.Tabs[0]
		if tab.TabRenderer.Content != nil && tab.TabRenderer.Content.SectionListRenderer != nil {
			return tab.TabRenderer.Content.SectionListRenderer.Contents
		}
	}
	return nil
}

func (br *YoutubeiBrowseResponse) GetHeaderChips() []FilterChip {
	if br.Contents == nil || br.Contents.SingleColumnBrowseResultsRenderer == nil || len(br.Contents.SingleColumnBrowseResultsRenderer.Tabs) == 0 {
		return nil
	}
	tab := br.Contents.SingleColumnBrowseResultsRenderer.Tabs[0]
	if tab.TabRenderer.Content == nil || tab.TabRenderer.Content.SectionListRenderer == nil || tab.TabRenderer.Content.SectionListRenderer.Header == nil {
		return nil
	}
	var chips []FilterChip
	for _, chip := range tab.TabRenderer.Content.SectionListRenderer.Header.ChipCloudRenderer.Chips {
		if chip.ChipCloudChipRenderer.Text != nil && chip.ChipCloudChipRenderer.NavigationEndpoint.BrowseEndpoint != nil {
			chips = append(chips, FilterChip{
				Text:   chip.ChipCloudChipRenderer.Text.FirstText(),
				Params: chip.ChipCloudChipRenderer.NavigationEndpoint.BrowseEndpoint.Params,
			})
		}
	}
	return chips
}

// YoutubeiNextResponse represents the complete page structure from /next endpoint.
type YoutubeiNextResponse struct {
	Contents struct {
		SingleColumnMusicWatchNextResultsRenderer struct {
			TabbedRenderer struct {
				WatchNextTabbedResultsRenderer struct {
					Tabs []Tab `json:"tabs"`
				} `json:"watchNextTabbedResultsRenderer"`
			} `json:"tabbedRenderer"`
		} `json:"singleColumnMusicWatchNextResultsRenderer"`
	} `json:"contents"`
}

type MusicQueueRenderer struct {
	Content             *MusicQueueRendererContent `json:"content,omitempty"`
	SubHeaderChipCloud  *SubHeaderChipCloud        `json:"subHeaderChipCloud,omitempty"`
}

type SubHeaderChipCloud struct {
	ChipCloudRenderer struct {
		Chips []RadioChip `json:"chips"`
	} `json:"chipCloudRenderer"`
}

type RadioChip struct {
	ChipCloudChipRenderer struct {
		NavigationEndpoint struct {
			QueueUpdateCommand struct {
				FetchContentsCommand struct {
					WatchEndpoint WatchEndpoint `json:"watchEndpoint"`
				} `json:"fetchContentsCommand"`
			} `json:"queueUpdateCommand"`
		} `json:"navigationEndpoint"`
	} `json:"chipCloudChipRenderer"`
}

type MusicQueueRendererContent struct {
	PlaylistPanelRenderer PlaylistPanelRenderer `json:"playlistPanelRenderer"`
}

type PlaylistPanelRenderer struct {
	Contents      []ResponseRadioItem `json:"contents"`
	Continuations []Continuation      `json:"continuations,omitempty"`
}

type ResponseRadioItem struct {
	PlaylistPanelVideoRenderer        *PlaylistPanelVideoRenderer        `json:"playlistPanelVideoRenderer,omitempty"`
	PlaylistPanelVideoWrapperRenderer *PlaylistPanelVideoWrapperRenderer `json:"playlistPanelVideoWrapperRenderer,omitempty"`
}

func (r ResponseRadioItem) GetRenderer() *PlaylistPanelVideoRenderer {
	if r.PlaylistPanelVideoRenderer != nil {
		return r.PlaylistPanelVideoRenderer
	}
	if r.PlaylistPanelVideoWrapperRenderer != nil {
		return r.PlaylistPanelVideoWrapperRenderer.PrimaryRenderer.PlaylistPanelVideoRenderer
	}
	return nil
}

type PlaylistPanelVideoWrapperRenderer struct {
	PrimaryRenderer struct {
		PlaylistPanelVideoRenderer *PlaylistPanelVideoRenderer `json:"playlistPanelVideoRenderer,omitempty"`
	} `json:"primaryRenderer"`
}

type PlaylistPanelVideoRenderer struct {
	VideoID       string             `json:"videoId"`
	Title         TextRuns           `json:"title"`
	LongBylineText TextRuns           `json:"longBylineText"`
	Menu          Menu               `json:"menu"`
	Thumbnail     MusicThumbnailRenderer `json:"thumbnail"`
	Badges        []Badge            `json:"badges,omitempty"`
	LengthText    *TextRuns          `json:"lengthText,omitempty"`
}

func (vr PlaylistPanelVideoRenderer) GetAlbum() *Playlist {
	for _, run := range vr.LongBylineText.Runs {
		if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
			if run.NavigationEndpoint.BrowseEndpoint.PageType() == "MUSIC_PAGE_TYPE_ALBUM" {
				return &Playlist{
					ID:   CleanPlaylistID(run.NavigationEndpoint.BrowseEndpoint.BrowseID),
					Name: run.Text,
				}
			}
		}
	}
	return nil
}

func (vr PlaylistPanelVideoRenderer) GetArtists() []Artist {
	var artists []Artist
	combined := append(vr.LongBylineText.Runs, vr.Title.Runs...)
	for _, run := range combined {
		if run.NavigationEndpoint != nil && run.NavigationEndpoint.BrowseEndpoint != nil {
			be := run.NavigationEndpoint.BrowseEndpoint
			if be.GetMediaItemType() == "ARTIST" {
				artists = append(artists, Artist{
					ID:   be.BrowseID,
					Name: run.Text,
				})
			}
		}
	}

	if len(artists) > 0 {
		return artists
	}

	for _, item := range vr.Menu.MenuRenderer.Items {
		if item.MenuNavigationItemRenderer != nil && item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint != nil {
			be := item.MenuNavigationItemRenderer.NavigationEndpoint.BrowseEndpoint
			if be.GetMediaItemType() == "ARTIST" {
				return []Artist{{ID: be.BrowseID}}
			}
		}
	}

	// Title-only fallback artist
	for _, run := range vr.LongBylineText.Runs {
		if run.NavigationEndpoint == nil && run.Text != "" {
			return []Artist{{ID: "", Name: run.Text}}
		}
	}

	return nil
}

type Menu struct {
	MenuRenderer struct {
		Items           []MenuItem        `json:"items"`
		TopLevelButtons []TopLevelButton  `json:"topLevelButtons,omitempty"`
	} `json:"menuRenderer"`
}

type TopLevelButton struct {
	ButtonRenderer *struct {
		Icon *struct {
			IconType string `json:"iconType"`
		} `json:"icon,omitempty"`
		NavigationEndpoint *NavigationEndpoint `json:"navigationEndpoint,omitempty"`
	} `json:"buttonRenderer,omitempty"`
}

type MenuItem struct {
	MenuNavigationItemRenderer *MenuNavigationItemRenderer `json:"menuNavigationItemRenderer,omitempty"`
}

type MenuNavigationItemRenderer struct {
	Icon struct {
		IconType string `json:"iconType"`
	} `json:"icon"`
	NavigationEndpoint NavigationEndpoint `json:"navigationEndpoint"`
}

type Continuation struct {
	NextContinuationData      *ContinuationData `json:"nextContinuationData,omitempty"`
	NextRadioContinuationData *ContinuationData `json:"nextRadioContinuationData,omitempty"`
}

func (c Continuation) GetToken() string {
	if c.NextContinuationData != nil {
		return c.NextContinuationData.Continuation
	}
	if c.NextRadioContinuationData != nil {
		return c.NextRadioContinuationData.Continuation
	}
	return ""
}

type ContinuationData struct {
	Continuation string `json:"continuation"`
}

type YoutubeiNextContinuationResponse struct {
	ContinuationContents struct {
		PlaylistPanelContinuation PlaylistPanelRenderer `json:"playlistPanelContinuation"`
	} `json:"continuationContents"`
}

func parseDurationMs(s string) int64 {
	if strings.Contains(s, ":") {
		parts := strings.Split(s, ":")
		if len(parts) == 2 {
			m, _ := strconv.Atoi(parts[0])
			sec, _ := strconv.Atoi(parts[1])
			return int64(m*60+sec) * 1000
		} else if len(parts) == 3 {
			h, _ := strconv.Atoi(parts[0])
			m, _ := strconv.Atoi(parts[1])
			sec, _ := strconv.Atoi(parts[2])
			return int64(h*3600+m*60+sec) * 1000
		}
	}

	var totalSeconds int64
	parts := strings.Fields(s)
	for i := 0; i < len(parts); i++ {
		val, err := strconv.Atoi(strings.TrimSuffix(parts[i], "+"))
		if err != nil {
			continue
		}
		if i+1 < len(parts) {
			unit := strings.ToLower(parts[i+1])
			if strings.HasPrefix(unit, "hour") || strings.HasPrefix(unit, "hr") || unit == "h" {
				totalSeconds += int64(val * 3600)
			} else if strings.HasPrefix(unit, "minute") || strings.HasPrefix(unit, "min") || unit == "m" {
				totalSeconds += int64(val * 60)
			} else if strings.HasPrefix(unit, "second") || strings.HasPrefix(unit, "sec") || unit == "s" {
				totalSeconds += int64(val)
			}
		}
	}
	if totalSeconds > 0 {
		return totalSeconds * 1000
	}
	return 0
}
