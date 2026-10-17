package commands

import (
	"context"
	"strings"
	"sync"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"github.com/polymorfa/hypermeow/types/events"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/store"
)

type CommandContext struct {
	Client    *whatsmeow.Client
	ChatJID   types.JID
	SenderJID types.JID
	IsGroup   bool
	RawText   string
	MsgID     string
	RawMsg    *events.Message
	Ctx       context.Context
}

type CommandHandler func(c *CommandContext) (bool, error)

type RegistryEntry struct {
	Prefix  string
	Match   func(text string) bool
	Handler CommandHandler
}

type CommandRegistry struct {
	mu             sync.RWMutex
	prefixMap      map[string]CommandHandler
	customHandlers []RegistryEntry
	handlers       []RegistryEntry
}

var GlobalRegistry *CommandRegistry

func InitRegistry() *CommandRegistry {
	r := &CommandRegistry{
		prefixMap:      make(map[string]CommandHandler),
		customHandlers: make([]RegistryEntry, 0, 8),
		handlers:       make([]RegistryEntry, 0, 128),
	}
	GlobalRegistry = r
	return r
}

func (r *CommandRegistry) Register(matcher func(text string) bool, handler CommandHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := RegistryEntry{
		Prefix:  "",
		Match:   matcher,
		Handler: handler,
	}
	r.customHandlers = append(r.customHandlers, entry)
	r.handlers = append(r.handlers, entry)
}

func matchCommandToken(text, prefix string) bool {
	lowerText := strings.ToLower(strings.TrimSpace(text))
	lowerPrefix := strings.ToLower(strings.TrimSpace(prefix))

	if lowerText == lowerPrefix {
		return true
	}

	if strings.HasPrefix(lowerText, lowerPrefix) {
		remainder := lowerText[len(lowerPrefix):]
		if len(remainder) == 0 {
			return true
		}
		firstRune := rune(remainder[0])
		if firstRune == ' ' || firstRune == '\t' || firstRune == '\n' || firstRune == '\r' || firstRune == ':' || firstRune == '|' || firstRune == ',' || firstRune == '?' {
			return true
		}
	}
	return false
}

func (r *CommandRegistry) RegisterPrefix(prefix string, handler CommandHandler) {
	lowerPrefix := strings.ToLower(strings.TrimSpace(prefix))
	if lowerPrefix == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := RegistryEntry{
		Prefix: lowerPrefix,
		Match: func(text string) bool {
			return matchCommandToken(text, lowerPrefix)
		},
		Handler: handler,
	}
	r.handlers = append(r.handlers, entry)

	// If prefix contains space or delimiter, place in custom fallback handlers
	if strings.ContainsAny(lowerPrefix, " \t\r\n:|?") {
		r.customHandlers = append(r.customHandlers, entry)
		return
	}

	// Single token prefix: direct O(1) map registration
	r.prefixMap[lowerPrefix] = handler
}

// isCommandDelimiter returns true if byte b terminates a command token
func isCommandDelimiter(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ':' || b == '|' || b == ',' || b == '?'
}

func (r *CommandRegistry) Dispatch(c *CommandContext) (bool, error) {
	raw := c.RawText
	n := len(raw)
	if n == 0 {
		return false, nil
	}

	// 1. Skip leading whitespace with zero allocations
	start := 0
	for start < n && (raw[start] == ' ' || raw[start] == '\t' || raw[start] == '\n' || raw[start] == '\r') {
		start++
	}
	if start >= n {
		return false, nil
	}

	// 2. Scan command token up to first delimiter
	end := start
	for end < n && !isCommandDelimiter(raw[end]) {
		end++
	}
	token := raw[start:end]

	// 3. Fast O(1) map lookup
	lowerToken := strings.ToLower(token)
	r.mu.RLock()
	handler, found := r.prefixMap[lowerToken]
	r.mu.RUnlock()

	if found && handler != nil {
		handled, err := handler(c)
		if handled || err != nil {
			return handled, err
		}
	}

	// 4. Fallback to custom/multi-token handlers
	r.mu.RLock()
	customs := r.customHandlers
	r.mu.RUnlock()

	if len(customs) > 0 {
		text := strings.TrimSpace(raw)
		for _, entry := range customs {
			if entry.Match(text) {
				handled, err := entry.Handler(c)
				if handled || err != nil {
					return handled, err
				}
			}
		}
	}

	return false, nil
}

