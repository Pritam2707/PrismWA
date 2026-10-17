package meowcaller

import (
	_ "whatsapp-bot/pkg/envfix"

	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/purpshell/meowcaller"
	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"
)

type AudioChannel byte

const (
	ChannelAll      AudioChannel = 0
	ChannelOutgoing AudioChannel = 1 // Bot's music or outgoing voice
	ChannelIncoming AudioChannel = 2 // Incoming voice from remote caller(s)
	ChannelMic      AudioChannel = 3 // Dashboard operator live mic broadcast
)

type audioSubscriber struct {
	id     string
	ch     chan []byte
	filter AudioChannel
}

type Track struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Query       string    `json:"query"`
	FilePath    string    `json:"filePath"`
	Duration    string    `json:"duration"`
	RequestedBy string    `json:"requestedBy"`
	AddedAt     time.Time `json:"addedAt"`
}

type ActiveCallInfo struct {
	ID          string    `json:"id"`
	Peer        string    `json:"peer"`
	Phase       string    `json:"phase"`
	IsOutgoing  bool      `json:"isOutgoing"`
	StartTime   time.Time `json:"startTime"`
	DurationSec int       `json:"durationSec"`
}

type PartyStatus struct {
	Enabled         bool            `json:"enabled"`
	TempSecondsLeft int             `json:"tempSecondsLeft"`
	AutoAcceptCalls bool            `json:"autoAcceptCalls"`
	IsPartyActive   bool            `json:"isPartyActive"`
	IsPlaying       bool            `json:"isPlaying"`
	IsPaused        bool            `json:"isPaused"`
	CurrentTrack    *Track          `json:"currentTrack"`
	Queue           []*Track        `json:"queue"`
	ActiveCall      *ActiveCallInfo    `json:"activeCall"`
	PendingCall     *ActiveCallInfo    `json:"pendingCall"`
	VoiceBot        *VoiceBotTelemetry `json:"voiceBot"`
	Radio           *RadioStatus       `json:"radio"`
	LinkedChatJID   string             `json:"linkedChatJid"`
}

type Manager struct {
	mu              sync.RWMutex
	enabled         bool
	tempTimer       *time.Timer
	tempExpiresAt   time.Time
	autoAcceptCalls bool

	callerClient *meowcaller.Client
	waClient     *whatsmeow.Client

	activeCall         *meowcaller.Call
	activeCallPeer     string
	activeCallStart    time.Time
	activeCallOutgoing bool
	activeCallPhase    string

	pendingCall      *meowcaller.Call
	pendingCallPeer  string
	pendingCallStart time.Time

	player        *meowcaller.Player
	currentTrack  *Track
	queue         []*Track
	isPlaying     bool
	isPaused      bool
	voiceQueue          []voiceChunkItem
	isVoicePlaying      bool
	linkedChatJID       string
	mediaDir      string
	logMu         sync.RWMutex
	logHandler    func(level, msg string)

	audioMu          sync.RWMutex
	audioSubscribers map[string]*audioSubscriber

	voiceBot *VoiceBot
	mixer    *radioAudioMixer
}

var GlobalManager = &Manager{
	enabled:          false,
	autoAcceptCalls:  false,
	queue:            make([]*Track, 0),
	mediaDir:         filepath.Join("data", "media", "party"),
	activeCallPhase:  "idle",
	audioSubscribers: make(map[string]*audioSubscriber),
}

func init() {
	GlobalManager.mixer = newRadioAudioMixer(GlobalManager)
	GlobalManager.player = meowcaller.NewPlayer()
	GlobalManager.player.Play(GlobalManager.mixer)
	GlobalManager.voiceBot = NewVoiceBot(GlobalManager)
}

func (m *Manager) SetLogHandler(fn func(level, msg string)) {
	m.logMu.Lock()
	defer m.logMu.Unlock()
	m.logHandler = fn
}

func (m *Manager) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("[MeowCaller] %s: %s", level, msg)
	m.logMu.RLock()
	fn := m.logHandler
	m.logMu.RUnlock()
	if fn != nil {
		fn(level, "[MeowCaller] "+msg)
	}
}

func (m *Manager) Init(callerClient *meowcaller.Client, waClient *whatsmeow.Client) {
	m.mu.Lock()
	m.callerClient = callerClient
	m.waClient = waClient
	_ = os.MkdirAll(m.mediaDir, 0755)
	m.mu.Unlock()

	if callerClient == nil {
		return
	}

	callerClient.OnIncomingCall(func(call *meowcaller.Call) {
		go m.handleIncomingCall(call)
	})
}

