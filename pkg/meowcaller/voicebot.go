package meowcaller

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"whatsapp-bot/pkg/gemini"
	"whatsapp-bot/pkg/tts"
)

// VoiceBotState represents the current real-time state of the voice agent
type VoiceBotState string

const (
	StateIdle         VoiceBotState = "idle"
	StateListening    VoiceBotState = "listening"
	StateTranscribing VoiceBotState = "transcribing"
	StateThinking     VoiceBotState = "thinking"
	StateSpeaking     VoiceBotState = "speaking"
	StateOffline      VoiceBotState = "offline"
)

// VoiceBotConfig holds configuration for the realtime voice chatbot
type VoiceBotConfig struct {
	Enabled            bool   `json:"enabled"`
	ServerURL          string `json:"serverUrl"`          // e.g. "ws://127.0.0.1:6006"
	Voice              string `json:"voice"`              // TTS voice, e.g. "elon_musk" or "en"
	GeminiModel        string `json:"geminiModel"`        // e.g. "gemini-3.5-flash-lite"
	SilenceThresholdMs int    `json:"silenceThresholdMs"` // Silence before LLM trigger (default: 700ms)
	SystemPrompt       string `json:"systemPrompt"`
	SongTriggerPhrase  string `json:"songTriggerPhrase"`  // Trigger phrase for in-call music (default: "play song")
}

// VoiceCallMessage represents a single conversational turn in the temporary call context
type VoiceCallMessage struct {
	Role string `json:"role"` // "user" or "model"
	Text string `json:"text"`
}

// VoiceBotTelemetry holds real-time telemetry exposed to dashboard
type VoiceBotTelemetry struct {
	Enabled           bool          `json:"enabled"`
	Connected         bool          `json:"connected"`
	Engine            string        `json:"engine"`
	State             VoiceBotState `json:"state"`
	ServerURL         string        `json:"serverUrl"`
	Voice             string        `json:"voice"`
	GeminiModel       string        `json:"geminiModel"`
	LastTranscript    string        `json:"lastTranscript"`
	LastResponse      string        `json:"lastResponse"`
	LastLatencyMs     int64         `json:"lastLatencyMs"`
	TotalTurns        int           `json:"totalTurns"`
	ContextTurns      int           `json:"contextTurns"`
	SongTriggerPhrase string        `json:"songTriggerPhrase"`
}

// sherpaResult models the JSON streaming response from sherpa-onnx-online-websocket-server
type sherpaResult struct {
	Text     string `json:"text"`
	Segment  int    `json:"segment"`
	Idx      int    `json:"idx"`
	Finished bool   `json:"finished"`
}

// VoiceBot manages ultra-low-latency bidirectional conversational voice over MeowCaller
type VoiceBot struct {
	manager *Manager

	mu             sync.RWMutex
	cfg            VoiceBotConfig
	state          VoiceBotState
	connected      bool
	lastTranscript string
	lastResponse   string
	lastLatencyMs  int64
	totalTurns     int

	// Call-scoped conversation context (temporarily stored during call, flushed on call end)
	contextMu   sync.RWMutex
	callContext []VoiceCallMessage
	callPeer    string

	// Audio accumulation for Sherpa-ONNX streaming (100ms chunks = 1600 samples at 16kHz)
	sampleBuffer []int16

	// Cloud ASR Voice Activity Detection (VAD) fallback
	vadSpeechBuffer []float32
	vadSpeaking     bool
	vadSilenceTimer *time.Timer
	vadPreRing      [8][]float32
	vadPreHead      int
	vadPreCount     int

	// Goroutine lifecycle
	cancelFn   context.CancelFunc
	audioInCh  chan []byte
	wsConn     *websocket.Conn
	activeTurn int64

	// Silence timer for Sherpa-ONNX streaming end-of-speech detection
	silenceTimer *time.Timer
	pendingText  string
	lastSegment  int
}

func NewVoiceBot(m *Manager) *VoiceBot {
	return &VoiceBot{
		manager: m,
		cfg: VoiceBotConfig{
			Enabled:            false,
			ServerURL:          "ws://127.0.0.1:6006",
			Voice:              "furina",
			GeminiModel:        "gemini-3.5-flash-lite",
			SilenceThresholdMs: 400,
			SongTriggerPhrase:  "play song",
			SystemPrompt: "You are a witty, helpful, ultra-concise real-time AI voice assistant on a live phone call. " +
				"Keep responses direct, natural, conversational, and under 2 short sentences. Do not use bullet points, markdown formatting, emojis, or lists.",
		},
		state: StateIdle,
	}
}

// GetTelemetry returns the current state for UI rendering
func (v *VoiceBot) GetTelemetry() VoiceBotTelemetry {
	v.mu.RLock()
	defer v.mu.RUnlock()

	engine := "Cloud ASR (Groq / Gemini)"
	if v.connected {
		engine = "Sherpa-ONNX (Local C++)"
	}

	v.contextMu.RLock()
	contextTurns := len(v.callContext) / 2
	if contextTurns == 0 && len(v.callContext) > 0 {
		contextTurns = 1
	}
	v.contextMu.RUnlock()

	return VoiceBotTelemetry{
		Enabled:           v.cfg.Enabled,
		Connected:         v.connected,
		Engine:            engine,
		State:             v.state,
		ServerURL:         v.cfg.ServerURL,
		Voice:             v.cfg.Voice,
		GeminiModel:       v.cfg.GeminiModel,
		LastTranscript:    v.lastTranscript,
		LastResponse:      v.lastResponse,
		LastLatencyMs:     v.lastLatencyMs,
		TotalTurns:        v.totalTurns,
		ContextTurns:      contextTurns,
		SongTriggerPhrase: v.cfg.SongTriggerPhrase,
	}
}

// UpdateConfig updates settings and reconnects if server URL changed
func (v *VoiceBot) UpdateConfig(cfg VoiceBotConfig) {
	v.mu.Lock()
	oldEnabled := v.cfg.Enabled
	oldURL := v.cfg.ServerURL
	if cfg.ServerURL == "" {
		cfg.ServerURL = "ws://127.0.0.1:6006"
	}
	if cfg.Voice == "" {
		cfg.Voice = "furina"
	}
	if cfg.GeminiModel == "" {
		cfg.GeminiModel = "gemini-3.5-flash-lite"
	}
	if cfg.SilenceThresholdMs <= 0 {
		cfg.SilenceThresholdMs = 400
	}
	if cfg.SongTriggerPhrase == "" {
		cfg.SongTriggerPhrase = "play song"
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = v.cfg.SystemPrompt
	}
	v.cfg = cfg
	v.mu.Unlock()

	if cfg.Enabled && (!oldEnabled || oldURL != cfg.ServerURL) {
		v.Start()
	} else if !cfg.Enabled && oldEnabled {
		v.Stop()
	}
}

