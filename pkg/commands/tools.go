package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/store"
)

var pollRegex = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

func RegisterToolsCommands(r *CommandRegistry) {
	r.RegisterPrefix("!poll", handlePoll)
	r.RegisterPrefix("!qr", handleQR)
	r.RegisterPrefix("!tagall", handleTagAll)
	r.RegisterPrefix("!everyone", handleTagAll)

	// Profile Picture (PFP / DP) Fetcher
	r.RegisterPrefix("!pfp", handleGetPFP)
	r.RegisterPrefix("!getpfp", handleGetPFP)
	r.RegisterPrefix("!dp", handleGetPFP)
	r.RegisterPrefix("!avatar", handleGetPFP)
	r.RegisterPrefix("!profilepic", handleGetPFP)
	r.RegisterPrefix("!grouppfp", handleGetGroupPFP)
	r.RegisterPrefix("!gpfp", handleGetGroupPFP)

	// Virtual Hierarchical Notes System
	r.RegisterPrefix("!notes", handleNotesCommand)
	r.RegisterPrefix("!tree", handleNotesCommand)
	r.RegisterPrefix("!ls", handleNotesCommand)
	r.RegisterPrefix("!dir", handleNotesCommand)
	r.RegisterPrefix("!cd", handleCdCommand)
	r.RegisterPrefix("!pwd", handlePwdCommand)
	r.RegisterPrefix("!getnote", handleGetNoteCommand)
	r.RegisterPrefix("!getnotes", handleGetNoteCommand)
	r.RegisterPrefix("!note", handleAddNoteCommand)
	r.RegisterPrefix("!addnote", handleAddNoteCommand)
	r.RegisterPrefix("!delnote", handleDelNoteCommand)
	r.RegisterPrefix("!rmnote", handleDelNoteCommand)
	r.RegisterPrefix("!deletenote", handleDelNoteCommand)

	r.RegisterPrefix("!autoreply", handleAutoReplyCommand)
	r.RegisterPrefix("!react", handleReactCommand)
	r.RegisterPrefix("!r", handleReactCommand)
}

func handlePoll(c *CommandContext) (bool, error) {
	raw := strings.TrimSpace(c.RawText)
	if len(raw) <= 5 {
		return true, c.ReplyText("Usage: !poll \"Question\" \"Option 1\" \"Option 2\" ...")
	}

	matches := pollRegex.FindAllStringSubmatch(raw, -1)

	items := make([]string, 0, len(matches))
	for _, m := range matches {
		for i := 1; i < len(m); i++ {
			if m[i] != "" {
				items = append(items, strings.TrimSpace(m[i]))
				break
			}
		}
	}

	if len(items) < 3 {
		return true, c.ReplyText("Provide a question and at least two options in quotes.")
	}

	question := items[0]
	options := items[1:]
	if len(options) > 12 {
		options = options[:12]
	}

	pollOptions := make([]*waProto.PollCreationMessage_Option, 0, len(options))
	for _, opt := range options {
		pollOptions = append(pollOptions, &waProto.PollCreationMessage_Option{
			OptionName: proto.String(opt),
		})
	}

	pollMsg := &waProto.Message{
		PollCreationMessage: &waProto.PollCreationMessage{
			Name:                   proto.String(question),
			Options:                pollOptions,
			SelectableOptionsCount: proto.Uint32(1),
		},
	}

	_, err := c.Client.SendMessage(c.Ctx, c.ChatJID, pollMsg)
	return true, err
}

func handleQR(c *CommandContext) (bool, error) {
	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: !qr <text or url>")
	}

	content := strings.Join(parts[1:], " ")
	pngBytes, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("QR generation failed: %v", err))
	}

	uploaded, err := c.Client.Upload(context.Background(), pngBytes, whatsmeow.MediaImage)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload image: %v", err))
	}

	caption := fmt.Sprintf("QR Code: %s", content)
	imageMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("image/png"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(pngBytes))),
			Caption:       proto.String(caption),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, imageMsg)
	return true, err
}

func handleTagAll(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("!tagall can only be used in group chats.")
	}

	groupInfo, err := c.Client.GetGroupInfo(c.Ctx, c.ChatJID)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to fetch group info: %v", err))
	}

	isAdmin := c.IsSenderOwner()
	senderClean := c.SenderJID.ToNonAD().String()
	for _, p := range groupInfo.Participants {
		if p.JID.ToNonAD().String() == senderClean && (p.IsAdmin || p.IsSuperAdmin) {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		return true, c.ReplyText("Only group admins can use !tagall.")
	}

	announcement := strings.TrimSpace(c.RawText)
	parts := strings.Fields(announcement)
	customMsg := "Attention all members"
	if len(parts) > 1 {
		customMsg = strings.Join(parts[1:], " ")
	}

	mentions := make([]string, 0, len(groupInfo.Participants))
	for _, p := range groupInfo.Participants {
		mentions = append(mentions, p.JID.String())
	}

	tagText := fmt.Sprintf("%s\n\n(%d members tagged)", customMsg, len(mentions))
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(tagText),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
			},
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, msg)
	return true, err
}