func (m *Manager) handleIncomingCall(call *meowcaller.Call) {
	m.mu.Lock()
	enabled := m.enabled
	if enabled && !m.tempExpiresAt.IsZero() && time.Now().After(m.tempExpiresAt) {
		m.enabled = false
		enabled = false
	}
	autoAccept := m.autoAcceptCalls
	peer := call.Peer().String()
	m.mu.Unlock()

	if !enabled {
		m.log("INFO", "Ignoring incoming call from %s (MeowCaller is disabled)", peer)
		return
	}

	// Attach lifecycle callbacks (OnReady, OnStateChange, OnEnd)
	m.hookCallLifecycle(call, peer)

	if autoAccept {
		m.log("INFO", "Auto-answering incoming call from %s", peer)
		if err := call.Answer(); err != nil {
			m.log("ERROR", "Failed to answer call from %s: %v", peer, err)
			return
		}

		m.mu.Lock()
		m.activeCall = call
		m.activeCallPeer = peer
		m.activeCallStart = time.Now()
		m.activeCallOutgoing = false
		m.activeCallPhase = "active"

		mixer := m.getOrCreateMixerLocked()
		if m.player != nil {
			if m.player.State() != meowcaller.PlayerPlaying {
				m.player.Play(mixer)
			}
			call.Subscribe(m.player)
		}
		m.mu.Unlock()
	} else {
		m.log("INFO", "Incoming call from %s ringing. Auto-accept is off. Waiting for answer or pickup on another device...", peer)
		m.mu.Lock()
		m.pendingCall = call
		m.pendingCallPeer = peer
		m.pendingCallStart = time.Now()
		m.mu.Unlock()
	}
}

func (m *Manager) hookCallLifecycle(call *meowcaller.Call, peer string) {
	if call == nil {
		return
	}

	var firstFrameOnce sync.Once
	sink := meowcaller.SinkFunc(func(frame []float32) {
		firstFrameOnce.Do(func() {
			m.log("SUCCESS", "Inbound voice stream active: receiving audio from %s (%d samples/frame)", peer, len(frame))
		})
		m.broadcastAudioFrame(ChannelIncoming, frame)

		// Feed inbound audio to real-time voice bot
		if m.voiceBot != nil && m.voiceBot.IsEnabled() {
			m.voiceBot.FeedAudioFrame(frame)
		}
	})

	attachMedia := func() {
		call.Receive(sink)
		mixer := m.getOrCreateMixer()
		m.mu.Lock()
		if m.player == nil {
			m.player = meowcaller.NewPlayer()
		}
		if m.player.State() != meowcaller.PlayerPlaying {
			m.player.Play(mixer)
		}
		call.Subscribe(m.player)
		m.mu.Unlock()
	}

	call.OnReady(func() {
		m.log("INFO", "Media session is READY for call with %s: relay connected and inbound audio active", peer)
		if m.voiceBot != nil {
			m.voiceBot.StartCallContext(peer)
		}
		attachMedia()
	})

	call.OnStateChange(func(phase meowcaller.CallPhase) {
		m.mu.Lock()
		m.activeCallPhase = phaseToString(phase)
		m.mu.Unlock()
		if phase == meowcaller.CallPhaseActive {
			if m.voiceBot != nil {
				m.voiceBot.StartCallContext(peer)
			}
			attachMedia()
		}
	})

	call.OnEnd(func(reason string) {
		m.log("INFO", "Call with %s ended (reason: %s)", peer, reason)
		m.mu.Lock()
		if m.activeCall == call {
			m.activeCall = nil
			m.activeCallPeer = ""
			m.activeCallPhase = "idle"
		}
		if m.pendingCall == call {
			m.pendingCall = nil
			m.pendingCallPeer = ""
		}
		m.mu.Unlock()

		if m.voiceBot != nil {
			m.voiceBot.FlushCallContext("call ended: " + reason)
		}
	})
}

func phaseToString(phase meowcaller.CallPhase) string {
	switch phase {
	case meowcaller.CallPhaseIdle:
		return "idle"
	case meowcaller.CallPhaseCalling:
		return "calling"
	case meowcaller.CallPhaseRinging:
		return "ringing"
	case meowcaller.CallPhaseConnecting:
		return "connecting"
	case meowcaller.CallPhaseActive:
		return "active"
	case meowcaller.CallPhaseEnded:
		return "ended"
	case meowcaller.CallPhaseWaitingRoom:
		return "waiting_room"
	default:
		return "unknown"
	}
}

func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return false
	}
	if !m.tempExpiresAt.IsZero() && time.Now().After(m.tempExpiresAt) {
		return false
	}
	return true
}