// Reply Helpers
func (c *CommandContext) ReplyText(text string) error {
	if c.Client == nil {
		return nil
	}
	var ctxInfo *waProto.ContextInfo
	if c.MsgID != "" && c.RawMsg != nil && c.RawMsg.Message != nil {
		ctxInfo = &waProto.ContextInfo{
			StanzaID:      proto.String(c.MsgID),
			QuotedMessage: c.RawMsg.Message,
		}
		if c.IsGroup {
			ctxInfo.Participant = proto.String(c.SenderJID.String())
		}
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        proto.String(text),
			ContextInfo: ctxInfo,
		},
	}
	resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err != nil {
		return c.SendText(text)
	}
	if resp.ID != "" {
		_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), text)
	}
	return nil
}

func (c *CommandContext) SendText(text string) error {
	if c.Client == nil {
		return nil
	}
	msg := &waProto.Message{
		Conversation: proto.String(text),
	}
	resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err == nil && resp.ID != "" {
		_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), text)
	}
	return err
}


func (c *CommandContext) ReplyTextWithMentions(text string, mentions []string) error {
	var ctxInfo *waProto.ContextInfo
	if c.MsgID != "" && c.RawMsg != nil && c.RawMsg.Message != nil {
		ctxInfo = &waProto.ContextInfo{
			StanzaID:      proto.String(c.MsgID),
			QuotedMessage: c.RawMsg.Message,
			MentionedJID:  mentions,
		}
		if c.IsGroup {
			ctxInfo.Participant = proto.String(c.SenderJID.String())
		}
	} else if len(mentions) > 0 {
		ctxInfo = &waProto.ContextInfo{
			MentionedJID: mentions,
		}
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        proto.String(text),
			ContextInfo: ctxInfo,
		},
	}
	resp, err := c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	if err != nil {
		return c.SendText(text)
	}
	if resp.ID != "" {
		_ = store.RecordBotMessage(resp.ID, c.ChatJID.String(), text)
	}
	return nil
}

func (c *CommandContext) IsSenderOwner() bool {
	if c.RawMsg != nil && c.RawMsg.Info.IsFromMe {
		return true
	}
	return config.AppConfig.IsOwner(c.SenderJID.String())
}

func (c *CommandContext) IsSenderAdmin() bool {
	if c.IsSenderOwner() {
		return true
	}
	if !c.IsGroup || c.Client == nil {
		return false
	}
	groupInfo, err := c.Client.GetGroupInfo(c.Ctx, c.ChatJID)
	if err != nil {
		return false
	}
	senderClean := c.SenderJID.ToNonAD().String()
	senderUser := c.SenderJID.ToNonAD().User
	for _, p := range groupInfo.Participants {
		pClean := p.JID.ToNonAD().String()
		pUser := p.JID.ToNonAD().User
		if (pClean == senderClean || pUser == senderUser) && (p.IsAdmin || p.IsSuperAdmin) {
			return true
		}
	}
	return false
}

func (c *CommandContext) IsChatSleeping() bool {
	return store.GlobalStore.IsSleeping()
}

// React sends an emoji reaction to the current command message
func (c *CommandContext) React(emoji string) error {
	if c.Client == nil || c.MsgID == "" {
		return nil
	}
	targetSender := c.SenderJID
	if c.RawMsg != nil && c.RawMsg.Info.IsFromMe && c.Client.Store != nil && c.Client.Store.ID != nil {
		targetSender = c.Client.Store.ID.ToNonAD()
	} else if !c.IsGroup {
		targetSender = c.ChatJID
	}
	reactionMsg := c.Client.BuildReaction(c.ChatJID, targetSender, c.MsgID, emoji)
	_, err := c.Client.SendMessage(context.Background(), c.ChatJID, reactionMsg)
	return err
}

// ReactToQuoted sends an emoji reaction to the quoted message if present, or to the current message
func (c *CommandContext) ReactToQuoted(emoji string) error {
	if c.Client == nil || c.RawMsg == nil || c.RawMsg.Message == nil {
		return nil
	}
	var ctxInfo *waProto.ContextInfo
	if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
		ctxInfo = ext.GetContextInfo()
	}
	if ctxInfo != nil && ctxInfo.StanzaID != nil && *ctxInfo.StanzaID != "" {
		quotedSender := c.SenderJID
		if ctxInfo.Participant != nil && *ctxInfo.Participant != "" {
			if parsed, err := types.ParseJID(*ctxInfo.Participant); err == nil {
				quotedSender = parsed
			}
		}
		reactionMsg := c.Client.BuildReaction(c.ChatJID, quotedSender, *ctxInfo.StanzaID, emoji)
		_, err := c.Client.SendMessage(context.Background(), c.ChatJID, reactionMsg)
		return err
	}
	return c.React(emoji)
}

