package pfp

import (
	"bytes"
	"image/jpeg"
	"testing"

	"whatsapp-bot/pkg/config"
)

func TestGenerateDeadlinePFP(t *testing.T) {
	futureDate := config.Now().AddDate(0, 0, 10).Format("2006-01-02")
	data, err := GenerateDeadlinePFP("Final Examination", futureDate)
	if err != nil {
		t.Fatalf("GenerateDeadlinePFP failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("Expected non-empty JPEG data")
	}

	// Validate JPEG headers and decodability
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to decode generated JPEG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 640 || bounds.Dy() != 640 {
		t.Fatalf("Expected 640x640 dimensions, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateDeadlinePFP_InvalidDate(t *testing.T) {
	_, err := GenerateDeadlinePFP("Test", "invalid-date")
	if err == nil {
		t.Fatalf("Expected error for invalid date format")
	}
}
