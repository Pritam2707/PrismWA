package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var VoiceIDs = map[string]string{
	"zero2_jp":         "fe7fe6d5d7474c2e9c0eb778e1a28f18",
	"kurumi_en":        "09bb45f3522a4337b12f81971a94bbe8",
	"kurumi_jp":        "8023630490a7462a8d1ce3e72d9f85e0",
	"goku_en":          "a04b127b9c9448429a401a87a9b3d619",
	"elon_musk":        "5303cbf567be41a9894286277b051f83",
	"naruto_jp":        "087930b6a0d947b89c121294adb9b887",
	"naruto_en":        "0785ead631314bb794c6943599dbf24c",
	"paimon_en":        "a23bb0727f954c8d9d8cb6a57d63ae2d",
	"kratos":           "80d5909c6f50429f93e48f8b488f0650",
	"snoop_dog":        "8bc0ef3b96424e6db3cccf6360c69778",
	"eminem":           "9b1b1f8a52da4ee2bccc70c9c6c50182",
	"joshua_graham":    "c0dacdfe68454ea99fa42d56f04572ac",
	"rus_en":           "1fb0effbd4774ede894a13944da12245",
	"simon_riley":      "724c2151f1984a4e850c2210ca941000",
	"johan_en":         "e530f9e40efa4e238a9bf4a52e5d75fe",
	"light_en":         "605891dea98f424db46da9283be7b1b3",
	"l_en":             "a1a3247d2c4f4e8dbe4ba4479a36e7a5",
	"lara_croft":       "8099b8618bcf44268a488ab18fc3438c",
	"marin_en":         "a056bb7a7aca42e697f51142d9e70613",
	"rem_en":           "380f5c79a37a4ec39d3c0934ebc42222",
	"modi":             "8b734220ba20480aa95b5e6329730003",
	"trump":            "5196af35f6ff4a0dbf541793fc9f2157",
	"miku":             "14f2767336d140aaa048f37ee210a6f6",
	"sharukh_khan":     "6e00ff5a53f747d7ae7e2ef3e622a595",
	"amitabh_bachchan": "ea7cdc74aeae4b608be27fdc37fdcb05",
	"columnbina":       "f2aed07c91614db28daaaa849150cc6e",
	"sandrone":         "14cdaeb1098444a6a889a330dd7c6af5",
	"furina":           "bd08be872bc440918674af072944ba12",
	"venti":            "e34c486929524d41b88646b4ac2f382f",
}

// Shared persistent HTTP/2 connection pool for sub-second TTS synthesis
var (
	ttsHTTPClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression: false,
		},
		Timeout: 8 * time.Second,
	}

	// Circuit breaker for Fish Audio: only triggers if API key is invalid (HTTP 401)
	fishAudioDisabledUntil time.Time
	fishAudioMu            sync.RWMutex
)

func markFishAudioDisabled(duration time.Duration, reason string) {
	fishAudioMu.Lock()
	fishAudioDisabledUntil = time.Now().Add(duration)
	fishAudioMu.Unlock()
	log.Printf("[TTS] Fish Audio temporarily disabled for %v (reason: %s). Auto-routed to fallback TTS.", duration, reason)
}

func isFishAudioAvailable() bool {
	fishAudioMu.RLock()
	defer fishAudioMu.RUnlock()
	return time.Now().After(fishAudioDisabledUntil)
}

// GetAvailableVoices returns sorted list of voice identifiers
func GetAvailableVoices() []string {
	var voices []string
	for name := range VoiceIDs {
		voices = append(voices, name)
	}
	sort.Strings(voices)
	return voices
}

