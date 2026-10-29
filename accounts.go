// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//
//	Read the signed-in account: its display identity and the channels it owns.
//
// Key Components:
//   - GetAccounts: the channel list behind the account switcher
//   - GetAccountInfo: the display name and email of the active Google account
//   - accountsListResponse: the response nesting both readers walk
//
// Dependencies:
//   - context
//   - strconv
//   - strings
//
// Error Types:
//   - ErrLoginRequired: returned without credentials
//   - ErrSessionExpired: returned when the server answers signed out
package ytm

import (
	"context"
	"strconv"
	"strings"
)

// Account is one channel of the signed-in Google account.
type Account struct {
	Name          string
	Byline        string
	ChannelHandle string
	ThumbnailURL  string
	IsSelected    bool

	// SigninURL is the per-channel sign-in URL the listing reports. It is kept
	// because it carries the channel's X-Goog-AuthUser index, which AuthUser
	// exposes separately.
	SigninURL string

	// PageID is the channel browse ID, the "UC…" form. It is what ChannelID
	// needs on AuthState, so a switch does not have to re-derive it.
	PageID string

	// DataSyncID is the channel's datasync token. It is bookkeeping only: sending
	// it as user.onBehalfOfUser makes the server answer HTTP 500.
	DataSyncID string

	// AuthUser is the X-Goog-AuthUser index for this channel, parsed out of
	// SigninURL. It is the value the request header must carry.
	AuthUser string
}

// AccountInfo is the display identity of the active Google account.
type AccountInfo struct {
	Name          string
	Email         string
	ChannelHandle string
	ThumbnailURL  string
}

// accountText accepts both text shapes the account renderers use: the runs form
// and the simpleText form. The same endpoint returns one or the other depending
// on which host served it.
type accountText struct {
	Runs       []TextRun `json:"runs"`
	SimpleText string    `json:"simpleText"`
}

func (t accountText) value() string {
	if len(t.Runs) == 0 {
		return t.SimpleText
	}
	var b strings.Builder
	for _, r := range t.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// identityToken is one entry of the channel identity tokens. Exactly one field
// is set per entry, which is why they are separate pointers rather than a map.
type identityToken struct {
	AccountStateToken *struct {
		HasChannel       bool   `json:"hasChannel"`
		ObfuscatedGaiaID string `json:"obfuscatedGaiaId"`
	} `json:"accountStateToken,omitempty"`
	OfflineCacheKeyToken *struct {
		ClientCacheKey string `json:"clientCacheKey"`
	} `json:"offlineCacheKeyToken,omitempty"`
	AccountSigninToken *struct {
		SigninURL string `json:"signinUrl"`
	} `json:"accountSigninToken,omitempty"`
	DatasyncIDToken *struct {
		DatasyncID string `json:"datasyncIdToken"`
	} `json:"datasyncIdToken,omitempty"`
}

type accountItem struct {
	AccountName   accountText `json:"accountName"`
	AccountByline accountText `json:"accountByline"`
	ChannelHandle accountText `json:"channelHandle"`
	AccountPhoto  struct {
		Thumbnails []Thumbnail `json:"thumbnails"`
	} `json:"accountPhoto"`
	IsSelected      bool `json:"isSelected"`
	HasChannel      bool `json:"hasChannel"`
	ServiceEndpoint struct {
		SelectActiveIdentityEndpoint struct {
			SupportedTokens []identityToken `json:"supportedTokens"`
		} `json:"selectActiveIdentityEndpoint"`
	} `json:"serviceEndpoint"`
}

// googleAccountHeaderRenderer names the Google account itself, as opposed to
// one of its channels.
type googleAccountHeaderRenderer struct {
	Name  accountText `json:"name"`
	Email accountText `json:"email"`
}

// accountsListResponse is the shape account/accounts_list returns. The account
// items and the Google account header live in the same response, which is why
// one request serves both readers.
type accountsListResponse struct {
	Actions []struct {
		GetMultiPageMenuAction *struct {
			Menu struct {
				MultiPageMenuRenderer struct {
					Sections []struct {
						AccountSectionListRenderer *struct {
							Header *struct {
								GoogleAccountHeaderRenderer *googleAccountHeaderRenderer `json:"googleAccountHeaderRenderer,omitempty"`
							} `json:"header,omitempty"`
							Contents []struct {
								AccountItemSectionRenderer *struct {
									Contents []struct {
										AccountItem *accountItem `json:"accountItem,omitempty"`
									} `json:"contents"`
								} `json:"accountItemSectionRenderer,omitempty"`
							} `json:"contents"`
						} `json:"accountSectionListRenderer,omitempty"`
					} `json:"sections"`
				} `json:"multiPageMenuRenderer"`
			} `json:"menu"`
		} `json:"getMultiPageMenuAction,omitempty"`
	} `json:"actions"`
}

// accountsList posts to account/accounts_list.
//
// The request carries no body on purpose. Measured against a real account: with
// requestType ACCOUNTS_LIST_REQUEST_TYPE_CHANNEL_SWITCHER this endpoint answers a
// bare "OK" and no accounts at all, while an empty body returns both the channel
// items and the Google account header.
func (c *Client) accountsList(ctx context.Context) (*accountsListResponse, error) {
	if err := requireAuth(c.auth); err != nil {
		return nil, err
	}

	var resp accountsListResponse
	if err := c.doInnerTube(ctx, "account/accounts_list", GetContextWebRemix(c.hl), nil, true, &resp); err != nil {
		return nil, err
	}
	if !c.SignedIn() {
		return nil, ErrSessionExpired
	}
	return &resp, nil
}

// accountItems flattens the account items out of the response.
func (r *accountsListResponse) accountItems() []*accountItem {
	var items []*accountItem
	for _, action := range r.Actions {
		if action.GetMultiPageMenuAction == nil {
			continue
		}
		for _, section := range action.GetMultiPageMenuAction.Menu.MultiPageMenuRenderer.Sections {
			if section.AccountSectionListRenderer == nil {
				continue
			}
			for _, content := range section.AccountSectionListRenderer.Contents {
				if content.AccountItemSectionRenderer == nil {
					continue
				}
				for _, entry := range content.AccountItemSectionRenderer.Contents {
					if entry.AccountItem != nil {
						items = append(items, entry.AccountItem)
					}
				}
			}
		}
	}
	return items
}

// googleAccountHeader returns the header that names the Google account itself,
// as opposed to one of its channels.
func (r *accountsListResponse) googleAccountHeader() *googleAccountHeaderRenderer {
	for _, action := range r.Actions {
		if action.GetMultiPageMenuAction == nil {
			continue
		}
		for _, section := range action.GetMultiPageMenuAction.Menu.MultiPageMenuRenderer.Sections {
			if section.AccountSectionListRenderer == nil || section.AccountSectionListRenderer.Header == nil {
				continue
			}
			if h := section.AccountSectionListRenderer.Header.GoogleAccountHeaderRenderer; h != nil {
				return h
			}
		}
	}
	return nil
}

/*
GetAccounts returns the channels the signed-in Google account owns.

A plain account owns exactly one channel, and the listing then holds a single
item with IsSelected set. Brand accounts return several, and the one the session
currently uses is the one flagged IsSelected.

	params:
	      ctx: execution context
	returns:
	      []Account: the channels, in the order the account switcher shows them
	      error: network, parsing, or ErrSessionExpired
*/
func (c *Client) GetAccounts(ctx context.Context) ([]Account, error) {
	resp, err := c.accountsList(ctx)
	if err != nil {
		return nil, err
	}

	var accounts []Account
	for _, item := range resp.accountItems() {
		accounts = append(accounts, newAccount(item))
	}
	return accounts, nil
}

/*
GetAccountInfo returns the display identity of the active Google account.

The email is not in the channel items; it only appears in the account menu
header, which is why this reads a different part of the same response.

	params:
	      ctx: execution context
	returns:
	      *AccountInfo: name, email, channel handle and avatar
	      error: network, parsing, or ErrSessionExpired
*/
func (c *Client) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	resp, err := c.accountsList(ctx)
	if err != nil {
		return nil, err
	}

	info := &AccountInfo{}
	if header := resp.googleAccountHeader(); header != nil {
		info.Name = header.Name.value()
		info.Email = header.Email.value()
	}

	// The header carries the account, not the channel, so the handle and avatar
	// come from whichever channel is currently selected.
	for _, item := range resp.accountItems() {
		if !item.IsSelected {
			continue
		}
		if info.ChannelHandle == "" {
			info.ChannelHandle = item.ChannelHandle.value()
		}
		if info.ThumbnailURL == "" {
			info.ThumbnailURL = bestThumbnail(item.AccountPhoto.Thumbnails)
		}
		break
	}
	if info.Name == "" {
		// Older layouts drop the Google header and only expose the channel name.
		for _, item := range resp.accountItems() {
			if item.IsSelected {
				info.Name = item.AccountName.value()
				break
			}
		}
	}
	return info, nil
}

