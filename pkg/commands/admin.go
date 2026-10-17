package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"
)

// -----------------------------------------------------------------------------
// Admin & Moderation Stores
// -----------------------------------------------------------------------------

var (
	adminMu         sync.RWMutex
	mutedUsers      = make(map[string][]string)               // chatJID -> []senderJID
	welcomeSettings = make(map[string]*WelcomeConfig)         // groupJID -> Config
	chatStats       = make(map[string]map[string]*UserStat)   // groupJID -> (senderJID -> Stat)
	muteFile        = filepath.Join("data", "muted_users.json")
	welcomeFile     = filepath.Join("data", "welcome_settings.json")
	statsFile       = filepath.Join("data", "chat_stats.json")
)

type WelcomeConfig struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message"`
}

type UserStat struct {
	Count    int       `json:"count"`
	LastSeen time.Time `json:"lastSeen"`
}

func init() {
	_ = os.MkdirAll("data", 0755)
	loadAdminStores()
}

func loadAdminStores() {
	adminMu.Lock()
	defer adminMu.Unlock()

	if data, err := os.ReadFile(muteFile); err == nil {
		_ = json.Unmarshal(data, &mutedUsers)
	}
	if data, err := os.ReadFile(welcomeFile); err == nil {
		_ = json.Unmarshal(data, &welcomeSettings)
	}
	if data, err := os.ReadFile(statsFile); err == nil {
		_ = json.Unmarshal(data, &chatStats)
	}
}

func RegisterAdminCommands(r *CommandRegistry) {
	// Mute
	r.RegisterPrefix("!mute", handleMute)
	r.RegisterPrefix("!unmute", handleUnmute)
	r.RegisterPrefix("!mutelist", handleMuteList)

	// Welcome
	r.RegisterPrefix("!welcome", handleWelcome)
	r.RegisterPrefix("!setwelcome", handleSetWelcome)

	// Stats
	r.RegisterPrefix("!stats", handleStats)
	r.RegisterPrefix("!leaderboard", handleLeaderboard)
	r.RegisterPrefix("!top", handleLeaderboard)
}

// -----------------------------------------------------------------------------
// 1. MUTE SYSTEM & HELPERS
// -----------------------------------------------------------------------------

func normalizeJID(jid string) string {
	s := strings.TrimSpace(jid)
	s = strings.TrimPrefix(s, "@")
	if s == "" {
		return ""
	}
	if parsed, err := types.ParseJID(s); err == nil && parsed.User != "" {
		return parsed.ToNonAD().String()
	}
	// Extract digits for phone numbers without server
	var cleanDigits strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cleanDigits.WriteRune(r)
		}
	}
	if cleanDigits.Len() > 0 {
		return cleanDigits.String() + "@s.whatsapp.net"
	}
	return s
}

func normalizeChatJID(chatJID string) string {
	s := strings.TrimSpace(chatJID)
	if parsed, err := types.ParseJID(s); err == nil && parsed.User != "" {
		return parsed.ToNonAD().String()
	}
	return s
}

func extractUser(jid string) string {
	s := strings.TrimPrefix(strings.TrimSpace(jid), "@")
	if at := strings.Index(s, "@"); at != -1 {
		s = s[:at]
	}
	if colon := strings.Index(s, ":"); colon != -1 {
		s = s[:colon]
	}
	return s
}

func getContextInfo(msg *waProto.Message) *waProto.ContextInfo {
	if msg == nil {
		return nil
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetContextInfo()
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetContextInfo()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetContextInfo()
	}
	if aud := msg.GetAudioMessage(); aud != nil {
		return aud.GetContextInfo()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetContextInfo()
	}
	if stk := msg.GetStickerMessage(); stk != nil {
		return stk.GetContextInfo()
	}
	return nil
}

func extractTargetUser(c *CommandContext) string {
	// 1. ContextInfo mentions or quoted participant
	if c.RawMsg != nil && c.RawMsg.Message != nil {
		ctxInfo := getContextInfo(c.RawMsg.Message)
		if ctxInfo != nil {
			if len(ctxInfo.GetMentionedJID()) > 0 && ctxInfo.GetMentionedJID()[0] != "" {
				return normalizeJID(ctxInfo.GetMentionedJID()[0])
			}
			if quoted := ctxInfo.GetParticipant(); quoted != "" {
				return normalizeJID(quoted)
			}
		}
	}

	// 2. Command argument (e.g. !mute 919876543210 or !mute @919876543210 or !mute 219838068817963@lid)
	parts := strings.Fields(c.RawText)
	if len(parts) > 1 {
		targetArg := strings.TrimSpace(parts[1])
		if targetArg != "" {
			return normalizeJID(targetArg)
		}
	}

	return ""
}

