package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/store"
)

func RegisterDeletedCommands(r *CommandRegistry) {
	r.RegisterPrefix("!deleted", handleDeletedCommands)
}

func handleDeletedCommands(c *CommandContext) (bool, error) {
	args := strings.Fields(c.RawText)
	chatJID := c.ChatJID.String()

	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}

	// 1. Delete operations: !deleted del <all|index>
	if sub == "del" || sub == "rm" {
		if !c.IsSenderOwner() {
			return true, c.ReplyText("⛔ *Access Denied:* Only the bot owner is permitted to delete saved log records.")
		}

		if len(args) < 3 {
			return true, c.ReplyText("⚠️ Usage: `!deleted del all` | `!deleted del 1`")
		}

		delArg := strings.ToLower(args[2])
		if delArg == "all" {
			store.GlobalPersistent.DeleteAllAuditForChat(chatJID)
			return true, c.ReplyText("🗑️ *Cleared all recorded deleted messages for this chat!*")
		}

		targetIdx, err := strconv.Atoi(delArg)
		if err != nil || targetIdx < 1 {
			return true, c.ReplyText("⚠️ *Invalid index!* Enter a valid positive number.")
		}

		chatList := store.GlobalPersistent.GetDeletedMessagesForChat(chatJID, 100)
		if targetIdx > len(chatList) {
			return true, c.ReplyText(fmt.Sprintf("⚠️ *Invalid index!* Current log count is `%d`.", len(chatList)))
		}

		targetItem := chatList[targetIdx-1]
		store.GlobalPersistent.DeleteAuditEntry(chatJID, targetItem.ID)
		return true, c.ReplyText(fmt.Sprintf("🗑️ *Successfully removed entry [#%d]!*", targetIdx))
	}

	// 2. Retrieve / get operation: !deleted get <index> or !deleted <number>
	isGet := sub == "get"
	targetIdx := 0
	if isGet && len(args) > 2 {
		targetIdx, _ = strconv.Atoi(args[2])
	} else if !isGet && sub != "" && sub != "list" {
		targetIdx, _ = strconv.Atoi(sub)
	}

	if targetIdx > 0 {
		chatList := store.GlobalPersistent.GetDeletedMessagesForChat(chatJID, 100)
		if targetIdx > len(chatList) {
			return true, c.ReplyText(fmt.Sprintf("⚠️ *Invalid index!* Use `!deleted list` to view entries (1-%d).", len(chatList)))
		}

		item := chatList[targetIdx-1]
		sender := item.SenderName
		if sender == "" {
			sender = item.PushName
		}
		if sender == "" {
			sender = item.SenderJID
		}

		delTime := item.DeletedAt.Format("03:04 PM, 02 Jan")

		// If local media file exists, resend the media
		if item.LocalFilePath != "" {
			if data, err := os.ReadFile(item.LocalFilePath); err == nil && len(data) > 0 {
				ext := strings.ToLower(filepath.Ext(item.LocalFilePath))
				caption := fmt.Sprintf("*[RECOVERED DELETED MEDIA]*\n👤 *From:* %s\n📅 *Deleted:* %s\n\n%s", sender, delTime, item.Text)

				switch ext {
				case ".jpg", ".jpeg", ".png":
					if up, upErr := c.Client.Upload(context.Background(), data, whatsmeow.MediaImage); upErr == nil {
						msg := &waProto.Message{
							ImageMessage: &waProto.ImageMessage{
								URL:           proto.String(up.URL),
								DirectPath:    proto.String(up.DirectPath),
								MediaKey:      up.MediaKey,
								Mimetype:      proto.String("image/jpeg"),
								FileEncSHA256: up.FileEncSHA256,
								FileSHA256:    up.FileSHA256,
								FileLength:    proto.Uint64(uint64(len(data))),
								Caption:       proto.String(caption),
							},
						}
						_, _ = c.Client.SendMessage(context.Background(), c.ChatJID, msg)
						return true, nil
					}
				case ".mp4":
					if up, upErr := c.Client.Upload(context.Background(), data, whatsmeow.MediaVideo); upErr == nil {
						msg := &waProto.Message{
							VideoMessage: &waProto.VideoMessage{
								URL:           proto.String(up.URL),
								DirectPath:    proto.String(up.DirectPath),
								MediaKey:      up.MediaKey,
								Mimetype:      proto.String("video/mp4"),
								FileEncSHA256: up.FileEncSHA256,
								FileSHA256:    up.FileSHA256,
								FileLength:    proto.Uint64(uint64(len(data))),
								Caption:       proto.String(caption),
							},
						}
						_, _ = c.Client.SendMessage(context.Background(), c.ChatJID, msg)
						return true, nil
					}
				case ".mp3", ".ogg":
					if up, upErr := c.Client.Upload(context.Background(), data, whatsmeow.MediaAudio); upErr == nil {
						msg := &waProto.Message{
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
						_, _ = c.Client.SendMessage(context.Background(), c.ChatJID, msg)
						return true, nil
					}
				case ".webp":
					if up, upErr := c.Client.Upload(context.Background(), data, whatsmeow.MediaImage); upErr == nil {
						msg := &waProto.Message{
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
						_, _ = c.Client.SendMessage(context.Background(), c.ChatJID, msg)
						return true, nil
					}
				}
			}
		}

		// Text fallback recovery
		out := fmt.Sprintf("*[RECOVERED DELETED MESSAGE]*\n👤 *From:* %s\n📅 *Deleted:* %s\n\n*Message:*\n\"%s\"",
			sender, delTime, item.Text)
		return true, c.ReplyText(out)
	}

	// 3. Default: List operation (!deleted or !deleted list)
	chatList := store.GlobalPersistent.GetDeletedMessagesForChat(chatJID, 25)
	if len(chatList) == 0 {
		return true, c.ReplyText("📂 *No stored deleted messages found for this chat.*")
	}

	var sb strings.Builder
	sb.WriteString("╭━━━ 🗑️ *DELETED MESSAGES LOG* ━━━╮\n\n")

	for i, item := range chatList {
		timeStr := item.DeletedAt.Format("03:04 PM")
		name := item.SenderName
		if name == "" {
			name = item.PushName
		}
		if name == "" {
			name = strings.Split(item.SenderJID, "@")[0]
		}

		snippet := item.Text
		if len(snippet) > 60 {
			snippet = snippet[:57] + "..."
		}
		if snippet == "" {
			snippet = "[Empty message or payload]"
		}

		sb.WriteString(fmt.Sprintf("*#%d* • *%s* `(%s)`\n└ %s\n\n", i+1, name, timeStr, snippet))
	}

	sb.WriteString("─────────────────────────\n")
	sb.WriteString("👉 *Get:* `!deleted get <num>`\n")
	sb.WriteString("❌ *Delete (Owner Only):* `!deleted del <num>` | `!deleted del all`")

	return true, c.ReplyText(sb.String())
}
