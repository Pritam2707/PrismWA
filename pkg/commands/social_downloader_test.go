package commands

import (
	"strings"
	"testing"
)

func TestFindSocialSite(t *testing.T) {
	tests := []struct {
		host     string
		expected string
	}{
		{"instagram.com", "Instagram"},
		{"www.instagram.com", "Instagram"},
		{"instagr.am", "Instagram"},
		{"facebook.com", "Facebook"},
		{"fb.watch", "Facebook"},
		{"reddit.com", "Reddit"},
		{"v.redd.it", "Reddit"},
		{"twitter.com", "Twitter/X"},
		{"x.com", "Twitter/X"},
		{"tiktok.com", "TikTok"},
		{"threads.net", "Threads"},
		{"pin.it", "Pinterest"},
		{"twitch.tv", "Twitch"},
		{"soundcloud.com", "SoundCloud"},
		// Non-allowed / private addresses must return nil
		{"google.com", ""},
		{"localhost", ""},
		{"192.168.1.1", ""},
		{"10.0.0.1", ""},
	}

	for _, tt := range tests {
		site := findSocialSite(tt.host)
		if tt.expected == "" {
			if site != nil {
				t.Errorf("findSocialSite(%q) = %s, expected nil", tt.host, site.Name)
			}
		} else {
			if site == nil || site.Name != tt.expected {
				t.Errorf("findSocialSite(%q) = %v, expected %s", tt.host, site, tt.expected)
			}
		}
	}
}

func TestExtractSocialURL(t *testing.T) {
	ctx := &CommandContext{
		RawText: "!dl https://www.instagram.com/reel/C12345/?igsh=abc",
	}
	url := extractSocialURL(ctx)
	if url != "https://www.instagram.com/reel/C12345/?igsh=abc" {
		t.Errorf("unexpected extracted url: %q", url)
	}

	// URL with trailing punctuation
	ctx2 := &CommandContext{
		RawText: "Check this out https://x.com/elonmusk/status/987654321! It's awesome",
	}
	url2 := extractSocialURL(ctx2)
	if url2 != "https://x.com/elonmusk/status/987654321" {
		t.Errorf("unexpected extracted url with punctuation: %q", url2)
	}
}

func TestRegisterSocialDownloadCommands(t *testing.T) {
	r := InitRegistry()
	RegisterSocialDownloadCommands(r)

	expectedPrefixes := []string{
		"!dl", "!download", "!ig", "!insta", "!instagram",
		"!fb", "!facebook", "!reddit", "!tiktok", "!twitter",
		"!tw", "!threads", "!pin",
	}

	for _, p := range expectedPrefixes {
		found := false
		for _, h := range r.handlers {
			if h.Prefix == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command prefix %q to be registered", p)
		}
	}
}

func TestSocialSupportedList(t *testing.T) {
	list := socialSupportedList()
	if !strings.Contains(list, "Instagram") || !strings.Contains(list, "Reddit") || !strings.Contains(list, "TikTok") {
		t.Errorf("socialSupportedList() missing core sites, got:\n%s", list)
	}
}