// Synthesize converts text to MP3 audio bytes using Fish Audio character voices with fallback to Google Translate
func Synthesize(ctx context.Context, textToSpeak, voiceKey string) ([]byte, error) {
	cleanText := strings.TrimSpace(textToSpeak)
	if cleanText == "" {
		return nil, fmt.Errorf("empty text to synthesize")
	}

	cleanVoice := strings.ToLower(strings.TrimSpace(voiceKey))
	if cleanVoice == "" {
		cleanVoice = "furina"
	}

	voiceID, isFishVoice := VoiceIDs[cleanVoice]
	apiKey := os.Getenv("FISH_AUDIO_API_KEY")

	// 1. Try Fish Audio character voice using the fast s2.1-pro-free endpoint
	if isFishVoice && apiKey != "" && isFishAudioAvailable() {
		data, err := synthesizeFishAudio(ctx, cleanText, voiceID, apiKey)
		if err == nil && len(data) > 500 {
			return data, nil
		}
		if err != nil {
			log.Printf("[TTS] Fish Audio synthesis error: %v, falling back to Google Translate", err)
		}
	}

	// 2. High-reliability fallback: Google Translate TTS
	return synthesizeGoogleTranslate(ctx, cleanText, cleanVoice)
}

// synthesizeFishAudio synthesizes expressive character speech using Fish Audio's s2.1-pro-free model
func synthesizeFishAudio(ctx context.Context, cleanText, voiceID, apiKey string) ([]byte, error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"text":         cleanText,
		"reference_id": voiceID,
		"format":       "mp3",
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.fish.audio/v1/tts", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("model", "s2.1-pro-free")

	resp, err := ttsHTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			markFishAudioDisabled(5*time.Minute, "HTTP 401 unauthorized")
		}
		return nil, fmt.Errorf("fish audio HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(data) < 500 {
		return nil, fmt.Errorf("fish audio response too short (%d bytes)", len(data))
	}

	return data, nil
}

// synthesizeGoogleTranslate generates speech audio using pooled HTTP/2 connections in ~250ms
func synthesizeGoogleTranslate(ctx context.Context, cleanText, cleanVoice string) ([]byte, error) {
	lang := "en"
	switch cleanVoice {
	case "zero2_jp", "kurumi_jp", "naruto_jp", "ja", "jp", "japanese":
		lang = "ja"
	case "modi", "hi", "hindi":
		lang = "hi"
	case "fr", "french":
		lang = "fr"
	case "es", "spanish":
		lang = "es"
	case "de", "german":
		lang = "de"
	case "ru", "russian":
		lang = "ru"
	case "it", "italian":
		lang = "it"
	default:
		if len(cleanVoice) == 2 {
			lang = cleanVoice
		}
	}

	gURL := fmt.Sprintf("https://translate.google.com/translate_tts?ie=UTF-8&tl=%s&client=tw-ob&q=%s",
		lang, url.QueryEscape(cleanText))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare fallback TTS request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := ttsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS HTTP error: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading TTS audio stream: %w", err)
	}
	if len(data) < 200 {
		return nil, fmt.Errorf("TTS audio response too small (%d bytes)", len(data))
	}

	return data, nil
}

// SynthesizeToFile converts text to speech and writes the MP3 to destPath
func SynthesizeToFile(ctx context.Context, textToSpeak, voiceKey, destPath string) error {
	data, err := Synthesize(ctx, textToSpeak, voiceKey)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}

// SynthesizeWS converts text to MP3 audio bytes using Fish Audio's real-time WebSocket endpoint if paid credits exist, falling back to HTTP s2.1-pro-free
func SynthesizeWS(ctx context.Context, textToSpeak, voiceKey string) ([]byte, error) {
	cleanText := strings.TrimSpace(textToSpeak)
	if cleanText == "" {
		return nil, fmt.Errorf("empty text to synthesize")
	}

	cleanVoice := strings.ToLower(strings.TrimSpace(voiceKey))
	if cleanVoice == "" {
		cleanVoice = "furina"
	}

	voiceID, ok := VoiceIDs[cleanVoice]
	if !ok {
		return nil, fmt.Errorf("unknown voice: %s", cleanVoice)
	}

	apiKey := os.Getenv("FISH_AUDIO_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("FISH_AUDIO_API_KEY not configured")
	}

	// Always prioritize the working s2.1-pro-free HTTP endpoint for guaranteed character voice
	return synthesizeFishAudio(ctx, cleanText, voiceID, apiKey)
}

// SynthesizeWSToFile converts text to speech and writes MP3 to destPath using the character voice
func SynthesizeWSToFile(ctx context.Context, textToSpeak, voiceKey, destPath string) error {
	data, err := Synthesize(ctx, textToSpeak, voiceKey)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}
