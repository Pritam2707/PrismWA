package meowcaller

import (
	"os"
	"sync"
	"time"

	"github.com/purpshell/meowcaller"
)

// RadioMode determines audio mixing behavior when the operator's microphone is live
type RadioMode string

const (
	// RadioModeDucking lowers music volume (e.g. to 15%) while operator speaks, like a radio DJ
	RadioModeDucking RadioMode = "ducking"
	// RadioModeMicOnly mutes music completely while the operator is speaking
	RadioModeMicOnly RadioMode = "mic_only"
)

// RadioStatus provides real-time telemetry about the radio station mic input
type RadioStatus struct {
	MicActive  bool    `json:"micActive"`
	Mode       string  `json:"mode"`
	DuckLevel  float32 `json:"duckLevel"`
	MicGain    float32 `json:"micGain"`
	HasCall    bool    `json:"hasCall"`
	IsSpeaking bool    `json:"isSpeaking"`
}

// radioAudioMixer implements meowcaller.AudioSource, dynamically mixing live DJ mic input
// with background party music or silence frames to maintain continuous WhatsApp VoIP RTP stream.
type radioAudioMixer struct {
	mu            sync.Mutex
	musicSrc      meowcaller.AudioSource
	musicTrack    *Track
	onMusicFinish func()
	musicPaused   bool

	micQueue       chan []float32
	micActive      bool
	lastMicAudio   time.Time
	radioMode      RadioMode
	duckLevel      float32 // 0.15 (15% music volume when ducked)
	micGain        float32 // 1.0 (gain multiplier)
	currentDucking float32 // smooth gain interpolation to prevent pops/clicks

	mgr *Manager
}

func newRadioAudioMixer(mgr *Manager) *radioAudioMixer {
	return &radioAudioMixer{
		mgr:            mgr,
		micQueue:       make(chan []float32, 128),
		radioMode:      RadioModeDucking,
		duckLevel:      0.15,
		micGain:        1.0,
		currentDucking: 1.0,
	}
}

// ReadFrame pulls a 60ms audio frame (960 float32 samples at 16kHz mono).
// It mixes the active music track (ducked or muted as appropriate) with any buffered mic speech.
// If neither is producing audio, it yields 960 silence samples to keep the WhatsApp relay bridge open.
func (r *radioAudioMixer) ReadFrame() ([]float32, error) {
	r.mu.Lock()
	musicSrc := r.musicSrc
	musicPaused := r.musicPaused
	micActive := r.micActive
	mode := r.radioMode
	duckLevel := r.duckLevel
	micGain := r.micGain
	r.mu.Unlock()

	out := make([]float32, 960)

	// 1. Process Music Source
	var musicFrame []float32
	var musicFinished bool
	if musicSrc != nil && !musicPaused {
		var err error
		musicFrame, err = musicSrc.ReadFrame()
		if err != nil {
			musicFinished = true
		}
	}

	if musicFinished {
		r.mu.Lock()
		var finishFn func()
		if r.musicSrc == musicSrc {
			r.musicSrc = nil
			track := r.musicTrack
			r.musicTrack = nil
			if track != nil {
				_ = os.Remove(track.FilePath)
			}
			finishFn = r.onMusicFinish
		}
		r.mu.Unlock()
		_ = musicSrc.Close()
		if finishFn != nil {
			go finishFn()
		}
	}

	// 2. Process Microphone
	var micFrame []float32
	if micActive {
		select {
		case micFrame = <-r.micQueue:
		default:
		}
	}

	// 3. Smooth Ducking Logic
	hasMicSpeech := micActive && len(micFrame) > 0
	r.mu.Lock()
	if hasMicSpeech {
		r.lastMicAudio = time.Now()
	}
	speakingRecently := time.Since(r.lastMicAudio) < 300*time.Millisecond

	targetDucking := float32(1.0)
	if micActive && (hasMicSpeech || speakingRecently) {
		if mode == RadioModeMicOnly {
			targetDucking = 0.0
		} else {
			targetDucking = duckLevel
		}
	}

	// Smooth linear ramp towards target ducking level (step ~0.08 per 60ms = ~350ms transition)
	curDuck := r.currentDucking
	step := float32(0.08)
	if curDuck < targetDucking {
		curDuck += step
		if curDuck > targetDucking {
			curDuck = targetDucking
		}
	} else if curDuck > targetDucking {
		curDuck -= step
		if curDuck < targetDucking {
			curDuck = targetDucking
		}
	}
	r.currentDucking = curDuck
	r.mu.Unlock()

	// 4. Mix Music
	if len(musicFrame) > 0 && curDuck > 0.001 {
		for i := 0; i < len(out) && i < len(musicFrame); i++ {
			out[i] = musicFrame[i] * curDuck
		}
	}

	// 5. Mix Live Microphone
	if len(micFrame) > 0 {
		for i := 0; i < len(out) && i < len(micFrame); i++ {
			v := out[i] + micFrame[i]*micGain
			if v > 1.0 {
				v = 1.0
			} else if v < -1.0 {
				v = -1.0
			}
			out[i] = v
		}
	}

	// 6. Broadcast outgoing audio frame to local audio monitor
	if r.mgr != nil {
		r.mgr.broadcastAudioFrame(ChannelOutgoing, out)
	}

	return out, nil
}

