package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/polymorfa/hypermeow/types"
)

func TestChatHistorySinceUserLastMessage(t *testing.T) {
	ctx := context.Background()
	s := InitMessageStore(ctx, 200)

	chatJID := "123456789-group@g.us"
	userA := "111111111@s.whatsapp.net"
	userB := "222222222@s.whatsapp.net"
	userC := "333333333@s.whatsapp.net"

	// 1. UserA sends a message
	s.Put(&CachedMessage{
		ID:        "msg-0",
		ChatJID:   types.NewJID(chatJID, "g.us"),
		SenderJID: types.NewJID("111111111", "s.whatsapp.net"),
		PushName:  "User A",
		Text:      "I will be away for lunch",
		Timestamp: time.Now().Add(-30 * time.Minute),
	})

	// 2. 5 messages while UserA is away
	for i := 1; i <= 5; i++ {
		sender := userB
		if i%2 == 0 {
			sender = userC
		}
		s.Put(&CachedMessage{
			ID:        fmt.Sprintf("msg-%d", i),
			ChatJID:   types.NewJID(chatJID, "g.us"),
			SenderJID: types.NewJID(sender, "s.whatsapp.net"),
			PushName:  "Friend",
			Text:      fmt.Sprintf("Discussion message %d", i),
			Timestamp: time.Now().Add(-time.Duration(30-i) * time.Minute),
		})
	}

	// 3. UserA calls !catchup (trigger message is msg-6)
	history, isAway := s.GetChatHistorySinceUserLastMessage(chatJID, userA, "msg-6", 50, 100)

	if !isAway {
		t.Fatalf("expected isAway to be true, got false")
	}
	if len(history) != 5 {
		t.Fatalf("expected 5 messages while user was away, got %d", len(history))
	}

	// Check order: oldest first
	if history[0].ID != "msg-1" || history[4].ID != "msg-5" {
		t.Errorf("expected chronological order msg-1 to msg-5, got %s to %s", history[0].ID, history[4].ID)
	}

	// 4. Test GetRecentChatHistory with explicit count
	recent := s.GetRecentChatHistory(chatJID, 3, "msg-6")
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent messages, got %d", len(recent))
	}
	if recent[0].ID != "msg-3" || recent[2].ID != "msg-5" {
		t.Errorf("expected msg-3 to msg-5, got %s to %s", recent[0].ID, recent[2].ID)
	}
}
