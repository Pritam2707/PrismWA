package meowcaller

import (
	"testing"
	"time"
)

func TestMeowCallerEnableAndTemporaryWindow(t *testing.T) {
	mgr := GlobalManager

	// Should be disabled by default
	mgr.SetEnabled(false, 0)
	if mgr.IsEnabled() {
		t.Errorf("expected MeowCaller to be disabled by default")
	}

	// Enable indefinitely
	mgr.SetEnabled(true, 0)
	if !mgr.IsEnabled() {
		t.Errorf("expected MeowCaller to be enabled")
	}
	status := mgr.GetStatus()
	if !status.Enabled {
		t.Errorf("status.Enabled should be true")
	}
	if status.TempSecondsLeft != 0 {
		t.Errorf("expected 0 temp seconds left for indefinite mode, got %d", status.TempSecondsLeft)
	}

	// Enable with temporary duration (e.g. 1 minute)
	mgr.SetEnabled(true, 1)
	if !mgr.IsEnabled() {
		t.Errorf("expected MeowCaller to be enabled in temp mode")
	}
	status = mgr.GetStatus()
	if status.TempSecondsLeft <= 0 || status.TempSecondsLeft > 60 {
		t.Errorf("expected tempSecondsLeft around 60, got %d", status.TempSecondsLeft)
	}

	// Disable again
	mgr.SetEnabled(false, 0)
	if mgr.IsEnabled() {
		t.Errorf("expected MeowCaller to be disabled")
	}
}

func TestAutoAcceptToggle(t *testing.T) {
	mgr := GlobalManager
	mgr.SetAutoAcceptCalls(true)
	status := mgr.GetStatus()
	if !status.AutoAcceptCalls {
		t.Errorf("expected AutoAcceptCalls to be true")
	}

	mgr.SetAutoAcceptCalls(false)
	status = mgr.GetStatus()
	if status.AutoAcceptCalls {
		t.Errorf("expected AutoAcceptCalls to be false")
	}
}

func TestQueueOperations(t *testing.T) {
	mgr := GlobalManager
	mgr.StopParty()

	track1 := &Track{ID: "t1", Title: "Song 1", RequestedBy: "Alice", AddedAt: time.Now()}
	track2 := &Track{ID: "t2", Title: "Song 2", RequestedBy: "Bob", AddedAt: time.Now()}

	mgr.mu.Lock()
	mgr.queue = append(mgr.queue, track1, track2)
	mgr.mu.Unlock()

	status := mgr.GetStatus()
	if len(status.Queue) != 2 {
		t.Fatalf("expected queue length 2, got %d", len(status.Queue))
	}

	// Remove track 0
	removed, err := mgr.RemoveFromQueue(0)
	if err != nil {
		t.Fatalf("unexpected error removing track: %v", err)
	}
	if removed.ID != "t1" {
		t.Errorf("expected removed track t1, got %s", removed.ID)
	}

	status = mgr.GetStatus()
	if len(status.Queue) != 1 || status.Queue[0].ID != "t2" {
		t.Errorf("expected remaining track t2, got %+v", status.Queue)
	}

	// Cleanup
	mgr.StopParty()
}

func TestPendingCallErrors(t *testing.T) {
	mgr := GlobalManager
	mgr.mu.Lock()
	mgr.pendingCall = nil
	mgr.pendingCallPeer = ""
	mgr.mu.Unlock()

	if err := mgr.AnswerCall(); err == nil {
		t.Errorf("expected error answering with no pending call")
	}

	if err := mgr.RejectCall(); err == nil {
		t.Errorf("expected error rejecting with no pending call")
	}
}

func TestAudioSubscriptionAndBroadcast(t *testing.T) {
	mgr := GlobalManager
	subID, ch, unsub := mgr.SubscribeAudio(ChannelIncoming)
	defer unsub()

	if subID == "" {
		t.Fatalf("expected non-empty subscriber ID")
	}

	// Broadcast an incoming frame
	testFrame := []float32{0.5, -0.5, 0.0}
	mgr.broadcastAudioFrame(ChannelIncoming, testFrame)

	select {
	case packet := <-ch:
		if len(packet) != 1+len(testFrame)*2 {
			t.Fatalf("expected packet length %d, got %d", 1+len(testFrame)*2, len(packet))
		}
		if packet[0] != byte(ChannelIncoming) {
			t.Errorf("expected channel tag %d, got %d", ChannelIncoming, packet[0])
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for audio frame on subscriber channel")
	}
}