// IsEnabled returns true if voice agent is active
func (v *VoiceBot) IsEnabled() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.cfg.Enabled
}

// Start initiates the voice agent (Sherpa-ONNX streaming + Cloud ASR fallback)
func (v *VoiceBot) Start() {
	v.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	v.mu.Lock()
	v.cancelFn = cancel
	v.audioInCh = make(chan []byte, 32)
	v.state = StateListening
	v.sampleBuffer = make([]int16, 0, 1600)
	v.vadSpeechBuffer = make([]float32, 0, 16000*3)
	v.resetVadPreBuffer()
	v.vadSpeaking = false
	v.pendingText = ""
	serverURL := v.cfg.ServerURL
	model := v.cfg.GeminiModel
	voice := v.cfg.Voice
	v.mu.Unlock()

	v.manager.log("SYSTEM", "[VoiceBot] VoiceBot ACTIVATED! Listening for caller speech (Model: %s, Sherpa-ONNX endpoint: %s)", model, serverURL)
	go v.ensureFillersCached(voice)
	go v.connectAndRun(ctx, serverURL)
}

func (v *VoiceBot) resetVadPreBuffer() {
	v.vadPreHead = 0
	v.vadPreCount = 0
}

// Stop cleanly closes WebSocket and background loops
func (v *VoiceBot) Stop() {
	v.mu.Lock()
	if v.cancelFn != nil {
		v.cancelFn()
		v.cancelFn = nil
	}
	if v.wsConn != nil {
		_ = v.wsConn.Close(websocket.StatusNormalClosure, "voice bot stopping")
		v.wsConn = nil
	}
	if v.silenceTimer != nil {
		v.silenceTimer.Stop()
		v.silenceTimer = nil
	}
	if v.vadSilenceTimer != nil {
		v.vadSilenceTimer.Stop()
		v.vadSilenceTimer = nil
	}
	v.connected = false
	v.state = StateIdle
	v.vadSpeaking = false
	v.vadSpeechBuffer = nil
	v.resetVadPreBuffer()
	v.mu.Unlock()

	v.FlushCallContext("voice bot stopped")
}

// OnPlaybackFinished is invoked by Manager when audio playback finishes to re-arm listening
func (v *VoiceBot) OnPlaybackFinished() {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Only transition back to listening if we were speaking
	if v.state == StateSpeaking {
		v.state = StateListening
		v.vadSpeaking = false
		v.vadSpeechBuffer = v.vadSpeechBuffer[:0]
		v.resetVadPreBuffer()
		v.manager.log("DEBUG", "[VoiceBot] Bot finished speaking, listening re-armed")
	}
}

// StartCallContext initializes a fresh conversation context for a new call
func (v *VoiceBot) StartCallContext(peer string) {
	v.contextMu.Lock()
	if v.callPeer == peer && len(v.callContext) > 0 {
		// Context already actively running for this peer
		v.contextMu.Unlock()
		return
	}
	turnsCount := len(v.callContext)
	v.callContext = make([]VoiceCallMessage, 0, 16)
	v.callPeer = peer
	v.contextMu.Unlock()

	v.mu.Lock()
	v.lastTranscript = ""
	v.lastResponse = ""
	v.lastLatencyMs = 0
	if v.state != StateSpeaking {
		v.state = StateListening
	}
	v.mu.Unlock()

	if turnsCount > 0 {
		v.manager.log("INFO", "[VoiceBot] 🧹 Flushed %d leftover turns and initialized fresh call context for %s", turnsCount, peer)
	} else {
		v.manager.log("INFO", "[VoiceBot] 📞 Initialized fresh temporary call context for %s", peer)
	}
}

// FlushCallContext clears all stored conversational context and resets in-flight voice state when a call ends
func (v *VoiceBot) FlushCallContext(reason string) {
	v.contextMu.Lock()
	turnsCount := len(v.callContext)
	v.callContext = nil
	peer := v.callPeer
	v.callPeer = ""
	v.contextMu.Unlock()

	// Invalidate any in-flight turn so background LLM or TTS goroutines stop immediately
	atomic.AddInt64(&v.activeTurn, 1)

	v.mu.Lock()
	if v.vadSilenceTimer != nil {
		v.vadSilenceTimer.Stop()
		v.vadSilenceTimer = nil
	}
	if v.silenceTimer != nil {
		v.silenceTimer.Stop()
		v.silenceTimer = nil
	}
	v.vadSpeechBuffer = v.vadSpeechBuffer[:0]
	v.vadSpeaking = false
	v.sampleBuffer = v.sampleBuffer[:0]
	v.pendingText = ""
	v.state = StateIdle
	v.mu.Unlock()

	v.manager.log("INFO", "[VoiceBot] 🧹 Flushed temporary conversation context (%d turns cleared) for %s (reason: %s)",
		turnsCount, peer, reason)
}

// AppendCallMessage appends a message to the temporary call context (max 16 messages = 8 turns rolling window)
func (v *VoiceBot) AppendCallMessage(role, text string) {
	v.appendCallMessage(role, text)
}

func (v *VoiceBot) appendCallMessage(role, text string) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return
	}
	v.contextMu.Lock()
	defer v.contextMu.Unlock()

	v.callContext = append(v.callContext, VoiceCallMessage{
		Role: role,
		Text: clean,
	})

	// Keep rolling window of last 16 messages to keep latency ultra-low
	const maxContextMessages = 16
	if len(v.callContext) > maxContextMessages {
		v.callContext = v.callContext[len(v.callContext)-maxContextMessages:]
	}
}

// GetCallContext returns a copy of the current temporary conversation context
func (v *VoiceBot) GetCallContext() []VoiceCallMessage {
	v.contextMu.RLock()
	defer v.contextMu.RUnlock()

	if len(v.callContext) == 0 {
		return nil
	}
	res := make([]VoiceCallMessage, len(v.callContext))
	copy(res, v.callContext)
	return res
}

// GetCallPeer returns the peer currently associated with this call context
func (v *VoiceBot) GetCallPeer() string {
	v.contextMu.RLock()
	defer v.contextMu.RUnlock()
	return v.callPeer
}