func (r *radioAudioMixer) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.musicSrc != nil {
		_ = r.musicSrc.Close()
		r.musicSrc = nil
	}
	r.musicTrack = nil
	r.onMusicFinish = nil
	// Drain mic queue
	for len(r.micQueue) > 0 {
		<-r.micQueue
	}
	return nil
}

func (r *radioAudioMixer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.musicSrc != nil {
		_ = r.musicSrc.Close()
		r.musicSrc = nil
	}
	r.musicTrack = nil
	r.musicPaused = false
	r.micActive = false
	r.currentDucking = 1.0
	for len(r.micQueue) > 0 {
		<-r.micQueue
	}
}

func (r *radioAudioMixer) SetMusicSource(src meowcaller.AudioSource, track *Track, onFinish func()) {
	r.mu.Lock()
	old := r.musicSrc
	r.musicSrc = src
	r.musicTrack = track
	r.onMusicFinish = onFinish
	r.musicPaused = false
	r.mu.Unlock()

	if old != nil && old != src {
		_ = old.Close()
	}
}

func (r *radioAudioMixer) ClearMusicSource() {
	r.mu.Lock()
	old := r.musicSrc
	r.musicSrc = nil
	r.musicTrack = nil
	r.musicPaused = false
	r.mu.Unlock()

	if old != nil {
		_ = old.Close()
	}
}

func (r *radioAudioMixer) PauseMusic() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.musicPaused = true
}

func (r *radioAudioMixer) ResumeMusic() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.musicPaused = false
}

func (r *radioAudioMixer) SetMicActive(active bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.micActive = active
	if !active {
		// Clear leftover mic chunks when turning mic off
		for len(r.micQueue) > 0 {
			<-r.micQueue
		}
	}
}

func (r *radioAudioMixer) SetRadioMode(mode RadioMode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if mode == RadioModeMicOnly || mode == RadioModeDucking {
		r.radioMode = mode
	}
}

func (r *radioAudioMixer) SetDuckLevel(level float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if level < 0.0 {
		level = 0.0
	} else if level > 1.0 {
		level = 1.0
	}
	r.duckLevel = level
}

func (r *radioAudioMixer) SetMicGain(gain float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if gain < 0.0 {
		gain = 0.0
	} else if gain > 5.0 {
		gain = 5.0
	}
	r.micGain = gain
}

func (r *radioAudioMixer) PushMicFrame(frame []float32) {
	r.mu.Lock()
	active := r.micActive
	r.mu.Unlock()

	if !active || len(frame) == 0 {
		return
	}

	// Make copy to isolate memory from network buffer
	cp := make([]float32, len(frame))
	copy(cp, frame)

	select {
	case r.micQueue <- cp:
	default:
		// Queue full; discard oldest to maintain ~60ms low latency
		select {
		case <-r.micQueue:
		default:
		}
		select {
		case r.micQueue <- cp:
		default:
		}
	}
}

func (r *radioAudioMixer) Status() RadioStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	isSpeaking := r.micActive && time.Since(r.lastMicAudio) < 300*time.Millisecond
	return RadioStatus{
		MicActive:  r.micActive,
		Mode:       string(r.radioMode),
		DuckLevel:  r.duckLevel,
		MicGain:    r.micGain,
		IsSpeaking: isSpeaking,
	}
}
