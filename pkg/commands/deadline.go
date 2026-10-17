package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/polymorfa/hypermeow"
	waProto "github.com/polymorfa/hypermeow/binary/proto"
	"google.golang.org/protobuf/proto"

	"whatsapp-bot/pkg/pfp"
	"whatsapp-bot/pkg/store"
)

// HandleSetDeadline sets or updates a group's target countdown and updates the group PFP
func HandleSetDeadline(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("Deadlines and group PFP updates can only be set in group chats.")
	}

	if !c.IsSenderAdmin() && !c.IsSenderOwner() {
		return true, c.ReplyText("Only group admins or the bot owner can configure deadlines.")
	}

	parts := strings.Fields(c.RawText)
	if len(parts) < 2 {
		return true, c.ReplyText("⏰ *Set Group Countdown Deadline*\n\n" +
			"*Usage:* `!setdeadline <YYYY-MM-DD> [Title]`\n" +
			"*Example:* `!setdeadline 2026-10-15 Final Semester Exams`\n\n" +
			"The bot will automatically render an aesthetic countdown profile picture and update the group photo every day at midnight!")
	}

	targetDateStr := parts[1]
	_, err := time.Parse("2006-01-02", targetDateStr)
	if err != nil {
		return true, c.ReplyText("❌ Invalid date format! Please use `YYYY-MM-DD` (e.g. `2026-10-15`).")
	}

	title := "Target Deadline"
	if len(parts) >= 3 {
		title = strings.Join(parts[2:], " ")
	}

	groupJID := c.ChatJID.ToNonAD().String()
	store.GlobalReportsStore.SetGroupDeadline(groupJID, targetDateStr, title)

	_ = c.ReplyText(fmt.Sprintf("⏳ Generating aesthetic PFP for *%s* (Due: %s)...", title, targetDateStr))

	// Generate PFP image
	jpegBytes, err := pfp.GenerateDeadlinePFP(title, targetDateStr)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to generate countdown graphic: %v", err))
	}

	// Update Group Profile Picture
	pfpUpdated := false
	var updateErr error
	if c.Client != nil {
		_, updateErr = c.Client.SetGroupPhoto(c.Ctx, c.ChatJID, jpegBytes)
		if updateErr == nil {
			pfpUpdated = true
		}
	}

	// Upload image and send preview to chat
	uploaded, upErr := c.Client.Upload(context.Background(), jpegBytes, whatsmeow.MediaImage)
	if upErr == nil {
		caption := fmt.Sprintf("🎯 *Countdown Configured: %s*\n📅 *Target Date:* %s\n", title, targetDateStr)
		if pfpUpdated {
			caption += "🖼️ *Group Icon:* Successfully updated!\n"
		} else if updateErr != nil {
			caption += fmt.Sprintf("⚠️ *Group Icon Update:* %v (Ensure bot is group admin to change group photo)\n", updateErr)
		}
		caption += "🔄 Automatically refreshes daily at midnight (00:00)."

		imageMsg := &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String("image/jpeg"),
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(jpegBytes))),
				Caption:       proto.String(caption),
			},
		}
		_, _ = c.Client.SendMessage(c.Ctx, c.ChatJID, imageMsg)
		return true, nil
	}

	statusMsg := fmt.Sprintf("✅ Deadline set for *%s* on *%s*.", title, targetDateStr)
	if pfpUpdated {
		statusMsg += " Group icon updated!"
	}
	return true, c.ReplyText(statusMsg)
}

// HandleDeadline displays current countdown and sends the rendered image
func HandleDeadline(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("!deadline can only be used inside group chats.")
	}

	groupJID := c.ChatJID.ToNonAD().String()
	cfg, found := store.GlobalReportsStore.GetGroupDeadline(groupJID)
	if !found {
		return true, c.ReplyText("📅 No active deadline set for this group.\nGroup admins can set one with `!setdeadline <YYYY-MM-DD> <Title>`.")
	}

	jpegBytes, err := pfp.GenerateDeadlinePFP(cfg.Title, cfg.TargetDate)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to generate countdown preview: %v", err))
	}

	uploaded, err := c.Client.Upload(context.Background(), jpegBytes, whatsmeow.MediaImage)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("🎯 *Group Deadline:* %s\n📅 *Target Date:* %s", cfg.Title, cfg.TargetDate))
	}

	caption := fmt.Sprintf("🎯 *Group Countdown: %s*\n📅 *Target Date:* %s\n🔄 Automatically updates group PFP daily at midnight.", cfg.Title, cfg.TargetDate)
	imageMsg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String("image/jpeg"),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(jpegBytes))),
			Caption:       proto.String(caption),
		},
	}

	_, err = c.Client.SendMessage(c.Ctx, c.ChatJID, imageMsg)
	return true, err
}

// HandleUpdatePFP forces updating the group photo immediately based on the active deadline
func HandleUpdatePFP(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("PFP update can only be run in group chats.")
	}
	if !c.IsSenderAdmin() && !c.IsSenderOwner() {
		return true, c.ReplyText("Only group admins or the bot owner can update the group icon.")
	}

	groupJID := c.ChatJID.ToNonAD().String()
	cfg, found := store.GlobalReportsStore.GetGroupDeadline(groupJID)
	if !found {
		return true, c.ReplyText("No deadline configured for this group. Set one with `!setdeadline <YYYY-MM-DD> <Title>` first.")
	}

	jpegBytes, err := pfp.GenerateDeadlinePFP(cfg.Title, cfg.TargetDate)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to generate countdown graphic: %v", err))
	}

	pictureID, err := c.Client.SetGroupPhoto(c.Ctx, c.ChatJID, jpegBytes)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("⚠️ Failed to update group icon: %v\n(Please ensure the bot has Admin privileges in this group).", err))
	}

	return true, c.ReplyText(fmt.Sprintf("✅ Group profile photo updated successfully! (Picture ID: %s)", pictureID))
}

// HandleClearDeadline deactivates the countdown for the group
func HandleClearDeadline(c *CommandContext) (bool, error) {
	if !c.IsGroup {
		return true, c.ReplyText("!cleardeadline can only be used in group chats.")
	}
	if !c.IsSenderAdmin() && !c.IsSenderOwner() {
		return true, c.ReplyText("Only group admins or the bot owner can remove group deadlines.")
	}

	groupJID := c.ChatJID.ToNonAD().String()
	store.GlobalReportsStore.RemoveGroupDeadline(groupJID)

	return true, c.ReplyText("🗑️ Group deadline removed. Automatic daily PFP updates have been stopped for this group.")
}
