package commands

import (
	"testing"
)

func TestMuteNormalizationAndMatching(t *testing.T) {
	chatKey := "120363429288396678@g.us"
	lidUser := "219838068817963@lid"
	phoneUser := "919876543210@s.whatsapp.net"

	// 1. Add muted user via LID
	AddMutedUser(chatKey, lidUser)

	// Should match raw LID
	if !IsUserMuted(chatKey, lidUser) {
		t.Errorf("Expected %s to be muted", lidUser)
	}

	// Should match companion device LID (e.g. :0, :1, :2)
	deviceLID := "219838068817963:0@lid"
	if !IsUserMuted(chatKey, deviceLID) {
		t.Errorf("Expected companion device %s to be muted", deviceLID)
	}

	deviceLID2 := "219838068817963:2@lid"
	if !IsUserMuted(chatKey, deviceLID2) {
		t.Errorf("Expected companion device %s to be muted", deviceLID2)
	}

	// Should match if alternate sender matches
	if !IsUserMuted(chatKey, "randomsender@lid", "219838068817963:0@lid") {
		t.Errorf("Expected alt sender to match muted LID")
	}

	// 2. Add muted user via phone number
	AddMutedUser(chatKey, phoneUser)

	// Should match phone with companion device
	devicePhone := "919876543210:1@s.whatsapp.net"
	if !IsUserMuted(chatKey, devicePhone) {
		t.Errorf("Expected companion phone %s to be muted", devicePhone)
	}

	// Clean up
	RemoveMutedUser(chatKey, lidUser)
	if IsUserMuted(chatKey, lidUser) {
		t.Errorf("Expected %s to be unmuted", lidUser)
	}

	RemoveMutedUser(chatKey, phoneUser)
	if IsUserMuted(chatKey, phoneUser) {
		t.Errorf("Expected %s to be unmuted", phoneUser)
	}
}

func TestNormalizeJID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"219838068817963:0@lid", "219838068817963@lid"},
		{"919876543210:1@s.whatsapp.net", "919876543210@s.whatsapp.net"},
		{"@919876543210", "919876543210@s.whatsapp.net"},
		{"919876543210", "919876543210@s.whatsapp.net"},
	}

	for _, tt := range tests {
		got := normalizeJID(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeJID(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractUser(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"219838068817963:0@lid", "219838068817963"},
		{"219838068817963@lid", "219838068817963"},
		{"@919876543210", "919876543210"},
		{"919876543210:2@s.whatsapp.net", "919876543210"},
	}

	for _, tt := range tests {
		got := extractUser(tt.input)
		if got != tt.expected {
			t.Errorf("extractUser(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}
