package commands

import (
	"context"
	"testing"

	"github.com/polymorfa/hypermeow/types"
)

func TestPFPCommandRegistration(t *testing.T) {
	r := InitRegistry()
	RegisterToolsCommands(r)

	prefixes := []string{"!pfp", "!getpfp", "!dp", "!avatar", "!profilepic", "!grouppfp", "!gpfp"}
	for _, p := range prefixes {
		c := &CommandContext{
			RawText:   p,
			Ctx:       context.Background(),
			ChatJID:   types.NewJID("123456789", "s.whatsapp.net"),
			SenderJID: types.NewJID("123456789", "s.whatsapp.net"),
			IsGroup:   false,
		}
		handled, err := r.Dispatch(c)
		if !handled {
			t.Errorf("expected command %s to be handled by registry", p)
		}
		if err != nil {
			t.Errorf("unexpected error dispatching %s: %v", p, err)
		}
	}
}

func TestGroupPFPNotInGroup(t *testing.T) {
	r := InitRegistry()
	RegisterToolsCommands(r)

	c := &CommandContext{
		RawText:   "!grouppfp",
		Ctx:       context.Background(),
		ChatJID:   types.NewJID("123456789", "s.whatsapp.net"),
		SenderJID: types.NewJID("123456789", "s.whatsapp.net"),
		IsGroup:   false,
	}

	handled, err := r.Dispatch(c)
	if !handled {
		t.Fatalf("expected !grouppfp to be handled")
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
