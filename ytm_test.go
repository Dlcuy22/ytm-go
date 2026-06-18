// Package ytm provides models and definitions for YouTube Music InnerTube requests and responses.
//
// Purpose:
//   Verify core ytm-go client functionality including hash generation, auth parsing, and caching.
//
// Key Components:
//   - TestSAPISIDHashCalculation: validates auth token hashing algorithm
//   - TestNewAuthFromCookieString: validates cookie parser
//   - TestCPNGeneration: validates correlation tracking token formatting
//   - TestClientContextPresets: validates headers and hl locale tags mapping
//
// Dependencies:
//   - testing
//   - strings
//
// Error Types:
//   - None
//
package ytm

import (
	"strings"
	"testing"
)

/*
TestSAPISIDHashCalculation ensures that SAPISID hashes are formatted correctly.

    params:
          t: testing framework context
*/
func TestSAPISIDHashCalculation(t *testing.T) {
	sapisid := "test_sapisid_token_123"
	hash := GenerateSAPISIDHash(sapisid)

	if !strings.HasPrefix(hash, "SAPISIDHASH ") {
		t.Errorf("expected hash to start with SAPISIDHASH, got: %s", hash)
	}

	parts := strings.Split(strings.TrimPrefix(hash, "SAPISIDHASH "), "_")
	if len(parts) != 2 {
		t.Errorf("expected two parts separated by underscore, got: %v", parts)
	}
}

/*
TestNewAuthFromCookieString ensures cookies containing SAPISID are correctly parsed.

    params:
          t: testing framework context
*/
func TestNewAuthFromCookieString(t *testing.T) {
	cookieStr := "YSC=123; SAPISID=sapisid_val_xyz; GPS=456"
	auth, err := NewAuthFromCookieString(cookieStr)
	if err != nil {
		t.Fatalf("unexpected error parsing cookie: %v", err)
	}

	if auth.SAPISID != "sapisid_val_xyz" {
		t.Errorf("expected SAPISID to be sapisid_val_xyz, got: %s", auth.SAPISID)
	}

	if auth.Cookie != cookieStr {
		t.Errorf("expected raw cookie string to match input, got: %s", auth.Cookie)
	}
}

/*
TestCPNGeneration validates that cpn generation produces a valid 16-character tracking identifier.

    params:
          t: testing framework context
*/
func TestCPNGeneration(t *testing.T) {
	cpn := generateCpn()
	if len(cpn) != 16 {
		t.Errorf("expected cpn length 16, got: %d", len(cpn))
	}

	for _, c := range cpn {
		if !strings.ContainsRune(cpnAlphabet, c) {
			t.Errorf("invalid character in cpn: %c", c)
		}
	}
}

/*
TestClientContextPresets checks default locale setting and client contexts mapping.

    params:
          t: testing framework context
*/
func TestClientContextPresets(t *testing.T) {
	c := NewClient()
	if c.hl != "en-GB" {
		t.Errorf("expected default hl to be en-GB, got: %s", c.hl)
	}

	webRemix := GetContextWebRemix(c.hl)
	if webRemix.ClientName != "WEB_REMIX" || webRemix.HL != "en-GB" {
		t.Errorf("invalid web remix context: %+v", webRemix)
	}

	androidMusic := GetContextAndroidMusic(c.hl)
	if androidMusic.ClientName != "ANDROID_MUSIC" || androidMusic.Platform != "MOBILE" {
		t.Errorf("invalid android music context: %+v", androidMusic)
	}
}
