package tts

import (
	"testing"
)

func TestGetAvailableVoices(t *testing.T) {
	voices := GetAvailableVoices()
	if len(voices) == 0 {
		t.Fatalf("expected available voices, got empty")
	}
}