func handleAutoReplyCommand(c *CommandContext) (bool, error) {
	raw := strings.TrimSpace(c.RawText)
	parts := strings.Fields(raw)

	if len(parts) >= 2 && strings.ToLower(parts[1]) == "add" {
		rest := strings.TrimSpace(raw[len(parts[0])+len(parts[1])+1:])
		pipeParts := strings.Split(rest, "|")
		if len(pipeParts) < 2 {
			return true, c.ReplyText("Usage: !autoreply add <trigger> | <reply>")
		}

		trigger := strings.TrimSpace(pipeParts[0])
		response := strings.TrimSpace(pipeParts[1])
		store.GlobalPersistent.AddAutoReply(store.AutoReplyRule{
			Trigger:  trigger,
			Response: response,
			Exact:    false,
		})
		return true, c.ReplyText(fmt.Sprintf("Added auto-reply for trigger: %s", trigger))
	}

	rules := store.GlobalPersistent.GetAutoReplies()
	if len(rules) == 0 {
		return true, c.ReplyText("No auto-replies configured. Use !autoreply add <trigger> | <response>")
	}

	var sb strings.Builder
	sb.WriteString("Auto-replies:\n")
	for i, r := range rules {
		sb.WriteString(fmt.Sprintf("%d. %s -> %s\n", i+1, r.Trigger, r.Response))
	}
	return true, c.ReplyText(strings.TrimSpace(sb.String()))
}

func handleReactCommand(c *CommandContext) (bool, error) {
	parts := strings.Fields(strings.TrimSpace(c.RawText))
	if len(parts) < 2 {
		return true, c.ReplyText("Usage: Reply to any message with `!react <emoji>` (e.g. `!react 🔥`)")
	}
	emoji := parts[1]
	err := c.ReactToQuoted(emoji)
	return true, err
}

func handleGetGroupPFP(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Group profile picture can only be fetched in a group chat.")
	}
	c.RawText = "!pfp group"
	return handleGetPFP(c)
}