func (m *Manager) SetEnabled(enabled bool, durationMinutes int) {
	m.mu.Lock()

	if m.tempTimer != nil {
		m.tempTimer.Stop()
		m.tempTimer = nil
	}

	m.enabled = enabled

	if enabled {
		if durationMinutes > 0 {
			duration := time.Duration(durationMinutes) * time.Minute
			m.tempExpiresAt = time.Now().Add(duration)
			m.tempTimer = time.AfterFunc(duration, func() {
				m.log("INFO", "Temporary MeowCaller window of %d minutes expired. Auto-disabling.", durationMinutes)
				m.SetEnabled(false, 0)
			})
			m.mu.Unlock()
			m.log("SYSTEM", "MeowCaller temporarily enabled for %d minutes (expires at %s)", durationMinutes, m.tempExpiresAt.Format("15:04:05"))
			return
		}

		m.tempExpiresAt = time.Time{}
		m.mu.Unlock()
		m.log("SYSTEM", "MeowCaller enabled indefinitely via dashboard")
		return
	}

	m.tempExpiresAt = time.Time{}
	activeCall := m.activeCall
	player := m.player
	m.isPlaying = false
	m.isPaused = false
	m.currentTrack = nil
	m.queue = make([]*Track, 0)
	m.mu.Unlock()

	if player != nil {
		player.Stop()
	}
	if activeCall != nil {
		_ = activeCall.Hangup()
	}
	if m.voiceBot != nil {
		m.voiceBot.FlushCallContext("manager disabled")
	}

	m.log("SYSTEM", "MeowCaller disabled via dashboard. All calls and song parties stopped.")
}

func (m *Manager) SetAutoAcceptCalls(accept bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoAcceptCalls = accept
	m.log("INFO", "Auto-accept calls set to %v", accept)
}

func (m *Manager) GetStatus() PartyStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	enabled := m.enabled
	tempSecs := 0
	if enabled && !m.tempExpiresAt.IsZero() {
		rem := time.Until(m.tempExpiresAt)
		if rem > 0 {
			tempSecs = int(rem.Seconds())
		} else {
			enabled = false
		}
	}

	var callInfo *ActiveCallInfo
	if m.activeCall != nil {
		dur := 0
		if !m.activeCallStart.IsZero() {
			dur = int(time.Since(m.activeCallStart).Seconds())
		}
		callInfo = &ActiveCallInfo{
			ID:          m.activeCall.ID(),
			Peer:        m.activeCallPeer,
			Phase:       m.activeCallPhase,
			IsOutgoing:  m.activeCallOutgoing,
			StartTime:   m.activeCallStart,
			DurationSec: dur,
		}
	}

	var pendingInfo *ActiveCallInfo
	if m.pendingCall != nil {
		pendingDur := 0
		if !m.pendingCallStart.IsZero() {
			pendingDur = int(time.Since(m.pendingCallStart).Seconds())
		}
		pendingInfo = &ActiveCallInfo{
			ID:          m.pendingCall.ID(),
			Peer:        m.pendingCallPeer,
			Phase:       "ringing",
			IsOutgoing:  false,
			StartTime:   m.pendingCallStart,
			DurationSec: pendingDur,
		}
	}

	queueCopy := make([]*Track, len(m.queue))
	copy(queueCopy, m.queue)

	var voiceBotTelemetry *VoiceBotTelemetry
	if m.voiceBot != nil {
		t := m.voiceBot.GetTelemetry()
		voiceBotTelemetry = &t
	}

	var radioStatus RadioStatus
	if m.mixer != nil {
		radioStatus = m.mixer.Status()
	}
	radioStatus.HasCall = m.activeCall != nil

	return PartyStatus{
		Enabled:         enabled,
		TempSecondsLeft: tempSecs,
		AutoAcceptCalls: m.autoAcceptCalls,
		IsPartyActive:   m.isPlaying || m.activeCall != nil,
		IsPlaying:       m.isPlaying,
		IsPaused:        m.isPaused,
		CurrentTrack:    m.currentTrack,
		Queue:           queueCopy,
		ActiveCall:      callInfo,
		PendingCall:     pendingInfo,
		VoiceBot:        voiceBotTelemetry,
		Radio:           &radioStatus,
		LinkedChatJID:   m.linkedChatJID,
	}
}

// DownloadAudio extracts or downloads audio using yt-dlp to an mp3 file
func (m *Manager) DownloadAudio(ctx context.Context, query string) (*Track, error) {
	_ = os.MkdirAll(m.mediaDir, 0755)

	target := strings.TrimSpace(query)
	if target == "" {
		return nil, fmt.Errorf("empty song query")
	}

	if !strings.HasPrefix(target, "http") {
		target = "ytsearch1:" + target
	}

	trackID := uuid.New().String()[:8]
	outPattern := filepath.Join(m.mediaDir, fmt.Sprintf("party-%s.%%(ext)s", trackID))

	args := []string{
		"--no-playlist",
		"--no-warnings",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--max-filesize", "50M",
		"--no-simulate",
		"--print", "%(title)s",
		"-o", outPattern,
		target,
	}

	if _, err := os.Stat("cookies.txt"); err == nil {
		args = append([]string{"--cookies", "cookies.txt"}, args...)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run yt-dlp: %w", err)
	}

	var songTitle string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && songTitle == "" {
			songTitle = line
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("yt-dlp download failed: %w", err)
	}

	expectedFile := filepath.Join(m.mediaDir, fmt.Sprintf("party-%s.mp3", trackID))
	if _, err := os.Stat(expectedFile); err != nil {
		matches, _ := filepath.Glob(filepath.Join(m.mediaDir, fmt.Sprintf("party-%s.*", trackID)))
		if len(matches) > 0 {
			convCmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", matches[0], "-acodec", "libmp3lame", expectedFile)
			if convErr := convCmd.Run(); convErr == nil {
				_ = os.Remove(matches[0])
			} else {
				expectedFile = matches[0]
			}
		} else {
			return nil, fmt.Errorf("downloaded audio file not found on disk")
		}
	}

	if songTitle == "" {
		songTitle = query
	}

	return &Track{
		ID:       trackID,
		Title:    songTitle,
		Query:    query,
		FilePath: expectedFile,
		AddedAt:  time.Now(),
	}, nil
}

