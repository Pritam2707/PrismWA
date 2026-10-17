package scheduler

import (
	"testing"
	"time"

	"whatsapp-bot/pkg/config"
)

func TestISTLocation(t *testing.T) {
	loc := config.GetLocation()
	if loc == nil {
		t.Fatalf("config.GetLocation() returned nil")
	}

	// Verify offset is +05:30 (19800 seconds)
	now := time.Now().In(loc)
	_, offset := now.Zone()
	expectedOffset := 5*3600 + 30*60 // 19800 seconds
	if offset != expectedOffset {
		t.Errorf("Expected IST offset %d seconds (+05:30), got %d", expectedOffset, offset)
	}
}

func TestNextMidnightIST(t *testing.T) {
	loc := config.GetLocation()

	// Case 1: Time is 2026-09-20 18:00:00 UTC, which is 23:30:00 IST on 2026-09-20
	t1 := time.Date(2026, 9, 20, 18, 0, 0, 0, time.UTC)
	nextMid := NextMidnight(t1, loc)

	// Next midnight in IST should be 2026-09-21 00:00:00 IST (which is 2026-09-20 18:30:00 UTC)
	expectedYear, expectedMonth, expectedDay := 2026, time.Month(9), 21
	if nextMid.Year() != expectedYear || nextMid.Month() != expectedMonth || nextMid.Day() != expectedDay {
		t.Errorf("Expected next midnight date %04d-%02d-%02d, got %04d-%02d-%02d",
			expectedYear, expectedMonth, expectedDay, nextMid.Year(), nextMid.Month(), nextMid.Day())
	}
	if nextMid.Hour() != 0 || nextMid.Minute() != 0 || nextMid.Second() != 0 {
		t.Errorf("Expected 00:00:00, got %02d:%02d:%02d", nextMid.Hour(), nextMid.Minute(), nextMid.Second())
	}

	// Delay between 23:30 IST and 00:00 IST should be exactly 30 minutes
	diff := nextMid.Sub(t1)
	if diff != 30*time.Minute {
		t.Errorf("Expected delay of 30m, got %v", diff)
	}

	// Case 2: Time is 2026-09-20 01:00:00 UTC, which is 06:30:00 IST on 2026-09-20
	t2 := time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)
	nextMid2 := NextMidnight(t2, loc)
	diff2 := nextMid2.Sub(t2)
	// From 06:30 IST to midnight is 17 hours 30 minutes
	expectedDiff2 := 17*time.Hour + 30*time.Minute
	if diff2 != expectedDiff2 {
		t.Errorf("Expected delay of %v, got %v", expectedDiff2, diff2)
	}
}