// buildGenerateContents prepares the Gemini/OpenAI multi-turn contents slice including previous context + current input
func (v *VoiceBot) buildGenerateContents(currentInput string) []gemini.Content {
	v.contextMu.RLock()
	defer v.contextMu.RUnlock()

	contents := make([]gemini.Content, 0, len(v.callContext)+1)
	for _, msg := range v.callContext {
		role := msg.Role
		if role == "" {
			role = "user"
		}
		contents = append(contents, gemini.Content{
			Role:  role,
			Parts: []gemini.Part{{Text: msg.Text}},
		})
	}

	contents = append(contents, gemini.Content{
		Role:  "user",
		Parts: []gemini.Part{{Text: currentInput}},
	})
	return contents
}

var thinkingFillers = []string{
	"Hmm...",
	"Okay...",
	"Let's see...",
	"Right...",
	"Uh-huh...",
}

// ensureFillersCached synthesizes thinking filler audio clips for the given voice if not already cached
func (v *VoiceBot) ensureFillersCached(voice string) {
	if voice == "" {
		voice = "furina"
	}
	dir := filepath.Join("data", "media", "voicebot", "fillers", voice)
	_ = os.MkdirAll(dir, 0755)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	for i, phrase := range thinkingFillers {
		path := filepath.Join(dir, fmt.Sprintf("filler_%d.mp3", i))
		if info, err := os.Stat(path); err == nil && info.Size() > 200 {
			continue // already cached
		}
		_ = tts.SynthesizeWSToFile(ctx, phrase, voice, path)
	}
}

// playThinkingFiller selects a random pre-cached filler noise and plays it into the active call
func (v *VoiceBot) playThinkingFiller(voice string) {
	if voice == "" {
		voice = "furina"
	}
	dir := filepath.Join("data", "media", "voicebot", "fillers", voice)

	var available []string
	for i := range thinkingFillers {
		path := filepath.Join(dir, fmt.Sprintf("filler_%d.mp3", i))
		if info, err := os.Stat(path); err == nil && info.Size() > 200 {
			available = append(available, path)
		}
	}

	if len(available) == 0 {
		return
	}

	idx := int(time.Now().UnixNano() % int64(len(available)))
	chosen := available[idx]
	v.manager.log("INFO", "[VoiceBot] AI is thinking... playing filler noise into call (%s)", filepath.Base(chosen))
	_ = v.manager.PlayVoiceFiller(chosen)
}

// isHallucinatedOrFakeTranscript detects Whisper / ASR hallucinations on background noise or silence
func isHallucinatedOrFakeTranscript(text string) bool {
	clean := strings.ToLower(strings.TrimSpace(text))
	if clean == "" {
		return true
	}

	// Remove common punctuation and enclosing brackets/parentheses
	clean = strings.Trim(clean, ".?!,;:\"'~`-_—()[]{}")
	clean = strings.TrimSpace(clean)
	if len(clean) <= 1 {
		return true
	}

	// Known Whisper / ASR silence hallucinations
	fakePhrases := []string{
		"you",
		"thank you",
		"thank you.",
		"thank you very much",
		"thank you so much",
		"thanks",
		"thanks for watching",
		"thanks for watching!",
		"bye",
		"goodbye",
		"see you next time",
		"so",
		"yeah",
		"yes",
		"yep",
		"okay",
		"ok",
		"subtitles by",
		"subtitles by the amara.org community",
		"amara.org",
		"transcription by",
		"silence",
		"music",
		"applause",
		"laughter",
		"cough",
		"snort",
		"blank_audio",
		"watching",
		"the end",
		"please subscribe",
		"subscribe to my channel",
		"subscribe to our channel",
		"like and subscribe",
		"mbc 뉴스",
	}

	for _, fake := range fakePhrases {
		if clean == fake {
			return true
		}
	}

	// Repetitive single syllables like "you you you" or "thank you thank you"
	tokens := strings.Fields(clean)
	if len(tokens) >= 2 {
		allSame := true
		for i := 1; i < len(tokens); i++ {
			if tokens[i] != tokens[0] {
				allSame = false
				break
			}
		}
		if allSame && (tokens[0] == "you" || tokens[0] == "thank" || tokens[0] == "thanks" || tokens[0] == "so") {
			return true
		}
	}

	return false
}