// Dial places a WhatsApp voice call to a user or group
func (m *Manager) Dial(ctx context.Context, target string) (*meowcaller.Call, error) {
	if !m.IsEnabled() {
		return nil, fmt.Errorf("MeowCaller is disabled. Please enable it temporarily via the dashboard first")
	}

	m.mu.Lock()
	client := m.callerClient
	m.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("MeowCaller VoIP client is not initialized")
	}

	cleanTarget := strings.TrimSpace(target)
	isGroup := strings.HasSuffix(cleanTarget, "@g.us")

	m.log("INFO", "Initiating call to target: %s (isGroup: %v)", cleanTarget, isGroup)

	var call *meowcaller.Call
	var err error

	if isGroup {
		call, err = client.GroupCallByID(ctx, cleanTarget)
	} else {
		call, err = client.Call(ctx, cleanTarget)
	}

	if err != nil {
		m.log("ERROR", "Failed to place call to %s: %v", cleanTarget, err)
		return nil, err
	}

	m.mu.Lock()
	m.activeCall = call
	m.activeCallPeer = cleanTarget
	m.activeCallStart = time.Now()
	m.activeCallOutgoing = true
	m.activeCallPhase = "calling"
	m.mu.Unlock()

	m.hookCallLifecycle(call, cleanTarget)
	return call, nil
}

// AnswerCall answers a pending ringing call
func (m *Manager) AnswerCall() error {
	m.mu.Lock()
	call := m.pendingCall
	peer := m.pendingCallPeer
	m.pendingCall = nil
	m.pendingCallPeer = ""
	m.mu.Unlock()

	if call == nil {
		return fmt.Errorf("no pending incoming call to answer")
	}

	m.log("INFO", "Answering incoming call from %s via dashboard", peer)
	if err := call.Answer(); err != nil {
		m.log("ERROR", "Failed to answer call from %s: %v", peer, err)
		return err
	}

	m.mu.Lock()
	m.activeCall = call
	m.activeCallPeer = peer
	m.activeCallStart = time.Now()
	m.activeCallOutgoing = false
	m.activeCallPhase = "active"

	mixer := m.getOrCreateMixerLocked()
	if m.player != nil {
		if m.player.State() != meowcaller.PlayerPlaying {
			m.player.Play(mixer)
		}
		call.Subscribe(m.player)
	}
	m.mu.Unlock()

	return nil
}

// RejectCall rejects a pending ringing call
func (m *Manager) RejectCall() error {
	m.mu.Lock()
	call := m.pendingCall
	peer := m.pendingCallPeer
	m.pendingCall = nil
	m.pendingCallPeer = ""
	m.mu.Unlock()

	if call == nil {
		return fmt.Errorf("no pending incoming call to reject")
	}

	if m.voiceBot != nil {
		m.voiceBot.FlushCallContext("call rejected")
	}

	m.log("INFO", "Rejecting incoming call from %s via dashboard", peer)
	return call.Reject()
}

// Hangup ends the active call and dismisses any pending call
func (m *Manager) Hangup() error {
	m.mu.Lock()
	call := m.activeCall
	m.activeCall = nil
	m.activeCallPeer = ""
	m.activeCallPhase = "idle"
	pending := m.pendingCall
	pendingPeer := m.pendingCallPeer
	m.pendingCall = nil
	m.pendingCallPeer = ""
	player := m.player
	mixer := m.mixer
	m.isPlaying = false
	m.isPaused = false
	m.currentTrack = nil
	m.mu.Unlock()

	if m.voiceBot != nil {
		m.voiceBot.FlushCallContext("hangup requested")
	}

	if pending != nil {
		_ = pending.Reject()
		m.log("INFO", "Pending call from %s dismissed on hangup", pendingPeer)
	}

	if mixer != nil {
		mixer.Reset()
	}

	if player != nil {
		player.Stop()
	}

	if call != nil {
		m.log("INFO", "Hanging up call %s", call.ID())
		return call.Hangup()
	}
	return nil
}