func handleMute(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Mute commands can only be used in groups.")
	}

	if !c.IsSenderAdmin() {
		return true, c.ReplyText("Only group admins can use mute commands.")
	}

	target := extractTargetUser(c)
	if target == "" {
		return true, c.ReplyText("Mention, reply to, or provide a phone number: !mute @user")
	}

	// Prevent bot muting itself
	if c.Client != nil && c.Client.Store != nil && c.Client.Store.ID != nil {
		botUser := c.Client.Store.ID.ToNonAD().User
		if extractUser(target) == botUser {
			return true, c.ReplyText("I cannot mute myself!")
		}
	}

	chatKey := normalizeChatJID(c.ChatJID.String())

	// Check if target is already muted
	if IsUserMuted(chatKey, target) {
		return true, c.ReplyTextWithMentions(fmt.Sprintf("User @%s is already muted.", extractUser(target)), []string{target})
	}

	adminMu.Lock()
	mutedUsers[chatKey] = append(mutedUsers[chatKey], target)
	saveMutedUsers()
	adminMu.Unlock()

	return true, c.ReplyTextWithMentions(fmt.Sprintf("🤫 Muted @%s in this group. All their future messages will be automatically deleted.", extractUser(target)), []string{target})
}

func handleUnmute(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Mute commands can only be used in groups.")
	}

	if !c.IsSenderAdmin() {
		return true, c.ReplyText("Only group admins can use mute commands.")
	}

	target := extractTargetUser(c)
	if target == "" {
		return true, c.ReplyText("Mention, reply to, or provide a phone number: !unmute @user")
	}

	chatKey := normalizeChatJID(c.ChatJID.String())
	targetClean := normalizeJID(target)
	targetUser := extractUser(target)

	adminMu.Lock()
	list := mutedUsers[chatKey]
	newList := make([]string, 0, len(list))
	found := false
	for _, u := range list {
		if normalizeJID(u) == targetClean || extractUser(u) == targetUser {
			found = true
		} else {
			newList = append(newList, u)
		}
	}
	if found {
		if len(newList) == 0 {
			delete(mutedUsers, chatKey)
		} else {
			mutedUsers[chatKey] = newList
		}
		saveMutedUsers()
	}
	adminMu.Unlock()

	if !found {
		return true, c.ReplyTextWithMentions(fmt.Sprintf("User @%s is not muted.", targetUser), []string{target})
	}
	return true, c.ReplyTextWithMentions(fmt.Sprintf("🔊 Unmuted @%s. They can now send messages again.", targetUser), []string{target})
}

func handleMuteList(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Mute commands can only be used in groups.")
	}

	chatKey := normalizeChatJID(c.ChatJID.String())
	adminMu.RLock()
	list := mutedUsers[chatKey]
	adminMu.RUnlock()

	if len(list) == 0 {
		return true, c.ReplyText("No users are currently muted in this chat.")
	}

	var sb strings.Builder
	mentions := make([]string, 0, len(list))
	sb.WriteString(fmt.Sprintf("🤫 *Muted Users in this Group (%d):*\n\n", len(list)))
	for i, u := range list {
		sb.WriteString(fmt.Sprintf("%d. @%s\n", i+1, extractUser(u)))
		mentions = append(mentions, normalizeJID(u))
	}
	return true, c.ReplyTextWithMentions(strings.TrimSpace(sb.String()), mentions)
}

func IsUserMuted(chatJID, senderJID string, senderAltJID ...string) bool {
	adminMu.RLock()
	defer adminMu.RUnlock()

	chatKey := normalizeChatJID(chatJID)
	list, exists := mutedUsers[chatKey]
	if !exists || len(list) == 0 {
		return false
	}

	senderClean := normalizeJID(senderJID)
	senderUser := extractUser(senderJID)

	var altClean, altUser string
	if len(senderAltJID) > 0 && senderAltJID[0] != "" {
		altClean = normalizeJID(senderAltJID[0])
		altUser = extractUser(senderAltJID[0])
	}

	for _, u := range list {
		uClean := normalizeJID(u)
		uUser := extractUser(u)

		if uClean == senderClean || (altClean != "" && uClean == altClean) {
			return true
		}
		if uUser != "" && (uUser == senderUser || (altUser != "" && uUser == altUser)) {
			return true
		}
	}
	return false
}