// sanitizeSpokenText removes markdown symbols, stage directions, and formatting that sounds awkward in TTS
func sanitizeSpokenText(s string) string {
	// Strip actions and stage directions like *smiles*, *sighs*, [laughs], (chuckles), <break>
	reAction := regexp.MustCompile(`\*.*?\*|\[.*?\]|\(.*?\)|<.*?>`)
	s = reAction.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

// FeedAudioFrame ingests decoded 16kHz mono float32 frames from MeowCaller.
// If Sherpa-ONNX is connected, streams 100ms packets to local C++ engine.
// If Sherpa-ONNX is not connected, uses Cloud VAD + Groq Whisper/Gemini fallback.
func (v *VoiceBot) FeedAudioFrame(frame []float32) {
	if len(frame) == 0 || !v.IsEnabled() {
		return
	}

	v.mu.Lock()

	// Calculate RMS energy for voice activity and barge-in detection with unrolled multiply-accumulate
	var sumSquares float64
	n := len(frame)
	i := 0
	for ; i+4 <= n; i += 4 {
		s0, s1, s2, s3 := frame[i], frame[i+1], frame[i+2], frame[i+3]
		sumSquares += float64(s0*s0 + s1*s1 + s2*s2 + s3*s3)
	}
	for ; i < n; i++ {
		s := frame[i]
		sumSquares += float64(s * s)
	}
	rms := math.Sqrt(sumSquares / float64(n))

	// Barge-in: if user starts speaking while AI is currently playing speech, interrupt bot playback immediately
	if rms > 0.06 && v.state == StateSpeaking {
		v.manager.log("INFO", "[VoiceBot] Barge-in detected (caller spoke, RMS: %.2f) - interrupting bot playback", rms)
		v.state = StateListening
		v.mu.Unlock()
		v.manager.InterruptAudio()
		v.mu.Lock()
	}

	// While AI is speaking, discard incoming audio frames to prevent room acoustic echo feedback
	if v.state == StateSpeaking {
		v.mu.Unlock()
		return
	}

	// -------------------------------------------------------------
	// Path 1: Local Sherpa-ONNX C++ Streaming Engine (if connected)
	// -------------------------------------------------------------
	if v.connected && v.wsConn != nil {
		var readyPackets [][]byte
		for _, s := range frame {
			val := s * 32768.0
			if val > 32767 {
				val = 32767
			} else if val < -32768 {
				val = -32768
			}
			v.sampleBuffer = append(v.sampleBuffer, int16(val))

			if len(v.sampleBuffer) >= 1600 {
				packet := make([]byte, 3200)
				for j := 0; j < 1600; j++ {
					binary.LittleEndian.PutUint16(packet[j*2:], uint16(v.sampleBuffer[j]))
				}
				v.sampleBuffer = v.sampleBuffer[:0]
				readyPackets = append(readyPackets, packet)
			}
		}
		audioCh := v.audioInCh
		v.mu.Unlock()

		for _, pkt := range readyPackets {
			select {
			case audioCh <- pkt:
			default:
			}
		}
		return
	}

	// -------------------------------------------------------------
	// Path 2: Cloud Fallback VAD (Groq Whisper / Gemini)
	// -------------------------------------------------------------
	speechTrigger := 0.020
	speechContinue := 0.013
	isSpeech := false
	if v.vadSpeaking {
		isSpeech = rms > speechContinue
	} else {
		isSpeech = rms > speechTrigger
	}

	if isSpeech {
		if !v.vadSpeaking {
			v.vadSpeaking = true
			if v.state != StateThinking && v.state != StateSpeaking {
				v.state = StateListening
			}
			// Prepend rolling pre-buffer in chronological order so beginning consonants aren't chopped
			for j := 0; j < v.vadPreCount; j++ {
				slot := (v.vadPreHead + j) % 8
				v.vadSpeechBuffer = append(v.vadSpeechBuffer, v.vadPreRing[slot]...)
			}
			v.resetVadPreBuffer()
		}

		v.vadSpeechBuffer = append(v.vadSpeechBuffer, frame...)

		if v.vadSilenceTimer != nil {
			v.vadSilenceTimer.Stop()
			v.vadSilenceTimer = nil
		}

		// Cap max single utterance to ~12 seconds
		if len(v.vadSpeechBuffer) > 16000*12 {
			speechCopy := make([]float32, len(v.vadSpeechBuffer))
			copy(speechCopy, v.vadSpeechBuffer)
			v.vadSpeechBuffer = v.vadSpeechBuffer[:0]
			v.vadSpeaking = false
			turnID := atomic.AddInt64(&v.activeTurn, 1)
			v.state = StateTranscribing
			v.mu.Unlock()

			go v.processCloudSpeech(turnID, speechCopy)
			return
		}
	} else {
		// Silence or low background noise
		if v.vadSpeaking {
			v.vadSpeechBuffer = append(v.vadSpeechBuffer, frame...)

			if v.vadSilenceTimer == nil {
				silenceMs := time.Duration(v.cfg.SilenceThresholdMs) * time.Millisecond
				if silenceMs < 300*time.Millisecond {
					silenceMs = 400 * time.Millisecond
				}

				turnID := atomic.AddInt64(&v.activeTurn, 1)
				v.vadSilenceTimer = time.AfterFunc(silenceMs, func() {
					v.mu.Lock()
					if !v.vadSpeaking || len(v.vadSpeechBuffer) < 8000 { // at least 500ms speech
						v.vadSpeechBuffer = v.vadSpeechBuffer[:0]
						v.vadSpeaking = false
						v.vadSilenceTimer = nil
						v.mu.Unlock()
						return
					}

					speechCopy := make([]float32, len(v.vadSpeechBuffer))
					copy(speechCopy, v.vadSpeechBuffer)
					v.vadSpeechBuffer = v.vadSpeechBuffer[:0]
					v.vadSpeaking = false
					v.vadSilenceTimer = nil
					v.state = StateTranscribing
					v.mu.Unlock()

					v.processCloudSpeech(turnID, speechCopy)
				})
			}
		} else {
			// Not speaking: keep rolling pre-buffer (~160ms) using circular ring buffer with zero allocations
			idx := (v.vadPreHead + v.vadPreCount) % 8
			if v.vadPreCount < 8 {
				v.vadPreCount++
			} else {
				v.vadPreHead = (v.vadPreHead + 1) % 8
			}
			if cap(v.vadPreRing[idx]) < len(frame) {
				v.vadPreRing[idx] = make([]float32, len(frame))
			} else {
				v.vadPreRing[idx] = v.vadPreRing[idx][:len(frame)]
			}
			copy(v.vadPreRing[idx], frame)
		}
	}

	v.mu.Unlock()
}

// processCloudSpeech converts captured float32 PCM to WAV and transcribes via Groq Whisper / Gemini
func (v *VoiceBot) processCloudSpeech(turnID int64, samples []float32) {
	if turnID != atomic.LoadInt64(&v.activeTurn) {
		return
	}

	// Calculate average RMS and peak amplitude across speech sample to reject acoustic background noise
	var sumSquares float64
	var maxPeak float32
	for _, s := range samples {
		sumSquares += float64(s * s)
		abs := s
		if abs < 0 {
			abs = -abs
		}
		if abs > maxPeak {
			maxPeak = abs
		}
	}
	avgRMS := math.Sqrt(sumSquares / float64(len(samples)))

	// If utterance was too quiet (background hiss/mic noise) or lacked clear speech peaks, drop it
	if avgRMS < 0.016 || maxPeak < 0.035 || len(samples) < 8000 { // at least 500ms and audible speech
		v.manager.log("DEBUG", "[VoiceBot] Audio discarded as silence/line noise (avgRMS: %.4f, peak: %.2f, dur: %.2fs)",
			avgRMS, maxPeak, float64(len(samples))/16000.0)
		v.mu.Lock()
		if v.state == StateTranscribing {
			v.state = StateListening
		}
		v.mu.Unlock()
		return
	}

	// Convert float32 -> int16
	pcm := make([]int16, len(samples))
	for i, s := range samples {
		val := s * 32768.0
		if val > 32767 {
			val = 32767
		} else if val < -32768 {
			val = -32768
		}
		pcm[i] = int16(val)
	}

	wavBytes := pcmToWav(pcm, 16000)
	durationSec := float64(len(samples)) / 16000.0
	v.manager.log("DEBUG", "[VoiceBot] Cloud ASR: Transcribing %.2fs utterance (%d bytes WAV)...", durationSec, len(wavBytes))

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	transcript, err := v.transcribeAudioCloud(ctx, wavBytes)
	if err != nil || strings.TrimSpace(transcript) == "" {
		v.manager.log("WARN", "[VoiceBot] Cloud ASR transcription empty or failed: %v", err)
		v.mu.Lock()
		if v.state == StateTranscribing {
			v.state = StateListening
		}
		v.mu.Unlock()
		return
	}

	cleanTranscript := strings.TrimSpace(transcript)
	if isHallucinatedOrFakeTranscript(cleanTranscript) {
		v.manager.log("INFO", "[VoiceBot] Ignored hallucinated silence artifact: \"%s\"", cleanTranscript)
		v.mu.Lock()
		if v.state == StateTranscribing {
			v.state = StateListening
		}
		v.mu.Unlock()
		return
	}
	v.mu.Lock()
	if turnID != atomic.LoadInt64(&v.activeTurn) {
		v.mu.Unlock()
		return
	}
	v.lastTranscript = cleanTranscript
	v.pendingText = cleanTranscript
	v.mu.Unlock()

	v.triggerAIResponse(turnID, cleanTranscript)
}

// Persistent HTTP/2 keep-alive client for Groq Whisper transcription (~250-350ms)
var groqASRClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression: false,
	},
	Timeout: 5 * time.Second,
}