// StartParty starts song party: downloads track, places call if needed, and streams music
func (m *Manager) StartParty(ctx context.Context, target string, query string, requestedBy string, chatJID string) (*Track, error) {
	if !m.IsEnabled() {
		return nil, fmt.Errorf("MeowCaller is disabled in dashboard. Enable it temporarily in the Web Dashboard to host a Song Party")
	}

	track, err := m.DownloadAudio(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to download audio for song party: %w", err)
	}
	track.RequestedBy = requestedBy

	m.mu.Lock()
	m.linkedChatJID = chatJID
	call := m.activeCall
	m.mu.Unlock()

	if call == nil && target != "" {
		call, err = m.Dial(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("failed to dial target for song party: %w", err)
		}
	}

	if err := m.playTrack(track); err != nil {
		return nil, err
	}

	m.notifyChat(fmt.Sprintf("🎉 *Song Party Started!*\n🎶 *Now Playing:* \"%s\"\n👤 *Requested by:* %s", track.Title, requestedBy))
	return track, nil
}

// AddToQueue downloads and queues a track for the party
func (m *Manager) AddToQueue(ctx context.Context, query string, requestedBy string) (*Track, error) {
	if !m.IsEnabled() {
		return nil, fmt.Errorf("MeowCaller is disabled in dashboard")
	}

	track, err := m.DownloadAudio(ctx, query)
	if err != nil {
		return nil, err
	}
	track.RequestedBy = requestedBy

	m.mu.Lock()
	playing := m.isPlaying
	if !playing {
		m.mu.Unlock()
		if err := m.playTrack(track); err != nil {
			return nil, err
		}
		m.notifyChat(fmt.Sprintf("🎶 *Now Playing (Song Party):* \"%s\"\n👤 *Requested by:* %s", track.Title, requestedBy))
		return track, nil
	}

	m.queue = append(m.queue, track)
	queueLen := len(m.queue)
	m.mu.Unlock()

	m.log("INFO", "Added track to queue: %s (Total queue: %d)", track.Title, queueLen)
	m.notifyChat(fmt.Sprintf("➕ *Added to Party Queue:* \"%s\"\n🔢 *Position:* #%d", track.Title, queueLen))
	return track, nil
}

// PlayTrack streams an MP3 track into the active WhatsApp call using the radioAudioMixer
func (m *Manager) PlayTrack(track *Track) error {
	return m.playTrack(track)
}

func (m *Manager) playTrack(track *Track) error {
	audioSource, err := meowcaller.MP3File(track.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open MP3 source %s: %w", track.FilePath, err)
	}

	m.mu.Lock()
	mixer := m.getOrCreateMixerLocked()
	player := m.player
	m.currentTrack = track
	m.isPlaying = true
	m.isPaused = false

	if player != nil && player.State() != meowcaller.PlayerPlaying {
		player.Play(mixer)
	}
	call := m.activeCall
	if call != nil && player != nil {
		call.Subscribe(player)
	}
	m.mu.Unlock()

	mixer.SetMusicSource(audioSource, track, func() {
		m.log("INFO", "Track finished: %s", track.Title)
		m.playNextTrack()
	})

	m.log("INFO", "Started streaming track: %s into call", track.Title)
	return nil
}

func (m *Manager) playNextTrack() {
	m.mu.Lock()
	if len(m.queue) == 0 {
		m.isPlaying = false
		m.currentTrack = nil
		m.mu.Unlock()
		m.log("INFO", "Song Party queue finished.")
		m.notifyChat("🏁 *Song Party playlist has finished.* Add more songs with `!party add <song>`!")
		return
	}

	nextTrack := m.queue[0]
	m.queue = m.queue[1:]
	m.mu.Unlock()

	if err := m.playTrack(nextTrack); err != nil {
		m.log("ERROR", "Failed to play next track %s: %v", nextTrack.Title, err)
		m.playNextTrack()
		return
	}

	m.notifyChat(fmt.Sprintf("🎶 *Now Playing (Song Party):* \"%s\"\n👤 *Requested by:* %s", nextTrack.Title, nextTrack.RequestedBy))
}

func (m *Manager) Pause() error {
	m.mu.Lock()
	if m.mixer == nil || !m.isPlaying {
		m.mu.Unlock()
		return fmt.Errorf("no song currently playing")
	}
	mixer := m.mixer
	m.isPaused = true
	m.mu.Unlock()

	mixer.PauseMusic()
	m.log("INFO", "Song party playback paused")
	return nil
}

func (m *Manager) Resume() error {
	m.mu.Lock()
	if m.mixer == nil || !m.isPlaying {
		m.mu.Unlock()
		return fmt.Errorf("no song currently playing")
	}
	mixer := m.mixer
	m.isPaused = false
	m.mu.Unlock()

	mixer.ResumeMusic()
	m.log("INFO", "Song party playback resumed")
	return nil
}