func saveMutedUsers() {
	if data, err := json.MarshalIndent(mutedUsers, "", "  "); err == nil {
		_ = os.WriteFile(muteFile, data, 0644)
	}
}

// -----------------------------------------------------------------------------
// 2. WELCOME SYSTEM
// -----------------------------------------------------------------------------

func handleWelcome(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Welcome settings can only be configured in groups.")
	}
	if !c.IsSenderAdmin() {
		return true, c.ReplyText("Only group admins can configure welcome messages.")
	}

	parts := strings.Fields(strings.ToLower(c.RawText))
	chatKey := c.ChatJID.String()

	adminMu.Lock()
	cfg, ok := welcomeSettings[chatKey]
	if !ok {
		cfg = &WelcomeConfig{
			Enabled: true,
			Message: "Welcome @user to the group!",
		}
		welcomeSettings[chatKey] = cfg
	}

	if len(parts) >= 2 {
		switch parts[1] {
		case "on", "enable":
			cfg.Enabled = true
			saveWelcomeSettings()
			adminMu.Unlock()
			return true, c.ReplyText("Welcome messages enabled for this group.")
		case "off", "disable":
			cfg.Enabled = false
			saveWelcomeSettings()
			adminMu.Unlock()
			return true, c.ReplyText("Welcome messages disabled for this group.")
		}
	}

	enabledStr := "disabled"
	if cfg.Enabled {
		enabledStr = "enabled"
	}
	adminMu.Unlock()

	return true, c.ReplyText(fmt.Sprintf("Welcome status: %s\nTemplate: \"%s\"\n\nUse !welcome on|off or !setwelcome <message>", enabledStr, cfg.Message))
}

func handleSetWelcome(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Welcome settings can only be configured in groups.")
	}
	if !c.IsSenderAdmin() {
		return true, c.ReplyText("Only group admins can set welcome templates.")
	}

	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !setwelcome <welcome message>\n(Use @user to mention the new member)")
	}

	msg := strings.TrimSpace(c.RawText[len(parts[0]):])
	chatKey := c.ChatJID.String()

	adminMu.Lock()
	cfg, ok := welcomeSettings[chatKey]
	if !ok {
		cfg = &WelcomeConfig{Enabled: true}
		welcomeSettings[chatKey] = cfg
	}
	cfg.Message = msg
	cfg.Enabled = true
	saveWelcomeSettings()
	adminMu.Unlock()

	return true, c.ReplyText("Welcome template updated.")
}

func TriggerGroupWelcome(client *whatsmeow.Client, groupJID types.JID, newParticipants []string) {
	chatKey := groupJID.String()
	adminMu.RLock()
	cfg, ok := welcomeSettings[chatKey]
	if !ok || !cfg.Enabled || cfg.Message == "" {
		adminMu.RUnlock()
		return
	}
	msgTemplate := cfg.Message
	adminMu.RUnlock()

	for _, user := range newParticipants {
		cleanUser := strings.Split(user, "@")[0]
		text := strings.ReplaceAll(msgTemplate, "@user", "@"+cleanUser)

		msg := &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: proto.String(text),
				ContextInfo: &waProto.ContextInfo{
					MentionedJID: []string{user},
				},
			},
		}
		_, _ = client.SendMessage(context.Background(), groupJID, msg)
	}
}

func saveWelcomeSettings() {
	if data, err := json.MarshalIndent(welcomeSettings, "", "  "); err == nil {
		_ = os.WriteFile(welcomeFile, data, 0644)
	}
}

// -----------------------------------------------------------------------------
// 3. STATS & LEADERBOARD
// -----------------------------------------------------------------------------

func TrackMessageStats(chatJID, senderJID string) {
	if chatJID == "" || senderJID == "" {
		return
	}

	adminMu.Lock()
	defer adminMu.Unlock()

	chat, ok := chatStats[chatJID]
	if !ok {
		chat = make(map[string]*UserStat)
		chatStats[chatJID] = chat
	}

	user, ok := chat[senderJID]
	if !ok {
		user = &UserStat{Count: 0, LastSeen: time.Now()}
		chat[senderJID] = user
	}

	user.Count++
	user.LastSeen = time.Now()
}

func saveChatStats() {
	if data, err := json.MarshalIndent(chatStats, "", "  "); err == nil {
		_ = os.WriteFile(statsFile, data, 0644)
	}
}

