package commands

import (
	"context"
	"fmt"
	"testing"
)

func TestCommandRegistryDispatch(t *testing.T) {
	r := InitRegistry()

	pingCalled := false
	r.RegisterPrefix("!ping", func(c *CommandContext) (bool, error) {
		pingCalled = true
		return true, nil
	})

	helpCalled := false
	r.RegisterPrefix("!help", func(c *CommandContext) (bool, error) {
		helpCalled = true
		return true, nil
	})

	numberCalled := false
	r.Register(func(text string) bool {
		return text == "42"
	}, func(c *CommandContext) (bool, error) {
		numberCalled = true
		return true, nil
	})

	// Test 1: Exact match with args
	c := &CommandContext{RawText: "!ping now", Ctx: context.Background()}
	handled, err := r.Dispatch(c)
	if !handled || err != nil || !pingCalled {
		t.Fatalf("expected !ping to be handled, got handled=%v, err=%v", handled, err)
	}

	// Test 2: Delimiter match
	c = &CommandContext{RawText: "!help:me", Ctx: context.Background()}
	handled, err = r.Dispatch(c)
	if !handled || err != nil || !helpCalled {
		t.Fatalf("expected !help:me to be handled, got handled=%v, err=%v", handled, err)
	}

	// Test 3: Custom fallback handler
	c = &CommandContext{RawText: "42", Ctx: context.Background()}
	handled, err = r.Dispatch(c)
	if !handled || err != nil || !numberCalled {
		t.Fatalf("expected custom matcher to be handled, got handled=%v, err=%v", handled, err)
	}

	// Test 4: Unhandled text
	c = &CommandContext{RawText: "random message in group", Ctx: context.Background()}
	handled, err = r.Dispatch(c)
	if handled || err != nil {
		t.Fatalf("expected random message to be unhandled, got handled=%v", handled)
	}
}

func BenchmarkCommandRegistryDispatch(b *testing.B) {
	r := InitRegistry()
	for i := 0; i < 150; i++ {
		prefix := fmt.Sprintf("!cmd%d", i)
		r.RegisterPrefix(prefix, func(c *CommandContext) (bool, error) {
			return true, nil
		})
	}

	cHit := &CommandContext{RawText: "!cmd149 payload arguments", Ctx: context.Background()}
	cMiss := &CommandContext{RawText: "just a normal chatter message in whatsapp group", Ctx: context.Background()}

	b.Run("Hit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.Dispatch(cHit)
		}
	})

	b.Run("Miss", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.Dispatch(cMiss)
		}
	})
}