// transcribeAudioCloud tries Groq Whisper first, then falls back to Gemini multimodal ASR
func (v *VoiceBot) transcribeAudioCloud(ctx context.Context, wavData []byte) (string, error) {
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey == "" && gemini.GlobalClient != nil {
		groqKey = gemini.GlobalClient.GetGroqKey()
	}
	if groqKey != "" {
		text, err := transcribeGroqWhisper(ctx, groqKey, wavData)
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
		if err != nil {
			v.manager.log("WARN", "[VoiceBot] Groq Whisper failed: %v, falling back to Gemini", err)
		}
	}

	if gemini.GlobalClient != nil {
		text, err := gemini.GlobalClient.SolveProblemForChat(ctx, "voicebot-live-call",
			"Transcribe verbatim the spoken words in this live phone call audio. Reply with strictly the transcribed text, without any added explanation or quotes.",
			"", "audio/wav", wavData)
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
		return "", fmt.Errorf("gemini ASR failed: %w", err)
	}

	return "", fmt.Errorf("no ASR service available (GROQ_API_KEY and GEMINI_API_KEY unconfigured)")
}

// transcribeGroqWhisper calls Groq's whisper-large-v3-turbo API endpoint using pooled connections
func transcribeGroqWhisper(ctx context.Context, apiKey string, wavData []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "speech.wav")
	if err != nil {
		return "", err
	}
	if _, err := part.Write(wavData); err != nil {
		return "", err
	}

	_ = writer.WriteField("model", "whisper-large-v3-turbo")
	_ = writer.WriteField("response_format", "json")
	_ = writer.WriteField("temperature", "0.0")
	_ = writer.WriteField("prompt", "Live telephone conversation audio.")
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.groq.com/openai/v1/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := groqASRClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Text, nil
}

// pcmToWav builds a standard 16-bit 16kHz mono RIFF WAV container in memory
func pcmToWav(pcm []int16, sampleRate int) []byte {
	numSamples := len(pcm)
	byteRate := sampleRate * 1 * 2 // mono, 16-bit
	dataSize := numSamples * 2
	fileSize := 36 + dataSize

	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // Subchunk1Size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // AudioFormat (PCM)
	binary.LittleEndian.PutUint16(buf[22:24], 1)  // Channels (1)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], 2)  // BlockAlign
	binary.LittleEndian.PutUint16(buf[34:36], 16) // BitsPerSample
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	for i, s := range pcm {
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(s))
	}
	return buf
}

// ConnectWebSocket tests and connects to a WebSocket server URL (supporting both ws:// and wss://)
func (v *VoiceBot) ConnectWebSocket(serverURL string) error {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		v.mu.RLock()
		serverURL = v.cfg.ServerURL
		v.mu.RUnlock()
	}

	dialOpts := &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 4 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, serverURL, dialOpts)
	if err != nil {
		v.mu.Lock()
		v.connected = false
		v.mu.Unlock()
		return fmt.Errorf("dial error (%s): %w", serverURL, err)
	}

	v.mu.Lock()
	v.cfg.ServerURL = serverURL
	v.connected = true
	if v.wsConn != nil {
		_ = v.wsConn.Close(websocket.StatusNormalClosure, "reconnecting")
	}
	v.wsConn = conn
	if v.state != StateSpeaking && v.state != StateThinking && v.state != StateTranscribing {
		v.state = StateListening
	}
	v.mu.Unlock()

	v.manager.log("SYSTEM", "[VoiceBot] Successfully connected to WebSocket server at %s!", serverURL)
	return nil
}

// connectAndRun handles connection to the local Sherpa-ONNX streaming server
func (v *VoiceBot) connectAndRun(ctx context.Context, serverURL string) {
	dialOpts := &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 4 * time.Second,
		},
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		connCtx, connCancel := context.WithTimeout(ctx, 3*time.Second)
		conn, _, err := websocket.Dial(connCtx, serverURL, dialOpts)
		connCancel()

		if err != nil {
			v.mu.Lock()
			v.connected = false
			// When Sherpa-ONNX is offline, VoiceBot uses Cloud ASR fallback
			if v.state != StateSpeaking && v.state != StateThinking && v.state != StateTranscribing {
				v.state = StateListening
			}
			v.mu.Unlock()

			select {
			case <-ctx.Done():
				return
			case <-time.After(8 * time.Second):
				continue
			}
		}

		v.mu.Lock()
		v.wsConn = conn
		v.connected = true
		v.state = StateListening
		v.mu.Unlock()
		v.manager.log("SYSTEM", "[VoiceBot] Connected to sherpa-onnx streaming engine! Realtime C++ Voice Chatbot is READY.")

		// Run sender and receiver concurrently
		errGroupCh := make(chan error, 2)
		go func() {
			errGroupCh <- v.senderLoop(ctx, conn)
		}()
		go func() {
			errGroupCh <- v.receiverLoop(ctx, conn)
		}()

		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "shutdown")
			return
		case err := <-errGroupCh:
			v.manager.log("WARN", "[VoiceBot] sherpa-onnx connection terminated (%v). Fallback to Cloud ASR...", err)
			_ = conn.Close(websocket.StatusAbnormalClosure, "stream error")
			v.mu.Lock()
			v.connected = false
			if v.state != StateSpeaking && v.state != StateThinking && v.state != StateTranscribing {
				v.state = StateListening
			}
			v.wsConn = nil
			v.mu.Unlock()

			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (v *VoiceBot) senderLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case pcmChunk, ok := <-v.audioInCh:
			if !ok {
				return nil
			}
			writeCtx, writeCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			err := conn.Write(writeCtx, websocket.MessageBinary, pcmChunk)
			writeCancel()
			if err != nil {
				return fmt.Errorf("audio send error: %w", err)
			}
		}
	}
}

