package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"github.com/polymorfa/hypermeow/types/events"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/commands"
	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/pool"
	"whatsapp-bot/pkg/store"
)

func (w *WhatsAppService) RegisterEventHandler(registry *commands.CommandRegistry) {
	w.Client.AddEventHandler(func(rawEvt interface{}) {
		switch evt := rawEvt.(type) {
		case *events.Connected:
			w.SetConnectionState("open")
			fmt.Println("WhatsApp connected and synchronized.")

		case *events.Disconnected:
			w.SetConnectionState("disconnected")
			fmt.Println("Disconnected from WhatsApp. Attempting reconnect...")
			go func() {
				time.Sleep(2 * time.Second)
				if w.Client != nil && w.Client.Store.ID != nil && !w.Client.IsConnected() {
					if err := w.Client.Connect(); err != nil {
						fmt.Printf("Reconnect failed: %v\n", err)
					} else {
						w.SetConnectionState("open")
						fmt.Println("Reconnected to WhatsApp successfully.")
					}
				}
			}()

		case *events.LoggedOut:
			w.SetConnectionState("logged_out")
			fmt.Println("WhatsApp session logged out.")

		case *events.GroupInfo:
			if len(evt.Join) > 0 {
				joins := make([]string, len(evt.Join))
				for i, j := range evt.Join {
					joins[i] = j.String()
				}
				commands.TriggerGroupWelcome(w.Client, evt.JID, joins)
			}

		case *events.OfflineSyncPreview:
			w.SetCatchingUp(true)
			fmt.Printf("Syncing %d offline messages from WhatsApp...\n", evt.Messages)

		case *events.OfflineSyncCompleted:
			w.SetCatchingUp(false)
			fmt.Println("Offline sync completed. Live message processing active.")

		case *events.Message:
			w.handleIncomingMessage(evt, registry)
		}
	})
}

