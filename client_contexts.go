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
