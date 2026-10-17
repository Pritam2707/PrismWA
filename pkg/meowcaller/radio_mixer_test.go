package meowcaller

import (
	"io"
	"math"
	"testing"
	"time"
)

type dummyAudioSource struct {
	frame []float32
	eof   bool
}

func (d *dummyAudioSource) ReadFrame() ([]float32, error) {
	if d.eof {
		return nil, io.EOF
	}
	cp := make([]float32, len(d.frame))
	copy(cp, d.frame)
	return cp, nil
}

func (d *dummyAudioSource) Close() error {
	return nil
}

func TestRadioMixerSilenceWhenIdle(t *testing.T) {
	mixer := newRadioAudioMixer(nil)
	frame, err := mixer.ReadFrame()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frame) != 960 {
		t.Fatalf("expected 960 samples, got %d", len(frame))
	}
	for i, s := range frame {
		if s != 0.0 {
			t.Errorf("expected silence sample 0.0 at %d, got %f", i, s)
			break
		}
	}
}

func TestRadioMixerMicDucking(t *testing.T) {
	mixer := newRadioAudioMixer(nil)
	mixer.SetRadioMode(RadioModeDucking)
	mixer.SetDuckLevel(0.15)
	mixer.SetMicGain(1.2)
	mixer.SetMicActive(true)

	status := mixer.Status()
	if !status.MicActive {
		t.Errorf("expected MicActive to be true")
	}
	if status.Mode != "ducking" {
		t.Errorf("expected mode ducking, got %s", status.Mode)
	}

	// Push 960 mic samples of 0.5 amplitude
	micChunk := make([]float32, 960)
	for i := range micChunk {
		micChunk[i] = 0.5
	}
	mixer.PushMicFrame(micChunk)

	frame, err := mixer.ReadFrame()
	if err != nil {
		t.Fatalf("unexpected error reading frame: %v", err)
	}
	if len(frame) != 960 {
		t.Fatalf("expected 960 samples, got %d", len(frame))
	}

	// First sample should reflect mic input: 0.5 * 1.2 = 0.6
	expected := float32(0.6)
	if math.Abs(float64(frame[0]-expected)) > 0.01 {
		t.Errorf("expected sample ~%f, got %f", expected, frame[0])
	}

	// Turn off mic
	mixer.SetMicActive(false)
	status = mixer.Status()
	if status.MicActive {
		t.Errorf("expected MicActive to be false after toggle")
	}
}

func TestRadioMixerNeverReturnsEOFOnClose(t *testing.T) {
	mixer := newRadioAudioMixer(nil)

	// Simulate Player.Play / Player.Stop calling Close() on the mixer
	if err := mixer.Close(); err != nil {
		t.Fatalf("unexpected Close error: %v", err)
	}

	// Mixer must still return valid silence frame, NOT io.EOF
	frame, err := mixer.ReadFrame()
	if err != nil {
		t.Fatalf("mixer returned error after Close: %v (expected non-nil silence frame)", err)
	}
	if len(frame) != 960 {
		t.Fatalf("expected 960 samples, got %d", len(frame))
	}
}

func TestRadioMixerSourceFinishCallback(t *testing.T) {
	mixer := newRadioAudioMixer(nil)

	finished := make(chan bool, 1)
	src := &dummyAudioSource{
		frame: make([]float32, 960),
		eof:   false,
	}

	mixer.SetMusicSource(src, nil, func() {
		finished <- true
	})

	// First frame reads audio
	_, err := mixer.ReadFrame()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Next frame returns EOF from dummy source
	src.eof = true
	_, err = mixer.ReadFrame()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// onFinish callback must have fired
	select {
	case <-finished:
		// success
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected onFinish callback to fire when music source finishes")
	}
}