func (w *WhatsAppService) handleIncomingMessage(evt *events.Message, registry *commands.CommandRegistry) {
	// 1. Check for Revoke (Anti-Delete) FIRST before any IsFromMe filtering!
	if isRevokeMessage(evt) {
		w.handleAntiDelete(evt)
		return
	}

	chatJID := evt.Info.Chat
	senderJID := evt.Info.Sender
	msgID := evt.Info.ID
	isGroup := evt.Info.IsGroup
	pushName := evt.Info.PushName

	// Update contact cache
	if pushName != "" {
		store.GlobalStore.SetContact(senderJID.String(), pushName)
		if !isGroup {
			store.GlobalStore.SetContact(chatJID.String(), pushName)
		}
	}

	// 2. Ignore messages sent before bot started or delayed backlog messages
	if w.IsCatchingUp() {
		return
	}
	if !evt.Info.Timestamp.IsZero() {
		if evt.Info.Timestamp.Before(w.GetStartTime()) || time.Since(evt.Info.Timestamp) > 2*time.Minute {
			return
		}
	}

	// 3. Muted users filter (auto-delete message and drop if user is muted in this group)
	altJID := ""
	if !evt.Info.SenderAlt.IsEmpty() {
		altJID = evt.Info.SenderAlt.String()
	}
	if isGroup && commands.IsUserMuted(chatJID.String(), senderJID.String(), altJID) {
		if w.Client != nil {
			revokeMsg := w.Client.BuildRevoke(chatJID, senderJID, msgID)
			_, _ = w.Client.SendMessage(context.Background(), chatJID, revokeMsg)
		}
		return
	}

	rawText, mediaType := extractMessageInfo(evt)
	lowerText := strings.ToLower(rawText)

	isTrigger := strings.HasPrefix(rawText, "!") ||
		strings.HasPrefix(lowerText, "@prism") ||
		strings.HasPrefix(lowerText, "@bot") ||
		strings.Contains(lowerText, "@prism") ||
		strings.Contains(lowerText, "@bot")

	if w.Client != nil && w.Client.Store != nil && w.Client.Store.ID != nil {
		botUser := w.Client.Store.ID.ToNonAD().User
		if botUser != "" && (strings.HasPrefix(lowerText, "@"+botUser) || strings.Contains(lowerText, "@"+botUser)) {
			isTrigger = true
		}
	}

	// 4. Pre-cache media locally if present (for anti-delete & view-once recovery)
	localFilePath := ""
	if mediaType != "" {
		localFilePath = w.saveMediaLocally(evt, msgID)
	}

	// 5. Cache message for Anti-Delete & View-Once interceptor
	cached := &store.CachedMessage{
		ID:        msgID,
		ChatJID:   chatJID,
		SenderJID: senderJID,
		PushName:  pushName,
		Text:      rawText,
		MediaType: mediaType,
		Timestamp: evt.Info.Timestamp,
		Raw:       evt,
	}
	if localFilePath != "" {
		cached.MediaType = mediaType
	}
	store.GlobalStore.Put(cached)

	// Persist chat message for Persona style ("self") and message history across restarts
	if store.GlobalPersistent != nil && (rawText != "" || mediaType != "") {
		isBot := store.IsBotMessage(msgID)
		ts := evt.Info.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		_ = store.SaveChatMessage(&store.ChatMessageRecord{
			ID:         msgID,
			ChatJID:    chatJID.String(),
			SenderJID:  senderJID.String(),
			SenderName: pushName,
			Text:       rawText,
			IsFromMe:   evt.Info.IsFromMe,
			IsBot:      isBot,
			Timestamp:  ts,
		})
	}

	// Don't execute commands from self unless it's an explicit trigger
	if evt.Info.IsFromMe && !isTrigger {
		return
	}

	// 6. Track message activity stats
	commands.TrackMessageStats(chatJID.String(), senderJID.String())

	// 7. Drop silent processing if sleeping and not a wake command
	if store.GlobalStore.IsSleeping() {
		trimmed := strings.ToLower(strings.TrimSpace(rawText))
		if trimmed != "!wake" {
			return
		}
	} else {
		store.GlobalStore.TouchActivity()
	}

	// 7. Submit to Concurrent Worker Pool
	taskID := "msg-" + msgID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	pool.GlobalPool.Submit(pool.Task{
		ID: taskID,
		Exec: func(ctx context.Context) error {
			cmdCtx := &commands.CommandContext{
				Client:    w.Client,
				ChatJID:   chatJID,
				SenderJID: senderJID,
				IsGroup:   isGroup,
				RawText:   rawText,
				MsgID:     msgID,
				RawMsg:    evt,
				Ctx:       ctx,
			}

			// Handle registered prefix commands
			handled, err := registry.Dispatch(cmdCtx)
			if handled {
				if err != nil {
					fmt.Printf("Command error [%s]: %v\n", rawText, err)
				}
				return err
			}

			// Route to AI conversation if explicitly tagged/mentioned, or if AutoReplyDMs is enabled for private chats
			if isBotMentioned(evt, w.Client, lowerText) || (!isGroup && config.AppConfig.AutoReplyDMs) {
				handled, err = commands.HandleAIChat(cmdCtx)
				if handled {
					return err
				}
			}

			// Auto-Reply triggers check
			if w.checkAutoReply(cmdCtx, rawText) {
				return nil
			}

			return nil
		},
	})
}

var mediaDirOnce sync.Once