func handleStats(c *CommandContext) (bool, error) {
	chatKey := c.ChatJID.String()
	senderKey := c.SenderJID.String()

	adminMu.RLock()
	chat := chatStats[chatKey]
	totalMsgs := 0
	senderCount := 0
	if chat != nil {
		for _, stat := range chat {
			totalMsgs += stat.Count
		}
		if s, ok := chat[senderKey]; ok {
			senderCount = s.Count
		}
	}
	adminMu.RUnlock()

	text := fmt.Sprintf("*Chat Stats*\n- Total messages tracked: %d\n- Your messages: %d\n- Active participants: %d",
		totalMsgs, senderCount, len(chat))
	return true, c.ReplyText(text)
}

func handleLeaderboard(c *CommandContext) (bool, error) {
	chatKey := c.ChatJID.String()

	adminMu.RLock()
	chat := chatStats[chatKey]
	if len(chat) == 0 {
		adminMu.RUnlock()
		return true, c.ReplyText("No message stats recorded for this chat yet.")
	}

	type entry struct {
		jid   string
		count int
	}
	list := make([]entry, 0, len(chat))
	for j, s := range chat {
		list = append(list, entry{jid: j, count: s.Count})
	}
	adminMu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].count > list[j].count
	})

	if len(list) > 10 {
		list = list[:10]
	}

	var sb strings.Builder
	sb.WriteString("*Top Chatters*\n")
	for i, e := range list {
		sb.WriteString(fmt.Sprintf("%d. @%s - %d messages\n", i+1, strings.Split(e.jid, "@")[0], e.count))
	}
	return true, c.ReplyText(strings.TrimSpace(sb.String()))
}

func GetAllChatStats() map[string]map[string]*UserStat {
	adminMu.RLock()
	defer adminMu.RUnlock()
	res := make(map[string]map[string]*UserStat)
	for k, v := range chatStats {
		sub := make(map[string]*UserStat)
		for sk, sv := range v {
			sub[sk] = &UserStat{Count: sv.Count, LastSeen: sv.LastSeen}
		}
		res[k] = sub
	}
	return res
}

func ResetChatStatsForJID(jid string) {
	adminMu.Lock()
	defer adminMu.Unlock()
	delete(chatStats, jid)
	saveChatStats()
}

func GetAllWelcomeSettings() map[string]*WelcomeConfig {
	adminMu.RLock()
	defer adminMu.RUnlock()
	res := make(map[string]*WelcomeConfig)
	for k, v := range welcomeSettings {
		res[k] = &WelcomeConfig{Enabled: v.Enabled, Message: v.Message}
	}
	return res
}

func SetWelcomeConfigForJID(jid string, enabled bool, message string) {
	adminMu.Lock()
	defer adminMu.Unlock()
	welcomeSettings[jid] = &WelcomeConfig{
		Enabled: enabled,
		Message: message,
	}
	saveWelcomeSettings()
}

func GetAllMutedUsers() map[string][]string {
	adminMu.RLock()
	defer adminMu.RUnlock()
	res := make(map[string][]string)
	for k, v := range mutedUsers {
		copied := make([]string, len(v))
		copy(copied, v)
		res[k] = copied
	}
	return res
}

func AddMutedUser(chatJID, userJID string) {
	adminMu.Lock()
	defer adminMu.Unlock()
	chatKey := normalizeChatJID(chatJID)
	userClean := normalizeJID(userJID)
	userExtract := extractUser(userJID)
	list := mutedUsers[chatKey]
	for _, u := range list {
		if normalizeJID(u) == userClean || extractUser(u) == userExtract {
			return
		}
	}
	mutedUsers[chatKey] = append(mutedUsers[chatKey], userClean)
	saveMutedUsers()
}

func RemoveMutedUser(chatJID, userJID string) {
	adminMu.Lock()
	defer adminMu.Unlock()
	chatKey := normalizeChatJID(chatJID)
	targetClean := normalizeJID(userJID)
	targetUser := extractUser(userJID)
	list := mutedUsers[chatKey]
	newList := make([]string, 0, len(list))
	for _, u := range list {
		if normalizeJID(u) != targetClean && extractUser(u) != targetUser {
			newList = append(newList, u)
		}
	}
	if len(newList) == 0 {
		delete(mutedUsers, chatKey)
	} else {
		mutedUsers[chatKey] = newList
	}
	saveMutedUsers()
}