func (m *Manager) Skip() (*Track, error) {
	m.mu.Lock()
	if !m.isPlaying {
		m.mu.Unlock()
		return nil, fmt.Errorf("no song currently playing to skip")
	}
	curr := m.currentTrack
	mixer := m.mixer
	m.mu.Unlock()

	if mixer != nil {
		mixer.ClearMusicSource()
	}
	if curr != nil {
		_ = os.Remove(curr.FilePath)
	}

	m.log("INFO", "Skipped current track")
	m.playNextTrack()
	return curr, nil
}

func (m *Manager) StopParty() {
	m.mu.Lock()
	mixer := m.mixer
	player := m.player
	m.isPlaying = false
	m.isPaused = false
	m.currentTrack = nil
	for _, t := range m.queue {
		_ = os.Remove(t.FilePath)
	}
	m.queue = make([]*Track, 0)
	call := m.activeCall
	m.mu.Unlock()

	if mixer != nil {
		mixer.ClearMusicSource()
	}
	if player != nil {
		player.Stop()
	}
	if call != nil {
		_ = call.Hangup()
	}
	m.log("INFO", "Song party stopped and queue cleared")
	m.notifyChat("⏹️ *Song Party stopped and call ended.*")
}

// StopMusic stops currently playing song/track without hanging up the active call
func (m *Manager) StopMusic() {
	m.mu.Lock()
	mixer := m.mixer
	m.isPlaying = false
	m.isPaused = false
	m.currentTrack = nil
	for _, t := range m.queue {
		_ = os.Remove(t.FilePath)
	}
	m.queue = make([]*Track, 0)
	m.mu.Unlock()

	if mixer != nil {
		mixer.ClearMusicSource()
	}
	m.log("INFO", "Stopped active music playback during call")
}

func (m *Manager) RemoveFromQueue(index int) (*Track, error) {
	m.mu.Lock()
	if index < 0 || index >= len(m.queue) {
		m.mu.Unlock()
		return nil, fmt.Errorf("invalid queue index")
	}

	removed := m.queue[index]
	m.queue = append(m.queue[:index], m.queue[index+1:]...)
	m.mu.Unlock()

	_ = os.Remove(removed.FilePath)
	m.log("INFO", "Removed track from queue: %s", removed.Title)
	return removed, nil
}

func (m *Manager) notifyChat(text string) {
	m.mu.RLock()
	chatJIDStr := m.linkedChatJID
	waClient := m.waClient
	m.mu.RUnlock()

	if chatJIDStr == "" || waClient == nil {
		return
	}

	targetJID, err := types.ParseJID(chatJIDStr)
	if err != nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = waClient.SendMessage(ctx, targetJID, &waProto.Message{
			Conversation: proto.String(text),
		})
	}()
}

// teeAudioSource duplicates frames read by the Player to an audio frame handler
type teeAudioSource struct {
	src     meowcaller.AudioSource
	onFrame func(frame []float32)
}

func newTeeAudioSource(src meowcaller.AudioSource, onFrame func(frame []float32)) *teeAudioSource {
	return &teeAudioSource{src: src, onFrame: onFrame}
}

func (t *teeAudioSource) ReadFrame() ([]float32, error) {
	frame, err := t.src.ReadFrame()
	if err == nil && len(frame) > 0 && t.onFrame != nil {
		t.onFrame(frame)
	}
	return frame, err
}

func (t *teeAudioSource) Close() error {
	return t.src.Close()
}

// SubscribeAudio registers a listener channel to receive live 16kHz s16le PCM audio frames
func (m *Manager) SubscribeAudio(filter AudioChannel) (string, <-chan []byte, func()) {
	subID := uuid.New().String()
	ch := make(chan []byte, 256)
	sub := &audioSubscriber{
		id:     subID,
		ch:     ch,
		filter: filter,
	}

	m.audioMu.Lock()
	if m.audioSubscribers == nil {
		m.audioSubscribers = make(map[string]*audioSubscriber)
	}
	m.audioSubscribers[subID] = sub
	activeCount := len(m.audioSubscribers)
	m.audioMu.Unlock()

	m.log("INFO", "Audio listener connected (id: %s, filter: %d, active listeners: %d)", subID[:8], filter, activeCount)

	cancel := func() {
		m.audioMu.Lock()
		delete(m.audioSubscribers, subID)
		remaining := len(m.audioSubscribers)
		m.audioMu.Unlock()
		m.log("INFO", "Audio listener disconnected (id: %s, remaining: %d)", subID[:8], remaining)
	}

	return subID, ch, cancel
}