func (v *VoiceBot) receiverLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msgType, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
		if msgType != websocket.MessageText {
			continue
		}

		var res sherpaResult
		if err := json.Unmarshal(data, &res); err != nil {
			continue
		}

		text := strings.TrimSpace(res.Text)
		if text == "" {
			continue
		}

		v.handleRecognitionUpdate(text, res.Segment, res.Finished)
	}
}

func (v *VoiceBot) handleRecognitionUpdate(text string, segment int, finished bool) {
	text = strings.TrimSpace(text)
	if text == "" || isHallucinatedOrFakeTranscript(text) {
		return
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Update live transcription text
	v.lastTranscript = text
	v.pendingText = text
	if v.state != StateThinking && v.state != StateSpeaking {
		v.state = StateTranscribing
	}

	threshold := time.Duration(v.cfg.SilenceThresholdMs) * time.Millisecond
	if threshold < 300*time.Millisecond {
		threshold = 400 * time.Millisecond
	}

	if v.silenceTimer != nil {
		v.silenceTimer.Stop()
	}

	currentTurn := atomic.AddInt64(&v.activeTurn, 1)

	// Trigger response immediately if segment finished
	if finished || (segment > v.lastSegment && v.lastSegment != 0) {
		v.lastSegment = segment
		go v.triggerAIResponse(currentTurn, text)
		return
	}
	v.lastSegment = segment

	v.silenceTimer = time.AfterFunc(threshold, func() {
		v.mu.Lock()
		finalText := v.pendingText
		v.pendingText = ""
		v.mu.Unlock()

		if strings.TrimSpace(finalText) != "" && !isHallucinatedOrFakeTranscript(finalText) {
			v.triggerAIResponse(currentTurn, finalText)
		}
	})
}

// extractNextSentence checks if the accumulated stream text contains a complete sentence or natural phrase.
// Returns the sentence (trimmed) and the remaining buffer. If incomplete, returns ("", original).
func extractNextSentence(buf string, isFirst bool) (string, string) {
	trimmed := strings.TrimSpace(buf)
	if trimmed == "" {
		return "", ""
	}

	// Delimiters for terminal sentences
	terminalDelims := []string{". ", "! ", "? ", ".\n", "!\n", "?\n", "\n\n"}
	minIdx := -1
	delimLen := 0

	for _, d := range terminalDelims {
		if idx := strings.Index(buf, d); idx != -1 {
			if minIdx == -1 || idx < minIdx {
				minIdx = idx
				delimLen = len(d)
			}
		}
	}

	// For first sentence: if buffer has >= 6 words and a natural clause boundary (e.g. ", "),
	// dispatch early to achieve ultra-fast conversational time-to-first-voice (<1.2s)!
	if isFirst && minIdx == -1 {
		words := strings.Fields(buf)
		if len(words) >= 6 {
			clauseDelims := []string{", ", "; ", " - ", ": "}
			for _, d := range clauseDelims {
				if idx := strings.Index(buf, d); idx != -1 && idx > 12 {
					if minIdx == -1 || idx < minIdx {
						minIdx = idx
						delimLen = len(d)
					}
				}
			}
		}
	}

	if minIdx != -1 {
		sentence := strings.TrimSpace(buf[:minIdx+delimLen])
		remaining := strings.TrimLeft(buf[minIdx+delimLen:], " \t\r\n")
		return sentence, remaining
	}

	return "", buf
}

// parseSongIntent checks if user input is an intentional song playback request or stop request
func parseSongIntent(input string, customTrigger string) (action string, query string) {
	clean := strings.ToLower(strings.TrimSpace(input))
	clean = strings.Trim(clean, ".?!,;:\"'~`-_—()[]{}")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "", ""
	}

	// 1. Check for stop song / stop music commands
	stopPhrases := []string{
		"stop song", "stop the song", "stop music", "stop the music",
		"stop playing", "stop the track", "pause music", "pause song",
	}
	for _, sp := range stopPhrases {
		if clean == sp || strings.HasPrefix(clean, sp+" ") || strings.HasSuffix(clean, " "+sp) {
			return "stop", ""
		}
	}

	// 2. Determine trigger phrase
	trigger := strings.ToLower(strings.TrimSpace(customTrigger))
	if trigger == "" {
		trigger = "play song"
	}

	extract := func(prefix string) (string, bool) {
		if strings.HasPrefix(clean, prefix) {
			q := strings.TrimSpace(clean[len(prefix):])
			q = strings.TrimPrefix(q, "called ")
			q = strings.TrimPrefix(q, "named ")
			q = strings.TrimPrefix(q, ": ")
			q = strings.Trim(q, " \"'.,?!")
			if len(q) >= 2 {
				return q, true
			}
		}
		return "", false
	}

	patterns := []string{
		trigger + " ",
		"please " + trigger + " ",
		"can you " + trigger + " ",
		"could you " + trigger + " ",
		"hey bot " + trigger + " ",
		"bot " + trigger + " ",
	}

	if trigger == "play song" {
		patterns = append(patterns,
			"play music ", "please play music ", "can you play music ", "could you play music ",
			"play track ", "please play track ", "can you play track ",
		)
	}

	for _, p := range patterns {
		if q, ok := extract(p); ok {
			return "play", q
		}
	}

	return "", ""
}

// cleanSongQueryForSpeech formats user query so it sounds natural when spoken by TTS
func cleanSongQueryForSpeech(q string) string {
	q = strings.TrimSpace(q)
	q = strings.Trim(q, " \"'.,?!")
	return q
}

// cleanSongTitleForSpeech removes YouTube video bloat like "(Official Video)", "[HD]", "4K", etc.
func cleanSongTitleForSpeech(title string) string {
	re := regexp.MustCompile(`(?i)\s*[\(\[](official\s*(music\s*)?video|audio|lyrics|hd|4k|hq|visualizer|remastered|explicit)[\)\]]`)
	cleaned := re.ReplaceAllString(title, "")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) > 80 {
		cleaned = cleaned[:80]
	}
	return cleaned
}

