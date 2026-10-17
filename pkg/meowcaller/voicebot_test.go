package meowcaller

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestPCMToWav(t *testing.T) {
	sampleRate := 16000
	numSamples := 1600 // 100ms
	pcm := make([]int16, numSamples)
	for i := range pcm {
		pcm[i] = int16(i * 10)
	}

	wav := pcmToWav(pcm, sampleRate)
	if len(wav) != 44+numSamples*2 {
		t.Fatalf("expected wav length %d, got %d", 44+numSamples*2, len(wav))
	}

	// Check RIFF header
	if string(wav[0:4]) != "RIFF" {
		t.Errorf("expected RIFF header, got %s", string(wav[0:4]))
	}
	if string(wav[8:12]) != "WAVE" {
		t.Errorf("expected WAVE header, got %s", string(wav[8:12]))
	}
	if string(wav[12:16]) != "fmt " {
		t.Errorf("expected fmt header, got %s", string(wav[12:16]))
	}
	if string(wav[36:40]) != "data" {
		t.Errorf("expected data header, got %s", string(wav[36:40]))
	}

	// Check sample rate
	sr := binary.LittleEndian.Uint32(wav[24:28])
	if int(sr) != sampleRate {
		t.Errorf("expected sample rate %d, got %d", sampleRate, sr)
	}

	// Check data size
	ds := binary.LittleEndian.Uint32(wav[40:44])
	if int(ds) != numSamples*2 {
		t.Errorf("expected data size %d, got %d", numSamples*2, ds)
	}
}

func TestVoiceBotLifecycleAndTelemetry(t *testing.T) {
	mgr := GlobalManager
	vb := NewVoiceBot(mgr)

	if vb.IsEnabled() {
		t.Errorf("expected VoiceBot to be disabled initially")
	}

	cfg := VoiceBotConfig{
		Enabled:     true,
		ServerURL:   "ws://127.0.0.1:9999", // non-existent server
		Voice:       "furina",
		GeminiModel: "gemini-3.5-flash-lite",
	}
	vb.UpdateConfig(cfg)

	if !vb.IsEnabled() {
		t.Errorf("expected VoiceBot to be enabled after UpdateConfig")
	}

	tel := vb.GetTelemetry()
	if !tel.Enabled {
		t.Errorf("expected telemetry Enabled to be true")
	}
	if tel.GeminiModel != "gemini-3.5-flash-lite" {
		t.Errorf("expected geminiModel to be gemini-3.5-flash-lite, got %s", tel.GeminiModel)
	}
	// Engine should report Groq Whisper STT
	if !strings.Contains(tel.Engine, "Groq") {
		t.Errorf("expected Groq STT engine, got %s", tel.Engine)
	}

	vb.Stop()
	if vb.state != StateIdle {
		t.Errorf("expected state to be idle after stop, got %s", vb.state)
	}
}

func TestHallucinationDetection(t *testing.T) {
	fakeExamples := []string{
		"you",
		"You",
		"YOU",
		"thank you",
		"Thank you.",
		"thank you very much",
		"thanks for watching!",
		"so",
		"yeah",
		"okay",
		"[silence]",
		"(music)",
		"subtitles by the amara.org community",
		"you you you",
		".",
		"??",
		"   ",
	}

	for _, text := range fakeExamples {
		if !isHallucinatedOrFakeTranscript(text) {
			t.Errorf("expected '%s' to be detected as fake/hallucination, but passed", text)
		}
	}

	realExamples := []string{
		"Hello, how are you doing today?",
		"What time is it in Tokyo?",
		"Can you play my favorite song?",
		"Who is the president of France?",
		"Tell me a joke",
	}

	for _, text := range realExamples {
		if isHallucinatedOrFakeTranscript(text) {
			t.Errorf("expected '%s' to be treated as valid speech, but was flagged as hallucination", text)
		}
	}
}

func TestSanitizeSpokenText(t *testing.T) {
	raw := "**Hello!** *smiles* I am happy to help you. [chuckles] How are you? #1"
	expected := "Hello! I am happy to help you. How are you? 1"
	got := sanitizeSpokenText(raw)
	if got != expected {
		t.Errorf("expected '%s', got '%s'", expected, got)
	}
}

func TestVoiceBotOnPlaybackFinished(t *testing.T) {
	mgr := GlobalManager
	vb := NewVoiceBot(mgr)

	vb.mu.Lock()
	vb.state = StateSpeaking
	vb.mu.Unlock()

	vb.OnPlaybackFinished()

	vb.mu.RLock()
	state := vb.state
	vb.mu.RUnlock()

	if state != StateListening {
		t.Errorf("expected state to transition to listening, got %s", state)
	}
}