func (w *WhatsAppService) saveMediaLocally(evt *events.Message, msgID string) string {
	if evt.Message == nil || w.Client == nil {
		return ""
	}

	var target whatsmeow.DownloadableMessage
	var ext string

	if img := evt.Message.GetImageMessage(); img != nil {
		target = img
		ext = ".jpg"
	} else if vid := evt.Message.GetVideoMessage(); vid != nil {
		target = vid
		ext = ".mp4"
	} else if aud := evt.Message.GetAudioMessage(); aud != nil {
		target = aud
		ext = ".mp3"
	} else if stk := evt.Message.GetStickerMessage(); stk != nil {
		target = stk
		ext = ".webp"
	} else if vo := evt.Message.GetViewOnceMessage(); vo != nil && vo.GetMessage() != nil {
		if vImg := vo.GetMessage().GetImageMessage(); vImg != nil {
			target = vImg
			ext = ".jpg"
		} else if vVid := vo.GetMessage().GetVideoMessage(); vVid != nil {
			target = vVid
			ext = ".mp4"
		}
	} else if vo2 := evt.Message.GetViewOnceMessageV2(); vo2 != nil && vo2.GetMessage() != nil {
		if vImg := vo2.GetMessage().GetImageMessage(); vImg != nil {
			target = vImg
			ext = ".jpg"
		} else if vVid := vo2.GetMessage().GetVideoMessage(); vVid != nil {
			target = vVid
			ext = ".mp4"
		}
	}

	if target == nil {
		return ""
	}

	mediaDirOnce.Do(func() {
		_ = os.MkdirAll(filepath.Join("data", "media"), 0755)
	})
	destPath := filepath.Join("data", "media", "msg_"+msgID+ext)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data, err := w.Client.Download(ctx, target)
		if err == nil && len(data) > 0 {
			_ = os.WriteFile(destPath, data, 0644)
		}
	}()

	return destPath
}

func isBotMentioned(evt *events.Message, cli *whatsmeow.Client, lowerText string) bool {
	if evt.Message == nil {
		return false
	}

	var ctxInfo *waProto.ContextInfo
	if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
		ctxInfo = ext.GetContextInfo()
	}

	botJID := ""
	botUser := ""
	if cli != nil && cli.Store != nil && cli.Store.ID != nil {
		botJID = cli.Store.ID.ToNonAD().String()
		botUser = cli.Store.ID.ToNonAD().User
	}

	if ctxInfo != nil && botJID != "" {
		for _, m := range ctxInfo.GetMentionedJID() {
			if m == botJID || strings.HasPrefix(m, botUser+"@") {
				return true
			}
		}
	}

	if lowerText == "" {
		text := extractMessageText(evt)
		lowerText = strings.ToLower(text)
	}

	if strings.Contains(lowerText, "@prism") || strings.Contains(lowerText, "@bot") {
		return true
	}
	if botUser != "" && strings.Contains(lowerText, "@"+botUser) {
		return true
	}

	return false
}