// broadcastAudioFrame encodes 960 float32 samples to 16-bit PCM and broadcasts to subscribers
func (m *Manager) broadcastAudioFrame(ch AudioChannel, frame []float32) {
	if len(frame) == 0 {
		return
	}

	m.audioMu.RLock()
	if len(m.audioSubscribers) == 0 {
		m.audioMu.RUnlock()
		return
	}

	subs := make([]*audioSubscriber, 0, len(m.audioSubscribers))
	for _, sub := range m.audioSubscribers {
		if sub.filter == ChannelAll || sub.filter == ch {
			subs = append(subs, sub)
		}
	}
	m.audioMu.RUnlock()

	if len(subs) == 0 {
		return
	}

	// Packet structure: [1 byte channel tag, 16-bit LE PCM samples]
	// 960 samples * 2 bytes = 1920 bytes PCM + 1 byte tag = 1921 bytes
	packet := make([]byte, 1+len(frame)*2)
	packet[0] = byte(ch)
	for i, s := range frame {
		v := s * 32768.0
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(packet[1+i*2:], uint16(int16(v)))
	}

	for _, sub := range subs {
		select {
		case sub.ch <- packet:
		default:
			// Subscriber buffer full; drop frame to preserve real-time timing
		}
	}
}

// GetAudioMonitorStatus returns stats about current audio listeners and call audio
func (m *Manager) GetAudioMonitorStatus() map[string]interface{} {
	m.mu.RLock()
	hasCall := m.activeCall != nil
	callPeer := m.activeCallPeer
	isPlaying := m.isPlaying
	m.mu.RUnlock()

	m.audioMu.RLock()
	listenerCount := len(m.audioSubscribers)
	m.audioMu.RUnlock()

	return map[string]interface{}{
		"activeCall":      hasCall,
		"callPeer":        callPeer,
		"isPlaying":       isPlaying,
		"activeListeners": listenerCount,
		"sampleRate":      16000,
		"channels":        1,
		"frameSamples":    960,
	}
}

// voiceChunkItem pairs an MP3 audio path with an optional completion callback
type voiceChunkItem struct {
	filePath string
	onFinish func()
}

// InterruptAudio immediately halts current playback (used for barge-in speech interruption)
func (m *Manager) InterruptAudio() {
	m.mu.Lock()
	mixer := m.mixer
	m.isPlaying = false
	m.isPaused = false
	m.isVoicePlaying = false
	for _, item := range m.voiceQueue {
		if !strings.Contains(item.filePath, "fillers") {
			_ = os.Remove(item.filePath)
		}
	}
	m.voiceQueue = nil
	m.mu.Unlock()

	if mixer != nil {
		mixer.ClearMusicSource()
	}
}

// PlayVoiceResponse streams an AI synthesized voice response into the active WhatsApp call.
// If an earlier sentence is already playing, subsequent audio chunks are queued for continuous seamless playback.
func (m *Manager) PlayVoiceResponse(filePath string) error {
	return m.PlayVoiceResponseWithCallback(filePath, nil)
}

// PlayVoiceResponseWithCallback streams a synthesized voice response and executes onFinish when playback finishes.
func (m *Manager) PlayVoiceResponseWithCallback(filePath string, onFinish func()) error {
	m.mu.Lock()
	call := m.activeCall
	if call == nil {
		m.mu.Unlock()
		if !strings.Contains(filePath, "fillers") {
			_ = os.Remove(filePath)
		}
		return fmt.Errorf("cannot play voice response: no active WhatsApp call")
	}

	item := voiceChunkItem{
		filePath: filePath,
		onFinish: onFinish,
	}

	if m.isVoicePlaying {
		m.voiceQueue = append(m.voiceQueue, item)
		m.mu.Unlock()
		return nil
	}

	m.isVoicePlaying = true
	m.isPlaying = true
	m.isPaused = false
	m.mu.Unlock()

	return m.playVoiceChunk(item)
}

func (m *Manager) playVoiceChunk(item voiceChunkItem) error {
	filePath := item.filePath
	m.mu.Lock()
	call := m.activeCall
	if call == nil {
		m.isVoicePlaying = false
		m.isPlaying = false
		m.mu.Unlock()
		if !strings.Contains(filePath, "fillers") {
			_ = os.Remove(filePath)
		}
		return fmt.Errorf("cannot play voice chunk: no active WhatsApp call")
	}

	mixer := m.getOrCreateMixerLocked()
	player := m.player
	m.isPlaying = true
	m.isPaused = false
	if player.State() != meowcaller.PlayerPlaying {
		player.Play(mixer)
	}
	call.Subscribe(player)
	m.mu.Unlock()

	audioSource, err := meowcaller.MP3File(filePath)
	if err != nil {
		m.log("ERROR", "Failed to open voice MP3 %s: %v", filePath, err)
		if !strings.Contains(filePath, "fillers") {
			_ = os.Remove(filePath)
		}
		if item.onFinish != nil {
			go item.onFinish()
		}
		m.mu.Lock()
		if len(m.voiceQueue) > 0 {
			next := m.voiceQueue[0]
			m.voiceQueue = m.voiceQueue[1:]
			m.mu.Unlock()
			return m.playVoiceChunk(next)
		}
		m.isVoicePlaying = false
		m.isPlaying = false
		m.mu.Unlock()
		return fmt.Errorf("failed to open voice MP3 %s: %w", filePath, err)
	}

	mixer.SetMusicSource(audioSource, &Track{Title: "AI Voice Response", FilePath: filePath}, func() {
		if !strings.Contains(filePath, "fillers") {
			_ = os.Remove(filePath)
		}

		if item.onFinish != nil {
			item.onFinish()
		}

		m.mu.Lock()
		if len(m.voiceQueue) > 0 {
			next := m.voiceQueue[0]
			m.voiceQueue = m.voiceQueue[1:]
			m.mu.Unlock()
			_ = m.playVoiceChunk(next)
			return
		}

		m.isVoicePlaying = false
		m.isPlaying = false
		m.mu.Unlock()

		if m.voiceBot != nil {
			// Small 250ms cooldown so residual room acoustic echo settles before listening re-arms
			time.AfterFunc(250*time.Millisecond, func() {
				m.voiceBot.OnPlaybackFinished()
			})
		}
	})

	return nil
}