// newAccount converts one listing entry into an Account.
func newAccount(item *accountItem) Account {
	account := Account{
		Name:          item.AccountName.value(),
		Byline:        item.AccountByline.value(),
		ChannelHandle: item.ChannelHandle.value(),
		ThumbnailURL:  bestThumbnail(item.AccountPhoto.Thumbnails),
		IsSelected:    item.IsSelected,
	}

	for _, token := range item.ServiceEndpoint.SelectActiveIdentityEndpoint.SupportedTokens {
		switch {
		case token.OfflineCacheKeyToken != nil && token.OfflineCacheKeyToken.ClientCacheKey != "":
			account.PageID = channelIDFromCacheKey(token.OfflineCacheKeyToken.ClientCacheKey)
		case token.DatasyncIDToken != nil && token.DatasyncIDToken.DatasyncID != "":
			account.DataSyncID = token.DatasyncIDToken.DatasyncID
		case token.AccountSigninToken != nil && token.AccountSigninToken.SigninURL != "":
			account.SigninURL = token.AccountSigninToken.SigninURL
			account.AuthUser = authUserFromSigninURL(token.AccountSigninToken.SigninURL)
		}
	}
	return account
}

// channelIDFromCacheKey expands the switcher's cache key into a channel browse
// ID. The listing reports the channel key without its "UC" prefix; the browse
// endpoints and playlist ownership both want the prefixed form.
func channelIDFromCacheKey(key string) string {
	if key == "" || strings.HasPrefix(key, "UC") {
		return key
	}
	return "UC" + key
}

// authUserFromSigninURL pulls the authuser index out of a switcher sign-in URL,
// which looks like "/signin?action_handle_signin=true&authuser=3&next=%2F".
func authUserFromSigninURL(raw string) string {
	query := raw
	if _, rest, ok := strings.Cut(raw, "?"); ok {
		query = rest
	}
	for _, pair := range strings.Split(query, "&") {
		key, value, ok := strings.Cut(pair, "=")
		if ok && key == "authuser" {
			if _, err := strconv.Atoi(value); err == nil {
				return value
			}
		}
	}
	return ""
}

// bestThumbnail picks the widest entry, since the listings are ordered by usage
// context rather than by size.
func bestThumbnail(thumbnails []Thumbnail) string {
	best := ""
	bestWidth := -1
	for _, t := range thumbnails {
		if t.Width > bestWidth && t.URL != "" {
			best, bestWidth = t.URL, t.Width
		}
	}
	return best
}