func (w *WhatsAppService) handleAntiDelete(evt *events.Message) {
	protoMsg := evt.Message.GetProtocolMessage()
	if protoMsg == nil {
		return
	}

	deletedKey := protoMsg.GetKey()
	if deletedKey == nil || deletedKey.GetID() == "" {
		return
	}

	targetID := deletedKey.GetID()
	originalMsg, found := store.GlobalStore.Get(targetID)
	chatJID := evt.Info.Chat.String()

	senderJID := evt.Info.Sender.String()
	senderName := evt.Info.PushName
	if senderName == "" {
		senderName = store.GlobalStore.GetContact(senderJID)
	}
	if senderName == "" {
		senderName = strings.Split(senderJID, "@")[0]
	}

	chatName := store.GlobalStore.GetContact(chatJID)
	if chatName == "" {
		chatName = strings.Split(chatJID, "@")[0]
	}

	localFilePath := ""
	// Check if pre-cached media exists on disk
	for _, ext := range []string{".jpg", ".mp4", ".mp3", ".webp"} {
		checkPath := filepath.Join("data", "media", fmt.Sprintf("msg_%s%s", targetID, ext))
		if _, err := os.Stat(checkPath); err == nil {
			localFilePath = checkPath
			break
		}
	}

	audit := store.DeletedMessageAudit{
		ID:            targetID,
		ChatJID:       chatJID,
		ChatName:      chatName,
		SenderJID:     senderJID,
		SenderName:    senderName,
		PushName:      evt.Info.PushName,
		Text:          "[Message deleted]",
		LocalFilePath: localFilePath,
		DeletedAt:     time.Now(),
	}

	if found {
		audit.SenderJID = originalMsg.SenderJID.String()
		if originalMsg.PushName != "" {
			audit.PushName = originalMsg.PushName
			audit.SenderName = originalMsg.PushName
		}
		if originalMsg.Text != "" {
			audit.Text = originalMsg.Text
		}
		if originalMsg.MediaType != "" {
			audit.MediaType = originalMsg.MediaType
		}
	}

	store.GlobalPersistent.RecordDeleted(audit)

	// Send WhatsApp notification if anti-delete is enabled for this chat
	if store.GlobalStore.IsAntiDeleteEnabled(chatJID) && found {
		alert := fmt.Sprintf("*[Deleted Message Intercepted]*\n- Sender: @%s\n- Message:\n\"%s\"",
			originalMsg.SenderJID.User, audit.Text)

		cmdCtx := &commands.CommandContext{
			Client:  w.Client,
			ChatJID: evt.Info.Chat,
			Ctx:     context.Background(),
		}
		_ = cmdCtx.SendText(alert)

		// If media file was recovered, upload and resend it
		if localFilePath != "" {
			go func() {
				data, err := os.ReadFile(localFilePath)
				if err != nil || len(data) == 0 {
					return
				}
				ext := strings.ToLower(filepath.Ext(localFilePath))
				switch ext {
				case ".jpg", ".jpeg", ".png":
					if up, upErr := w.Client.Upload(context.Background(), data, whatsmeow.MediaImage); upErr == nil {
						imgMsg := &waProto.Message{
							ImageMessage: &waProto.ImageMessage{
								URL:           proto.String(up.URL),
								DirectPath:    proto.String(up.DirectPath),
								MediaKey:      up.MediaKey,
								Mimetype:      proto.String("image/jpeg"),
								FileEncSHA256: up.FileEncSHA256,
								FileSHA256:    up.FileSHA256,
								FileLength:    proto.Uint64(uint64(len(data))),
								Caption:       proto.String(fmt.Sprintf("*[Recovered Deleted Photo]*\nFrom: @%s", originalMsg.SenderJID.User)),
							},
						}
						_, _ = w.Client.SendMessage(context.Background(), evt.Info.Chat, imgMsg)
					}
				case ".mp4":
					if up, upErr := w.Client.Upload(context.Background(), data, whatsmeow.MediaVideo); upErr == nil {
						vidMsg := &waProto.Message{
							VideoMessage: &waProto.VideoMessage{
								URL:           proto.String(up.URL),
								DirectPath:    proto.String(up.DirectPath),
								MediaKey:      up.MediaKey,
								Mimetype:      proto.String("video/mp4"),
								FileEncSHA256: up.FileEncSHA256,
								FileSHA256:    up.FileSHA256,
								FileLength:    proto.Uint64(uint64(len(data))),
								Caption:       proto.String(fmt.Sprintf("*[Recovered Deleted Video]*\nFrom: @%s", originalMsg.SenderJID.User)),
							},
						}
						_, _ = w.Client.SendMessage(context.Background(), evt.Info.Chat, vidMsg)
					}
				case ".mp3", ".ogg":
					if up, upErr := w.Client.Upload(context.Background(), data, whatsmeow.MediaAudio); upErr == nil {
						audMsg := &waProto.Message{
							AudioMessage: &waProto.AudioMessage{
								URL:           proto.String(up.URL),
								DirectPath:    proto.String(up.DirectPath),
								MediaKey:      up.MediaKey,
								Mimetype:      proto.String("audio/mp4"),
								FileEncSHA256: up.FileEncSHA256,
								FileSHA256:    up.FileSHA256,
								FileLength:    proto.Uint64(uint64(len(data))),
							},
						}
						_, _ = w.Client.SendMessage(context.Background(), evt.Info.Chat, audMsg)
					}
				case ".webp":
					if up, upErr := w.Client.Upload(context.Background(), data, whatsmeow.MediaImage); upErr == nil {
						stkMsg := &waProto.Message{
							StickerMessage: &waProto.StickerMessage{
								URL:           proto.String(up.URL),
								DirectPath:    proto.String(up.DirectPath),
								MediaKey:      up.MediaKey,
								Mimetype:      proto.String("image/webp"),
								FileEncSHA256: up.FileEncSHA256,
								FileSHA256:    up.FileSHA256,
								FileLength:    proto.Uint64(uint64(len(data))),
							},
						}
						_, _ = w.Client.SendMessage(context.Background(), evt.Info.Chat, stkMsg)
					}
				}
			}()
		}
	}
}