func TestVoiceBotCallContextLifecycle(t *testing.T) {
	mgr := GlobalManager
	vb := NewVoiceBot(mgr)

	peer := "+1234567890"
	vb.StartCallContext(peer)

	if vb.GetCallPeer() != peer {
		t.Errorf("expected peer %s, got %s", peer, vb.GetCallPeer())
	}
	if len(vb.GetCallContext()) != 0 {
		t.Errorf("expected empty context on start, got %d messages", len(vb.GetCallContext()))
	}

	// Add turn 1
	vb.AppendCallMessage("user", "Hello there!")
	vb.AppendCallMessage("model", "Hello! How can I assist you?")

	if len(vb.GetCallContext()) != 2 {
		t.Fatalf("expected 2 messages in context, got %d", len(vb.GetCallContext()))
	}

	tel := vb.GetTelemetry()
	if tel.ContextTurns != 1 {
		t.Errorf("expected 1 turn in telemetry, got %d", tel.ContextTurns)
	}

	// Add turn 2
	vb.AppendCallMessage("user", "What is your name?")
	vb.AppendCallMessage("model", "I am your voice assistant.")

	if len(vb.GetCallContext()) != 4 {
		t.Fatalf("expected 4 messages in context, got %d", len(vb.GetCallContext()))
	}

	// Check multi-turn prompt construction
	currentInput := "Are you there?"
	contents := vb.buildGenerateContents(currentInput)
	if len(contents) != 5 {
		t.Fatalf("expected 5 contents (4 history + 1 current), got %d", len(contents))
	}
	if contents[0].Role != "user" || contents[0].Parts[0].Text != "Hello there!" {
		t.Errorf("unexpected content 0: %+v", contents[0])
	}
	if contents[1].Role != "model" || contents[1].Parts[0].Text != "Hello! How can I assist you?" {
		t.Errorf("unexpected content 1: %+v", contents[1])
	}
	if contents[4].Role != "user" || contents[4].Parts[0].Text != "Are you there?" {
		t.Errorf("unexpected content 4: %+v", contents[4])
	}

	// Flush call context when call ends
	vb.FlushCallContext("test call completed")

	if len(vb.GetCallContext()) != 0 {
		t.Errorf("expected 0 messages after flush, got %d", len(vb.GetCallContext()))
	}
	if vb.GetCallPeer() != "" {
		t.Errorf("expected empty peer after flush, got %s", vb.GetCallPeer())
	}
	telAfter := vb.GetTelemetry()
	if telAfter.ContextTurns != 0 {
		t.Errorf("expected 0 ContextTurns in telemetry after flush, got %d", telAfter.ContextTurns)
	}
	if vb.state != StateIdle {
		t.Errorf("expected state to be idle after flush, got %s", vb.state)
	}
}

func TestVoiceBotContextRollingWindow(t *testing.T) {
	mgr := GlobalManager
	vb := NewVoiceBot(mgr)
	vb.StartCallContext("+9999999999")

	// Append 20 messages (exceeding max 16 messages / 8 turns limit)
	for i := 1; i <= 10; i++ {
		vb.AppendCallMessage("user", "question")
		vb.AppendCallMessage("model", "answer")
	}

	ctx := vb.GetCallContext()
	if len(ctx) != 16 {
		t.Errorf("expected rolling window to cap at 16 messages, got %d", len(ctx))
	}

	vb.FlushCallContext("test finished")
	if len(vb.GetCallContext()) != 0 {
		t.Errorf("expected context flushed to 0, got %d", len(vb.GetCallContext()))
	}
}

func TestParseSongIntent(t *testing.T) {
	tests := []struct {
		input       string
		trigger     string
		wantAction  string
		wantQuery   string
	}{
		// Default trigger "play song"
		{input: "play song shape of you", trigger: "play song", wantAction: "play", wantQuery: "shape of you"},
		{input: "can you play song believer", trigger: "play song", wantAction: "play", wantQuery: "believer"},
		{input: "please play music bohemian rhapsody", trigger: "play song", wantAction: "play", wantQuery: "bohemian rhapsody"},
		{input: "play track despacito", trigger: "play song", wantAction: "play", wantQuery: "despacito"},
		{input: "hey bot play song faded", trigger: "play song", wantAction: "play", wantQuery: "faded"},
		
		// False trigger prevention (should NOT trigger song playback)
		{input: "I went to play outside", trigger: "play song", wantAction: "", wantQuery: ""},
		{input: "don't play with my feelings", trigger: "play song", wantAction: "", wantQuery: ""},
		{input: "we can play soccer tomorrow", trigger: "play song", wantAction: "", wantQuery: ""},
		{input: "play", trigger: "play song", wantAction: "", wantQuery: ""},
		{input: "how are you doing today?", trigger: "play song", wantAction: "", wantQuery: ""},

		// Stop music commands
		{input: "stop song", trigger: "play song", wantAction: "stop", wantQuery: ""},
		{input: "stop music", trigger: "play song", wantAction: "stop", wantQuery: ""},
		{input: "stop the music please", trigger: "play song", wantAction: "stop", wantQuery: ""},
		{input: "pause music", trigger: "play song", wantAction: "stop", wantQuery: ""},

		// Custom trigger phrase e.g. "dj play"
		{input: "dj play starboy", trigger: "dj play", wantAction: "play", wantQuery: "starboy"},
		{input: "can you dj play starboy", trigger: "dj play", wantAction: "play", wantQuery: "starboy"},
		{input: "play song starboy", trigger: "dj play", wantAction: "", wantQuery: ""}, // default trigger ignored when custom trigger is set
	}

	for _, tt := range tests {
		act, q := parseSongIntent(tt.input, tt.trigger)
		if act != tt.wantAction || q != tt.wantQuery {
			t.Errorf("parseSongIntent(%q, %q) = (%q, %q); want (%q, %q)",
				tt.input, tt.trigger, act, q, tt.wantAction, tt.wantQuery)
		}
	}
}

func TestCleanSongTitleForSpeech(t *testing.T) {
	raw := "Ed Sheeran - Shape of You (Official Music Video)"
	expected := "Ed Sheeran - Shape of You"
	got := cleanSongTitleForSpeech(raw)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}

	raw2 := "Alan Walker - Faded [Lyrics] [HD]"
	expected2 := "Alan Walker - Faded"
	got2 := cleanSongTitleForSpeech(raw2)
	if got2 != expected2 {
		t.Errorf("expected %q, got %q", expected2, got2)
	}
}

func BenchmarkFeedAudioFrame(b *testing.B) {
	mgr := &Manager{}
	vb := NewVoiceBot(mgr)
	vb.cfg.Enabled = true
	vb.state = StateListening

	// 20ms frame = 320 float32 samples at 16kHz
	silentFrame := make([]float32, 320)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vb.FeedAudioFrame(silentFrame)
	}
}