// handleSongRequest manages song playback during a call with real-time spoken TTS updates
func (v *VoiceBot) handleSongRequest(turnID int64, songQuery string) {
	v.mu.Lock()
	if turnID != atomic.LoadInt64(&v.activeTurn) {
		v.mu.Unlock()
		return
	}
	voice := v.cfg.Voice
	if voice == "" {
		voice = "furina"
	}
	v.state = StateSpeaking
	v.mu.Unlock()

	startTime := time.Now()
	v.manager.log("INFO", "[VoiceBot] 🎵 Song request received: %q. Starting download with real-time voice updates...", songQuery)

	tmpDir := filepath.Join("data", "media", "voicebot")
	_ = os.MkdirAll(tmpDir, 0755)

	// Step 1: Speak immediate confirmation ("Searching and downloading...")
	ackText := fmt.Sprintf("Searching and downloading %s. One moment please!", cleanSongQueryForSpeech(songQuery))
	ackPath := filepath.Join(tmpDir, fmt.Sprintf("ack_song_%d.mp3", time.Now().UnixNano()))

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := tts.SynthesizeWSToFile(ctx, ackText, voice, ackPath); err == nil {
		if playErr := v.manager.PlayVoiceResponse(ackPath); playErr != nil {
			v.manager.log("WARN", "[VoiceBot] Failed to play ack voice: %v", playErr)
		}
		v.mu.Lock()
		v.lastResponse = ackText
		v.mu.Unlock()
	} else {
		v.manager.log("WARN", "[VoiceBot] Failed to synthesize ack TTS: %v", err)
	}

	// Channel for download result
	type dlResult struct {
		track *Track
		err   error
	}
	resCh := make(chan dlResult, 1)

	go func() {
		track, err := v.manager.DownloadAudio(ctx, songQuery)
		resCh <- dlResult{track: track, err: err}
	}()

	// Step 2: Intermediate progress update if download takes > 6s
	progressTicker := time.NewTimer(6 * time.Second)
	defer progressTicker.Stop()

	var res dlResult
	select {
	case <-progressTicker.C:
		// Download is still running: give caller reassurance
		v.mu.RLock()
		isStillActive := turnID == atomic.LoadInt64(&v.activeTurn)
		v.mu.RUnlock()
		if isStillActive {
			progText := fmt.Sprintf("Still downloading %s, almost ready...", cleanSongQueryForSpeech(songQuery))
			progPath := filepath.Join(tmpDir, fmt.Sprintf("prog_song_%d.mp3", time.Now().UnixNano()))
			if err := tts.SynthesizeWSToFile(ctx, progText, voice, progPath); err == nil {
				_ = v.manager.PlayVoiceResponse(progPath)
			}
		}

		select {
		case res = <-resCh:
		case <-ctx.Done():
			res = dlResult{err: ctx.Err()}
		}

	case res = <-resCh:
		// Completed before 6s
	case <-ctx.Done():
		res = dlResult{err: ctx.Err()}
	}

	// Check if turn was superseded (e.g. call ended or hangup)
	if turnID != atomic.LoadInt64(&v.activeTurn) {
		return
	}

	// Step 3: Handle outcome
	if res.err != nil || res.track == nil {
		v.manager.log("ERROR", "[VoiceBot] Song download failed for %q: %v", songQuery, res.err)
		errText := "Sorry, I couldn't download that song. Let's keep chatting!"
		errPath := filepath.Join(tmpDir, fmt.Sprintf("err_song_%d.mp3", time.Now().UnixNano()))
		if err := tts.SynthesizeWSToFile(ctx, errText, voice, errPath); err == nil {
			_ = v.manager.PlayVoiceResponse(errPath)
		}
		v.mu.Lock()
		v.lastResponse = errText
		v.state = StateListening
		v.mu.Unlock()
		return
	}

	track := res.track
	durMs := time.Since(startTime).Milliseconds()
	v.manager.log("SUCCESS", "[VoiceBot] 🎶 Song ready in %dms: %q. Announcing and streaming into call...", durMs, track.Title)

	cleanTitle := cleanSongTitleForSpeech(track.Title)
	annText := fmt.Sprintf("Now playing: %s. Enjoy!", cleanTitle)
	annPath := filepath.Join(tmpDir, fmt.Sprintf("ann_song_%d.mp3", time.Now().UnixNano()))

	v.mu.Lock()
	v.lastResponse = annText
	v.mu.Unlock()

	var playedWithAnnouncement bool
	if err := tts.SynthesizeWSToFile(ctx, annText, voice, annPath); err == nil {
		playErr := v.manager.PlayVoiceResponseWithCallback(annPath, func() {
			v.mu.Lock()
			v.state = StateListening // listening remains enabled so caller can speak or say "stop music"
			v.mu.Unlock()
			if err := v.manager.PlayTrack(track); err != nil {
				v.manager.log("ERROR", "[VoiceBot] Failed to stream track %q: %v", track.Title, err)
			}
		})
		if playErr == nil {
			playedWithAnnouncement = true
		} else {
			v.manager.log("WARN", "[VoiceBot] Failed to queue song announcement voice (%v), streaming song directly", playErr)
		}
	} else {
		v.manager.log("WARN", "[VoiceBot] Failed to synthesize song announcement TTS (%v), streaming song directly", err)
	}

	if !playedWithAnnouncement {
		// Fallback: start track immediately if TTS synthesis/queue failed
		if err := v.manager.PlayTrack(track); err != nil {
			v.manager.log("ERROR", "[VoiceBot] Failed to stream track %q directly: %v", track.Title, err)
		}
		v.mu.Lock()
		v.state = StateListening
		v.mu.Unlock()
	}

	// Record the turn in conversation context
	v.appendCallMessage("user", fmt.Sprintf("Play song %s", songQuery))
	v.appendCallMessage("model", annText)
}

// handleStopSongRequest stops the active music track and announces confirmation
func (v *VoiceBot) handleStopSongRequest(turnID int64) {
	v.manager.StopMusic()
	voice := v.cfg.Voice
	if voice == "" {
		voice = "furina"
	}
	v.mu.Lock()
	v.state = StateSpeaking
	v.mu.Unlock()

	stopText := "Stopped the music. What else can I help you with?"
	tmpDir := filepath.Join("data", "media", "voicebot")
	_ = os.MkdirAll(tmpDir, 0755)
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("stop_song_%d.mp3", time.Now().UnixNano()))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tts.SynthesizeWSToFile(ctx, stopText, voice, tmpPath); err == nil {
		_ = v.manager.PlayVoiceResponse(tmpPath)
	}
	v.appendCallMessage("user", "Stop music")
	v.appendCallMessage("model", stopText)
}