func (w *WhatsAppService) checkAutoReply(c *commands.CommandContext, text string) bool {
	if text == "" || store.GlobalPersistent == nil {
		return false
	}
	resp, matched := store.GlobalPersistent.MatchAutoReply(text)
	if matched {
		_ = c.ReplyText(resp)
		return true
	}
	return false
}

func extractMessageInfo(evt *events.Message) (text string, mediaType string) {
	if evt.Message == nil {
		return "", ""
	}
	if evt.Message.Conversation != nil {
		return *evt.Message.Conversation, ""
	}
	if evt.Message.ExtendedTextMessage != nil && evt.Message.ExtendedTextMessage.Text != nil {
		return *evt.Message.ExtendedTextMessage.Text, ""
	}
	if img := evt.Message.GetImageMessage(); img != nil {
		caption := img.GetCaption()
		if caption == "" {
			return "[Photo]", "image"
		}
		return caption, "image"
	}
	if vid := evt.Message.GetVideoMessage(); vid != nil {
		caption := vid.GetCaption()
		if caption == "" {
			return "[Video]", "video"
		}
		return caption, "video"
	}
	if aud := evt.Message.GetAudioMessage(); aud != nil {
		if aud.GetPTT() {
			return "[Voice Note]", "audio"
		}
		return "[Audio]", "audio"
	}
	if stk := evt.Message.GetStickerMessage(); stk != nil {
		return "[Sticker]", "sticker"
	}
	if doc := evt.Message.GetDocumentMessage(); doc != nil {
		fn := doc.GetFileName()
		if fn == "" {
			fn = "Document"
		}
		return fmt.Sprintf("[Document: %s]", fn), "document"
	}
	if vo := evt.Message.GetViewOnceMessage(); vo != nil && vo.GetMessage() != nil {
		inner := vo.GetMessage()
		if img := inner.GetImageMessage(); img != nil {
			caption := img.GetCaption()
			if caption == "" {
				return "[View-Once Photo]", "image"
			}
			return fmt.Sprintf("[View-Once Photo] %s", caption), "image"
		}
		if vid := inner.GetVideoMessage(); vid != nil {
			caption := vid.GetCaption()
			if caption == "" {
				return "[View-Once Video]", "video"
			}
			return fmt.Sprintf("[View-Once Video] %s", caption), "video"
		}
	}
	if vo2 := evt.Message.GetViewOnceMessageV2(); vo2 != nil && vo2.GetMessage() != nil {
		inner := vo2.GetMessage()
		if img := inner.GetImageMessage(); img != nil {
			caption := img.GetCaption()
			if caption == "" {
				return "[View-Once Photo]", "image"
			}
			return fmt.Sprintf("[View-Once Photo] %s", caption), "image"
		}
		if vid := inner.GetVideoMessage(); vid != nil {
			caption := vid.GetCaption()
			if caption == "" {
				return "[View-Once Video]", "video"
			}
			return fmt.Sprintf("[View-Once Video] %s", caption), "video"
		}
	}
	return "", ""
}

func extractMessageText(evt *events.Message) string {
	text, _ := extractMessageInfo(evt)
	return text
}

func isRevokeMessage(evt *events.Message) bool {
	if evt.Message == nil {
		return false
	}
	protoMsg := evt.Message.GetProtocolMessage()
	if protoMsg != nil && protoMsg.GetType() == waProto.ProtocolMessage_REVOKE {
		return true
	}
	return false
}