func handleGetPFP(c *CommandContext) (bool, error) {
	if c.Client == nil {
		return true, c.ReplyText("WhatsApp client not available.")
	}

	raw := strings.TrimSpace(c.RawText)
	fields := strings.Fields(raw)
	subArg := ""
	if len(fields) > 1 {
		subArg = strings.ToLower(fields[1])
	}

	var targetJID types.JID
	targetName := ""
	isGroupPic := false

	// 1. Group PFP explicitly requested
	if subArg == "group" || subArg == "g" || subArg == "-g" || (len(fields) > 0 && (strings.HasPrefix(strings.ToLower(fields[0]), "!grouppfp") || strings.HasPrefix(strings.ToLower(fields[0]), "!gpfp"))) {
		if !c.IsGroup {
			return true, c.ReplyText("Group profile picture can only be fetched in a group chat.")
		}
		targetJID = c.ChatJID
		isGroupPic = true
		targetName = "Group"
		if groupInfo, err := c.Client.GetGroupInfo(c.Ctx, c.ChatJID); err == nil && groupInfo != nil && groupInfo.Name != "" {
			targetName = groupInfo.Name
		}
	}

	// 2. Mentioned user in the message
	if targetJID.IsEmpty() && c.RawMsg != nil && c.RawMsg.Message != nil {
		var ctxInfo *waProto.ContextInfo
		if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
			ctxInfo = ext.GetContextInfo()
		}
		if ctxInfo != nil && len(ctxInfo.GetMentionedJID()) > 0 {
			for _, m := range ctxInfo.GetMentionedJID() {
				if parsed, err := types.ParseJID(m); err == nil && !parsed.IsEmpty() {
					targetJID = parsed.ToNonAD()
					break
				}
			}
		}
	}

	// 3. Quoted message participant
	if targetJID.IsEmpty() && c.RawMsg != nil && c.RawMsg.Message != nil {
		var ctxInfo *waProto.ContextInfo
		if ext := c.RawMsg.Message.GetExtendedTextMessage(); ext != nil {
			ctxInfo = ext.GetContextInfo()
		}
		if ctxInfo != nil && ctxInfo.Participant != nil && *ctxInfo.Participant != "" {
			if parsed, err := types.ParseJID(*ctxInfo.Participant); err == nil && !parsed.IsEmpty() {
				targetJID = parsed.ToNonAD()
			}
		}
	}

	// 4. Raw phone number or @user text in argument
	if targetJID.IsEmpty() && len(fields) > 1 && subArg != "group" && subArg != "g" && subArg != "-g" && subArg != "me" && subArg != "self" {
		cleaned := strings.TrimPrefix(fields[1], "@")
		cleaned = strings.TrimPrefix(cleaned, "+")
		allDigits := true
		for _, r := range cleaned {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits && len(cleaned) >= 7 {
			targetJID = types.NewJID(cleaned, "s.whatsapp.net")
		}
	}

	// 5. Direct Message (DM) default or self
	if targetJID.IsEmpty() {
		if subArg == "me" || subArg == "self" {
			targetJID = c.SenderJID.ToNonAD()
		} else if !c.IsGroup {
			targetJID = c.ChatJID.ToNonAD()
		} else {
			// In group with no arguments, no mentions, no quote: fetch the sender's own PFP
			targetJID = c.SenderJID.ToNonAD()
		}
	}

	if targetJID.IsEmpty() {
		return true, c.ReplyText("Could not determine user or group to fetch profile picture for.\nUsage: `!pfp @user`, reply to a message with `!pfp`, `!pfp group`, or `!pfp` in DM.")
	}

	_ = c.React("🖼️")

	params := &whatsmeow.GetProfilePictureParams{
		Preview: false,
	}
	if c.IsGroup && targetJID.Server != "g.us" {
		params.CommonGID = c.ChatJID
	}

	pfpInfo, err := c.Client.GetProfilePictureInfo(c.Ctx, targetJID, params)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		if errors.Is(err, whatsmeow.ErrProfilePictureNotSet) || strings.Contains(errStr, "not set") || strings.Contains(errStr, "404") {
			if isGroupPic {
				return true, c.ReplyText("This group does not have a profile picture.")
			}
			return true, c.ReplyText(fmt.Sprintf("@%s does not have a profile picture set.", targetJID.User))
		}
		if errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) || strings.Contains(errStr, "hidden") || strings.Contains(errStr, "401") {
			return true, c.ReplyText(fmt.Sprintf("@%s has hidden their profile picture due to privacy settings.", targetJID.User))
		}
		return true, c.ReplyText(fmt.Sprintf("Could not retrieve profile picture: %v", err))
	}

	if pfpInfo == nil || pfpInfo.URL == "" {
		return true, c.ReplyText("Profile picture URL not available.")
	}

	// Download image bytes via HTTP
	httpCtx, cancel := context.WithTimeout(c.Ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, "GET", pfpInfo.URL, nil)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to initialize download: %v", err))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to download profile picture: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return true, c.ReplyText(fmt.Sprintf("Profile picture download returned HTTP %d", resp.StatusCode))
	}

	imgBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(imgBytes) == 0 {
		return true, c.ReplyText("Failed to read profile picture data.")
	}

	uploaded, err := c.Client.Upload(c.Ctx, imgBytes, whatsmeow.MediaImage)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to upload image: %v", err))
	}

	var caption string
	var mentions []string
	if isGroupPic {
		caption = fmt.Sprintf("📸 *Profile Picture: %s*", targetName)
	} else {
		caption = fmt.Sprintf("📸 *Profile Picture of @%s*", targetJID.User)
		mentions = []string{targetJID.String()}
		if c.IsGroup && subArg == "" && c.RawMsg != nil && (c.RawMsg.Message.GetExtendedTextMessage() == nil || len(c.RawMsg.Message.GetExtendedTextMessage().GetContextInfo().GetMentionedJID()) == 0) {
			caption += "\n_Tip: Mention @user, reply to someone, or use !pfp group to get group picture._"
		}
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
	if len(mentions) > 0 {
		if ctxInfo == nil {
			ctxInfo = &waProto.ContextInfo{}
		}
		ctxInfo.MentionedJID = mentions
	}

	mimetype := http.DetectContentType(imgBytes)
	if !strings.HasPrefix(mimetype, "image/") {
		mimetype = "image/jpeg"
	}

	imageMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimetype),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(imgBytes))),
			Caption:       proto.String(caption),
			ContextInfo:   ctxInfo,
		},
	}

	_, sendErr := c.Client.SendMessage(c.Ctx, c.ChatJID, imageMsg)
	return true, sendErr
}