func (v *VoiceBot) triggerAIResponse(turnID int64, userInput string) {
	cleanInput := strings.TrimSpace(userInput)
	if cleanInput == "" || len(cleanInput) < 2 || isHallucinatedOrFakeTranscript(cleanInput) {
		v.mu.Lock()
		if v.state != StateSpeaking {
			v.state = StateListening
		}
		v.mu.Unlock()
		return
	}

	v.mu.RLock()
	triggerPhrase := v.cfg.SongTriggerPhrase
	v.mu.RUnlock()

	// Check if caller requested in-call song playback or to stop music
	action, songQuery := parseSongIntent(cleanInput, triggerPhrase)
	if action == "play" && songQuery != "" {
		go v.handleSongRequest(turnID, songQuery)
		return
	} else if action == "stop" {
		go v.handleStopSongRequest(turnID)
		return
	}

	v.mu.Lock()
	if turnID != atomic.LoadInt64(&v.activeTurn) {
		v.mu.Unlock()
		return
	}
	v.state = StateThinking
	voice := v.cfg.Voice
	geminiModel := v.cfg.GeminiModel
	if geminiModel == "" {
		geminiModel = "gemini-3.5-flash-lite"
	}
	sysPrompt := v.cfg.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = "You are a witty, helpful, ultra-concise real-time AI voice assistant on a live phone call. Keep responses direct, natural, conversational, and under 2 short sentences. Do not use bullet points, markdown formatting, emojis, asterisks, or lists."
	}
	v.mu.Unlock()

	startTime := time.Now()
	v.manager.log("INFO", "[VoiceBot] Caller said: \"%s\" (Streaming response with %s)...", cleanInput, geminiModel)

	// Play thinking filler noise after 180ms if still generating
	fillerTimer := time.AfterFunc(180*time.Millisecond, func() {
		v.mu.RLock()
		isStillThinking := v.state == StateThinking && atomic.LoadInt64(&v.activeTurn) == turnID
		v.mu.RUnlock()
		if isStillThinking {
			v.playThinkingFiller(voice)
		}
	})
	defer fillerTimer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	tmpDir := filepath.Join("data", "media", "voicebot")
	_ = os.MkdirAll(tmpDir, 0755)

	var streamMu sync.Mutex
	var streamBuf strings.Builder
	chunksSpoken := 0

	dispatchSentence := func(sentence string, isFirst bool) {
		clean := sanitizeSpokenText(sentence)
		if clean == "" {
			return
		}

		if isFirst {
			fillerTimer.Stop()
			v.mu.Lock()
			v.state = StateSpeaking
			v.mu.Unlock()
		}

		tmpPath := filepath.Join(tmpDir, fmt.Sprintf("reply_%d_%d.mp3", time.Now().UnixNano(), chunksSpoken))
		ttsStart := time.Now()

		if err := tts.SynthesizeWSToFile(ctx, clean, voice, tmpPath); err != nil {
			v.manager.log("WARN", "[VoiceBot] TTS synthesis failed for chunk %q: %v", clean, err)
			return
		}

		ttsDur := time.Since(ttsStart)
		if isFirst {
			firstLat := time.Since(startTime).Milliseconds()
			v.mu.Lock()
			v.lastLatencyMs = firstLat
			v.lastResponse = clean
			v.totalTurns++
			v.mu.Unlock()
			v.manager.log("INFO", "[VoiceBot] 🚀 FAST RESPONSE: First sentence playing in %dms (TTS: %dms): \"%s\"",
				firstLat, ttsDur.Milliseconds(), clean)
		} else {
			v.manager.log("DEBUG", "[VoiceBot] Queued sentence in %dms: \"%s\"", ttsDur.Milliseconds(), clean)
		}

		if playErr := v.manager.PlayVoiceResponse(tmpPath); playErr != nil {
			v.manager.log("WARN", "[VoiceBot] Failed to stream speech chunk %q into call: %v", clean, playErr)
		}
		chunksSpoken++
	}

	onChunk := func(token string) error {
		if turnID != atomic.LoadInt64(&v.activeTurn) {
			return errors.New("turn superseded")
		}

		streamMu.Lock()
		streamBuf.WriteString(token)
		currentBuf := streamBuf.String()
		sentence, remaining := extractNextSentence(currentBuf, chunksSpoken == 0)
		if sentence != "" {
			streamBuf.Reset()
			streamBuf.WriteString(remaining)
		}
		streamMu.Unlock()

		if sentence != "" {
			dispatchSentence(sentence, chunksSpoken == 0)
		}
		return nil
	}

	var replyText string
	var err error

	contents := v.buildGenerateContents(cleanInput)
	if gemini.GlobalClient != nil {
		replyText, err = gemini.GlobalClient.SolveMultiTurnWithModelStream(ctx, geminiModel, contents, sysPrompt, onChunk)
	} else {
		err = fmt.Errorf("gemini client is not initialized")
	}

	fillerTimer.Stop()

	// Flush any trailing text remaining after stream completes
	streamMu.Lock()
	trailing := streamBuf.String()
	streamBuf.Reset()
	streamMu.Unlock()

	if strings.TrimSpace(trailing) != "" {
		dispatchSentence(trailing, chunksSpoken == 0)
	}

	// If streaming yielded no chunks at all, try single-turn non-streaming fallback
	if chunksSpoken == 0 {
		if err != nil || strings.TrimSpace(replyText) == "" {
			v.manager.log("ERROR", "[VoiceBot] Generation failed: %v", err)
			v.mu.Lock()
			v.state = StateListening
			v.mu.Unlock()
			return
		}
		cleanReply := sanitizeSpokenText(replyText)
		if cleanReply != "" {
			dispatchSentence(cleanReply, true)
		}
	} else if replyText != "" {
		v.mu.Lock()
		v.lastResponse = sanitizeSpokenText(replyText)
		v.mu.Unlock()
	}

	// Record turn into temporary call conversation context
	var finalBotReply string
	if replyText != "" {
		finalBotReply = sanitizeSpokenText(replyText)
	} else if chunksSpoken > 0 {
		v.mu.RLock()
		finalBotReply = v.lastResponse
		v.mu.RUnlock()
	}

	if finalBotReply != "" {
		v.appendCallMessage("user", cleanInput)
		v.appendCallMessage("model", finalBotReply)
		v.manager.log("DEBUG", "[VoiceBot] 💬 Recorded turn in temporary call context (User: %q | Bot: %q | Total Turns: %d)",
			cleanInput, finalBotReply, len(v.GetCallContext())/2)
	}
}