// PlayVoiceFiller streams a short thinking noise / filler into the active call
func (m *Manager) PlayVoiceFiller(filePath string) error {
	m.mu.Lock()
	call := m.activeCall
	if call == nil {
		m.mu.Unlock()
		return fmt.Errorf("cannot play voice filler: no active WhatsApp call")
	}

	mixer := m.getOrCreateMixerLocked()
	player := m.player
	m.isPlaying = true
	m.isPaused = false
	if player.State() != meowcaller.PlayerPlaying {
		player.Play(mixer)
	}
	call.Subscribe(player)
	m.mu.Unlock()

	audioSource, err := meowcaller.MP3File(filePath)
	if err != nil {
		return fmt.Errorf("failed to open filler MP3 %s: %w", filePath, err)
	}

	mixer.SetMusicSource(audioSource, &Track{Title: "AI Thinking Filler", FilePath: filePath}, func() {
		m.mu.Lock()
		m.isPlaying = false
		m.mu.Unlock()
	})

	return nil
}

// getOrCreateMixerLocked returns the active radioAudioMixer and ensures player is initialized (m.mu must be held)
func (m *Manager) getOrCreateMixerLocked() *radioAudioMixer {
	if m.mixer == nil {
		m.mixer = newRadioAudioMixer(m)
	}
	if m.player == nil {
		m.player = meowcaller.NewPlayer()
	}
	return m.mixer
}

// getOrCreateMixer returns the active radioAudioMixer and ensures player is initialized
func (m *Manager) getOrCreateMixer() *radioAudioMixer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getOrCreateMixerLocked()
}

// SetRadioMicActive toggles whether the dashboard operator's microphone streams into the call
func (m *Manager) SetRadioMicActive(active bool) {
	mixer := m.getOrCreateMixer()
	mixer.SetMicActive(active)
	m.log("INFO", "Radio Mic Active set to: %v", active)
}

// SetRadioMode sets the radio mixing mode ("ducking" or "mic_only")
func (m *Manager) SetRadioMode(mode string) {
	mixer := m.getOrCreateMixer()
	mixer.SetRadioMode(RadioMode(mode))
	m.log("INFO", "Radio Mode set to: %s", mode)
}

// SetRadioDuckLevel sets the volume of background music when ducked (0.0 to 1.0)
func (m *Manager) SetRadioDuckLevel(level float32) {
	mixer := m.getOrCreateMixer()
	mixer.SetDuckLevel(level)
}

// SetRadioMicGain sets the gain multiplier for incoming microphone PCM
func (m *Manager) SetRadioMicGain(gain float32) {
	mixer := m.getOrCreateMixer()
	mixer.SetMicGain(gain)
}

// PushMicFrame pushes a frame of 16kHz float32 audio samples from the operator's mic
func (m *Manager) PushMicFrame(frame []float32) {
	mixer := m.getOrCreateMixer()
	mixer.PushMicFrame(frame)
}

// GetRadioStatus returns current telemetry about the radio station mic input
func (m *Manager) GetRadioStatus() RadioStatus {
	m.mu.RLock()
	mixer := m.mixer
	hasCall := m.activeCall != nil
	m.mu.RUnlock()

	if mixer == nil {
		mixer = m.getOrCreateMixer()
	}
	status := mixer.Status()
	status.HasCall = hasCall
	return status
}

// GetVoiceBot returns the real-time voice bot instance
func (m *Manager) GetVoiceBot() *VoiceBot {
	return m.voiceBot
}

// UpdateVoiceBotConfig updates configuration for the VoiceBot
func (m *Manager) UpdateVoiceBotConfig(cfg VoiceBotConfig) {
	if m.voiceBot != nil {
		m.voiceBot.UpdateConfig(cfg)
	}
}

// ConnectVoiceBotWebSocket tests and connects VoiceBot to a WebSocket server
func (m *Manager) ConnectVoiceBotWebSocket(serverURL string) error {
	if m.voiceBot == nil {
		return fmt.Errorf("voice bot is not initialized")
	}
	return m.voiceBot.ConnectWebSocket(serverURL)
}



