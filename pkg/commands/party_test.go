package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/polymorfa/hypermeow/types"
	"whatsapp-bot/pkg/meowcaller"
)

func TestPartyCommandsDisabledPrompt(t *testing.T) {
	mgr := meowcaller.GlobalManager
	mgr.SetEnabled(false, 0)

	registry := InitRegistry()
	RegisterPartyCommands(registry)

	var lastReply string
	mockContext := &CommandContext{
		ChatJID:   types.NewJID("1234567890", types.DefaultUserServer),
		SenderJID: types.NewJID("1234567890", types.DefaultUserServer),
		RawText:   "!party start blinding lights",
		Ctx:       context.Background(),
	}

	handled, err := registry.Dispatch(mockContext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected command to be handled")
	}

	_ = lastReply
}

func TestPartyHelpCommand(t *testing.T) {
	registry := InitRegistry()
	RegisterPartyCommands(registry)

	mockContext := &CommandContext{
		ChatJID:   types.NewJID("1234567890", types.DefaultUserServer),
		SenderJID: types.NewJID("1234567890", types.DefaultUserServer),
		RawText:   "!party help",
		Ctx:       context.Background(),
	}

	handled, err := registry.Dispatch(mockContext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected !party help to be handled")
	}
}

func TestPartyStatusWhenEnabled(t *testing.T) {
	mgr := meowcaller.GlobalManager
	mgr.SetEnabled(true, 30)
	defer mgr.SetEnabled(false, 0)

	registry := InitRegistry()
	RegisterPartyCommands(registry)

	mockContext := &CommandContext{
		ChatJID:   types.NewJID("1234567890", types.DefaultUserServer),
		SenderJID: types.NewJID("1234567890", types.DefaultUserServer),
		RawText:   "!party status",
		Ctx:       context.Background(),
	}

	handled, err := registry.Dispatch(mockContext)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected !party status to be handled")
	}

	status := mgr.GetStatus()
	if !status.Enabled {
		t.Errorf("expected manager to be enabled")
	}
	if status.TempSecondsLeft <= 0 {
		t.Errorf("expected temporary timer remaining")
	}
}

func TestIntentRouterPartyMatch(t *testing.T) {
	intent := FastPathIntent("song party shape of you", false, false, false)
	if intent == nil || intent.Intent != "party" {
		t.Fatalf("expected party intent, got %+v", intent)
	}
	if strings.TrimSpace(intent.Parameters["query"]) != "shape of you" {
		t.Errorf("expected query 'shape of you', got '%s'", intent.Parameters["query"])
	}

	intentStatus := FastPathIntent("party status", false, false, false)
	if intentStatus == nil || intentStatus.Intent != "party" {
		t.Fatalf("expected party status intent, got %+v", intentStatus)
	}
}

