// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Define preset ClientContext models to impersonate different device profiles.
//
// Key Components:
//   - ClientContext: Profile payload containing client version, name, and user-agent details
//   - Preset context generators: GetContextWebRemix, GetContextAndroid, etc.
//
// Dependencies:
//   - None
//
// Error Types:
//   - None
//
package ytm

const (
	YtmUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:145.0) Gecko/20100101 Firefox/145.0"
	DefaultHL    = "en-GB"
)

// ClientContext impersonates an InnerTube client context.
type ClientContext struct {
	HL            string `json:"hl"`
	Platform      string `json:"platform,omitempty"`
	ClientName    string `json:"clientName"`
	ClientVersion string `json:"clientVersion"`
	UserAgent     string `json:"userAgent,omitempty"`
	OsName        string `json:"osName,omitempty"`
	OsVersion     string `json:"osVersion,omitempty"`
	DeviceMake    string `json:"deviceMake,omitempty"`
	DeviceModel   string `json:"deviceModel,omitempty"`
	AcceptHeader  string `json:"acceptHeader,omitempty"`
	VisitorData   string `json:"visitorData,omitempty"`

	// ClientID is the numeric X-YouTube-Client-Name value that pairs with
	// ClientName. It is not part of the request body, so it carries no JSON
	// tag. An empty value leaves the historical WEB_REMIX header in place,
	// which is what every preset other than VISIONOS still relies on.
	ClientID string `json:"-"`
}

/*
GetContextWebRemix generates the desktop music web client context.

    params:
          hl: language tag (e.g. en-GB)
    returns:
          ClientContext: WEB_REMIX desktop client context
*/
func GetContextWebRemix(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		Platform:      "DESKTOP",
		ClientName:    "WEB_REMIX",
		ClientVersion: "1.20230306.01.00",
		UserAgent:     YtmUserAgent,
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}

/*
GetContextAndroid generates the Android YouTube app client context.

    params:
          hl: language tag
    returns:
          ClientContext: ANDROID client context
*/
func GetContextAndroid(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		Platform:      "MOBILE",
		ClientName:    "ANDROID",
		ClientVersion: "20.10.38",
		UserAgent:     "com.google.android.youtube/20.10.38 (Linux; U; Android 11) gzip",
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}

/*
GetContextMobile generates the mobile browser music client context.

    params:
          hl: language tag
    returns:
          ClientContext: WEB_REMIX mobile client context
*/
func GetContextMobile(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		Platform:      "MOBILE",
		ClientName:    "WEB_REMIX",
		ClientVersion: "1.20230503.01.00",
		OsName:        "Android",
		OsVersion:     "12",
		UserAgent:     "Mozilla/5.0 (Linux; Android 12; Pixel 3a) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.74 Mobile Safari/537.36,gzip(gfe)",
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}

/*
GetContextAndroidMusic generates the Android YouTube Music app client context.

    params:
          hl: language tag
    returns:
          ClientContext: ANDROID_MUSIC client context
*/
func GetContextAndroidMusic(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		Platform:      "MOBILE",
		ClientName:    "ANDROID_MUSIC",
		ClientVersion: "5.28.1",
		UserAgent:     "com.google.android.apps.youtube.music/5.28.1 (Linux; U; Android 11) gzip",
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}

/*
GetContextWeb generates the standard YouTube web client context.

    params:
          hl: language tag
    returns:
          ClientContext: WEB client context
*/
func GetContextWeb(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		Platform:      "DESKTOP",
		ClientName:    "WEB",
		ClientVersion: "2.20240509.00.00",
		UserAgent:     YtmUserAgent,
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}

/*
GetContextIOS generates the iOS YouTube app client context.

    params:
          hl: language tag
    returns:
          ClientContext: IOS client context
*/
func GetContextIOS(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		ClientName:    "IOS",
		ClientVersion: "19.29.1",
		DeviceMake:    "Apple",
		DeviceModel:   "iPhone16,2",
		OsName:        "iPhone",
		OsVersion:     "17.5.1.21F90",
		UserAgent:     "com.google.ios.youtube/19.29.1 (iPhone16,2; U; CPU iOS 17_5_1 like Mac OS X;)",
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}

/*
GetContextVisionOS generates the visionOS playback client context.

This is the profile that makes direct streaming possible. It is an unreleased
YouTube client identity that YouTube still answers with plain, already-authorised
media URLs: no signatureCipher, no PO token, and no SABR-only response. Those
URLs stream a whole track with an ordinary ranged GET.

Two properties are load-bearing and easy to lose in a refactor:

  - The context must carry visitorData. Without it the endpoint answers
    LOGIN_REQUIRED. That is why this client is only usable after
    ensureVisitorID has run, which doInnerTube already does for every path
    except visitor_id.
  - ClientName is what selects the profile. The X-YouTube-Client-Name header
    does not have to match, but ClientID keeps it consistent.

It is a playback-only profile. Unlike WEB_REMIX it cannot browse, search, or
carry a login, so catalogue and library calls must stay on WEB_REMIX.

    params:
          hl: language tag
    returns:
          ClientContext: VISIONOS client context
*/
func GetContextVisionOS(hl string) ClientContext {
	return ClientContext{
		HL:            hl,
		Platform:      "DESKTOP",
		ClientName:    "VISIONOS",
		ClientVersion: "1.02",
		ClientID:      "101",
		UserAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15",
		OsName:        "visionOS",
		OsVersion:     "26.5.23O471",
		DeviceMake:    "Apple",
		DeviceModel:   "RealityDevice17,1",
		AcceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	}
}
