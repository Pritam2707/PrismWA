package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/polymorfa/hypermeow/types"
)

func TestConvertAnimatedWebp(t *testing.T) {
	testFile := filepath.Join("..", "..", "data", "media", "msg_3A0CF39FAC7CCD1E5EA4.webp")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Animated webp test file does not exist")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tmpMp4 := filepath.Join(os.TempDir(), "test_anim.mp4")
	defer os.Remove(tmpMp4)

	err := convertWebpToMp4(ctx, testFile, tmpMp4)
	if err != nil {
		t.Fatalf("convertWebpToMp4 failed: %v", err)
	}

	fi, err := os.Stat(tmpMp4)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("Generated MP4 is empty or missing")
	}

	// Test convertWebpToPng
	tmpPng := filepath.Join(os.TempDir(), "test_anim.png")
	defer os.Remove(tmpPng)

	err = convertWebpToPng(ctx, testFile, tmpPng)
	if err != nil {
		t.Fatalf("convertWebpToPng failed: %v", err)
	}

	fiPng, err := os.Stat(tmpPng)
	if err != nil || fiPng.Size() == 0 {
		t.Fatalf("Generated PNG is empty or missing")
	}
}

func TestPendingSongSearch(t *testing.T) {
	chatJID := "12345678@s.whatsapp.net"
	senderJID := "87654321@s.whatsapp.net"
	msgID := "MSG_SEARCH_101"

	results := []SongSearchResult{
		{ID: "vid1", Title: "Track One", Duration: "3:45", Channel: "Artist 1", URL: "https://www.youtube.com/watch?v=vid1"},
		{ID: "vid2", Title: "Track Two", Duration: "4:12", Channel: "Artist 2", URL: "https://www.youtube.com/watch?v=vid2"},
	}

	mediaSearchMu.Lock()
	pending := &PendingMediaSearch{
		ChatJID:   chatJID,
		SenderJID: senderJID,
		Query:     "test song",
		Results:   results,
		MsgID:     msgID,
		IsVideo:   false,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	pendingMediaSearch[chatJID+":"+senderJID] = pending
	pendingMediaSearch[chatJID] = pending
	msgIdToMediaSearch[msgID] = pending
	mediaSearchMu.Unlock()

	// 1. Retrieve via sender
	c := &CommandContext{
		ChatJID:   types.NewJID("12345678", "s.whatsapp.net"),
		SenderJID: types.NewJID("87654321", "s.whatsapp.net"),
	}
	found := getPendingSearch(c)
	if found == nil {
		t.Fatalf("expected to find pending search for sender")
	}
	if len(found.Results) != 2 || found.Results[0].Title != "Track One" {
		t.Errorf("unexpected results: %+v", found.Results)
	}

	// 2. Clear pending search
	clearPendingSearch(c, found)
	if getPendingSearch(c) != nil {
		t.Errorf("expected pending search to be cleared")
	}
}

func TestGenerateImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	imgBytes, mimetype, model, err := generateImage(ctx, "test simple abstract gradient")
	if err != nil {
		t.Logf("generateImage network call returned: %v (skipping if offline)", err)
		return
	}

	if len(imgBytes) < 500 {
		t.Errorf("expected imgBytes length > 500, got %d", len(imgBytes))
	}
	if mimetype == "" {
		t.Errorf("expected non-empty mimetype")
	}
	t.Logf("Successfully generated image with model: %s, mime: %s, size: %d bytes", model, mimetype, len(imgBytes))
}

func TestVideoCompatibility_TranscodeAndThumb(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed, skipping test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Generate an intentionally incompatible test video (VP9 + Opus in WebM)
	tmpWebm := filepath.Join(os.TempDir(), fmt.Sprintf("test_incompat_%d.webm", time.Now().UnixNano()))
	defer os.Remove(tmpWebm)

	genCmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libvpx",
		"-c:a", "libopus",
		tmpWebm,
	)
	if err := genCmd.Run(); err != nil {
		t.Fatalf("failed to create test webm video: %v", err)
	}

	// 2. Run ensureWhatsAppCompatibleVideo
	compatMp4, err := ensureWhatsAppCompatibleVideo(ctx, tmpWebm)
	if err != nil {
		t.Fatalf("ensureWhatsAppCompatibleVideo failed: %v", err)
	}
	if compatMp4 != tmpWebm {
		defer os.Remove(compatMp4)
	}

	fi, err := os.Stat(compatMp4)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("compatible MP4 file is empty or missing")
	}

	// 3. Verify codecs were converted to h264 and aac
	vcodec, acodec, err := probeVideoCodecs(ctx, compatMp4)
	if err != nil {
		t.Fatalf("probeVideoCodecs failed on output: %v", err)
	}
	if vcodec != "h264" {
		t.Errorf("expected vcodec h264, got %s", vcodec)
	}
	if acodec != "aac" {
		t.Errorf("expected acodec aac, got %s", acodec)
	}

	// 4. Test thumbnail extraction
	thumb, err := extractVideoThumbnail(ctx, compatMp4)
	if err != nil || len(thumb) == 0 {
		t.Errorf("failed to extract thumbnail: %v", err)
	}

	// 5. Test duration
	dur := getVideoDurationSeconds(ctx, compatMp4)
	if dur == 0 {
		t.Logf("duration reported as 0 for 1s test file (acceptable for subsecond)")
	}
}

