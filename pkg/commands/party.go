package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"whatsapp-bot/pkg/meowcaller"
	"whatsapp-bot/pkg/tts"
)

func RegisterPartyCommands(r *CommandRegistry) {
	r.RegisterPrefix("!party", handlePartyCommand)
	r.RegisterPrefix("!songparty", handlePartyCommand)
	r.RegisterPrefix("!meowcaller", handlePartyCommand)
	r.RegisterPrefix("!caller", handlePartyCommand)
}

func handlePartyCommand(c *CommandContext) (bool, error) {
	text := strings.TrimSpace(c.RawText)
	allParts := strings.Fields(text)

	var parts []string
	if len(allParts) > 1 {
		parts = allParts[1:]
	}

	action := "status"
	if len(parts) > 0 {
		action = strings.ToLower(parts[0])
	}

	pushName := ""
	if c.RawMsg != nil && c.RawMsg.Info.PushName != "" {
		pushName = c.RawMsg.Info.PushName
	}

	mgr := meowcaller.GlobalManager

	// If disabled, allow 'status' or 'help', but block all actions with an informative prompt
	if !mgr.IsEnabled() && action != "help" {
		msg := "🔒 *MeowCaller is currently Disabled*\n\n" +
			"To protect privacy and prevent unsolicited WhatsApp voice calls, MeowCaller is kept *off by default*.\n\n" +
			"👉 *How to host a Song Party:*\n" +
			"1. Open the Admin Web Dashboard \n" +
			"2. Navigate to the *MeowCaller & Party* tab\n" +
			"3. Temporarily enable MeowCaller (e.g. for 30 or 60 minutes)\n" +
			"4. Use `!party start <song>` to start the call & music party!\n\n" +
			"💡 _Tip: You can also start the party and dial members directly from the Web Dashboard._"
		return true, c.ReplyText(msg)
	}

	switch action {
	case "help":
		helpMsg := "🎉 *MeowCaller Voice & Song Party Commands*\n\n" +
			"• `!party start <song>` — Start a voice call party and play a song\n" +
			"• `!party call` — Voice call this chat to join the active party\n" +
			"• `!party add <song>` — Add a song to the live party queue\n" +
			"• `!party queue` — View the current song and upcoming playlist\n" +
			"• `!party skip` — Skip the current track to the next in queue\n" +
			"• `!party pause` / `!party resume` — Control party playback\n" +
			"• `!party stop` / `!party hangup` — Stop music and hang up call\n" +
			"• `!party voicebot [on|off]` — Enable/disable Real-Time AI Voice Chatbot\n" +
			"• `!party voice <name>` — Change AI Voice persona (e.g. elon_musk, trump)\n" +
			"• `!party voices` — List all 29 available AI character voices\n" +
			"• `!party status` — Check call telemetry, music status & VoiceBot info\n\n" +
			"🎛️ _Full DJ controls & Real-Time Audio Monitor are live in your Web Dashboard!_"
		return true, c.ReplyText(helpMsg)

	case "status":
		status := mgr.GetStatus()
		var sb strings.Builder
		sb.WriteString("🎛️ *MeowCaller & Song Party Status*\n\n")
		sb.WriteString("• *State:* 🟢 Active & Enabled\n")
		if status.TempSecondsLeft > 0 {
			mins := status.TempSecondsLeft / 60
			secs := status.TempSecondsLeft % 60
			sb.WriteString(fmt.Sprintf("• *Temporary Timer:* ⏱️ %dm %ds remaining\n", mins, secs))
		} else {
			sb.WriteString("• *Temporary Timer:* ♾️ Until manually disabled\n")
		}

		if status.ActiveCall != nil {
			sb.WriteString(fmt.Sprintf("• *Call Peer:* `%s`\n", status.ActiveCall.Peer))
			sb.WriteString(fmt.Sprintf("• *Call Phase:* `%s`\n", status.ActiveCall.Phase))
			sb.WriteString(fmt.Sprintf("• *Call Duration:* %ds\n", status.ActiveCall.DurationSec))
		} else {
			sb.WriteString("• *Call Status:* 📞 No active call\n")
		}

		if status.CurrentTrack != nil {
			stateIcon := "▶️"
			if status.IsPaused {
				stateIcon = "⏸️"
			}
			sb.WriteString(fmt.Sprintf("• *Now Playing:* %s \"%s\"\n", stateIcon, status.CurrentTrack.Title))
			sb.WriteString(fmt.Sprintf("• *Requested by:* %s\n", status.CurrentTrack.RequestedBy))
		} else {
			sb.WriteString("• *Playback:* ⏹️ Idle (No track playing)\n")
		}

		sb.WriteString(fmt.Sprintf("• *Queue Length:* %d songs\n", len(status.Queue)))
		return true, c.ReplyText(sb.String())

	case "start":
		if len(parts) < 2 {
			return true, c.ReplyText("Usage: `!party start <song name or youtube url>`\nExample: `!party start blinding lights`")
		}
		query := strings.Join(parts[1:], " ")
		_ = c.React("🎧")

		target := c.ChatJID.String()
		requester := c.SenderJID.User
		if pushName != "" {
			requester = pushName
		}

		_ = c.ReplyText(fmt.Sprintf("🔍 *Preparing Song Party...*\nSearching & downloading: _\"%s\"_\nCalling chat in a moment...", query))

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			track, err := mgr.StartParty(ctx, target, query, requester, c.ChatJID.String())
			if err != nil {
				_ = c.ReplyText(fmt.Sprintf("❌ *Failed to start Song Party:* %v", err))
				return
			}
			_ = c.React("🎉")
			_ = c.ReplyText(fmt.Sprintf("🎉 *Song Party is LIVE!*\n📞 Connected/calling: `%s`\n🎶 *Now Playing:* \"%s\"\n\n_Anyone in this chat can add tracks using `!party add <song>`!_", target, track.Title))
		}()
		return true, nil

	case "call":
		_ = c.React("📞")
		target := c.ChatJID.String()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := mgr.Dial(ctx, target)
			if err != nil {
				_ = c.ReplyText(fmt.Sprintf("❌ *Failed to place call:* %v", err))
				return
			}
			_ = c.ReplyText(fmt.Sprintf("📞 *Calling `%s` for Song Party!* Answer to listen.", target))
		}()
		return true, nil

	case "add", "play":
		if len(parts) < 2 {
			return true, c.ReplyText("Usage: `!party add <song name or url>`\nExample: `!party add shape of you`")
		}
		query := strings.Join(parts[1:], " ")
		_ = c.React("🔍")

		requester := c.SenderJID.User
		if pushName != "" {
			requester = pushName
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			track, err := mgr.AddToQueue(ctx, query, requester)
			if err != nil {
				_ = c.ReplyText(fmt.Sprintf("❌ *Failed to queue song:* %v", err))
				return
			}
			_ = c.React("🎵")
			_ = c.ReplyText(fmt.Sprintf("🎶 *Queued:* \"%s\"\n👤 *Requested by:* %s", track.Title, requester))
		}()
		return true, nil

	case "queue", "list":
		status := mgr.GetStatus()
		var sb strings.Builder
		sb.WriteString("🎵 *Song Party Playlist Queue*\n\n")
		if status.CurrentTrack != nil {
			sb.WriteString(fmt.Sprintf("▶️ *Now Playing:* \"%s\" _(by %s)_\n\n", status.CurrentTrack.Title, status.CurrentTrack.RequestedBy))
		} else {
			sb.WriteString("⏹️ *No track currently playing.*\n\n")
		}

		if len(status.Queue) == 0 {
			sb.WriteString("Queue is empty. Add songs with `!party add <song>`!")
		} else {
			sb.WriteString("*Upcoming Tracks:*\n")
			for i, t := range status.Queue {
				sb.WriteString(fmt.Sprintf("%d. *%s* _(requested by %s)_\n", i+1, t.Title, t.RequestedBy))
			}
		}
		return true, c.ReplyText(sb.String())

	case "skip":
		track, err := mgr.Skip()
		if err != nil {
			return true, c.ReplyText(fmt.Sprintf("❌ %v", err))
		}
		_ = c.React("⏭️")
		return true, c.ReplyText(fmt.Sprintf("⏭️ *Skipped:* \"%s\"", track.Title))

	case "pause":
		if err := mgr.Pause(); err != nil {
			return true, c.ReplyText(fmt.Sprintf("❌ %v", err))
		}
		_ = c.React("⏸️")
		return true, c.ReplyText("⏸️ *Song Party paused.* Resume with `!party resume`.")

	case "resume":
		if err := mgr.Resume(); err != nil {
			return true, c.ReplyText(fmt.Sprintf("❌ %v", err))
		}
		_ = c.React("▶️")
		return true, c.ReplyText("▶️ *Song Party resumed!*")

	case "stop", "end", "hangup":
		mgr.StopParty()
		_ = c.React("⏹️")
		return true, c.ReplyText("⏹️ *Song Party ended and call hung up.*")

	case "voicebot", "bot", "ai":
		vb := mgr.GetVoiceBot()
		if vb == nil {
			return true, c.ReplyText("❌ VoiceBot is not available.")
		}
		if len(parts) > 1 {
			subAction := strings.ToLower(parts[1])
			if subAction == "on" || subAction == "enable" || subAction == "start" {
				t := vb.GetTelemetry()
				mgr.UpdateVoiceBotConfig(meowcaller.VoiceBotConfig{
					Enabled:            true,
					ServerURL:          t.ServerURL,
					Voice:              t.Voice,
					GeminiModel:        t.GeminiModel,
					SilenceThresholdMs: 700,
				})
				return true, c.ReplyText("🎙️ *Realtime AI Voice Bot:* 🟢 *ENABLED*\nThe AI will now listen and speak in active calls via Groq Whisper STT + AI Voice Responses!")
			} else if subAction == "off" || subAction == "disable" || subAction == "stop" {
				t := vb.GetTelemetry()
				mgr.UpdateVoiceBotConfig(meowcaller.VoiceBotConfig{
					Enabled:            false,
					ServerURL:          t.ServerURL,
					Voice:              t.Voice,
					GeminiModel:        t.GeminiModel,
					SilenceThresholdMs: 700,
				})
				return true, c.ReplyText("🎙️ *Realtime AI Voice Bot:* 🔴 *DISABLED*")
			}
		}

		telemetry := vb.GetTelemetry()
		var sb strings.Builder
		sb.WriteString("🎙️ *Realtime Voice Chatbot Telemetry*\n\n")
		if telemetry.Enabled {
			sb.WriteString("• *Enabled:* 🟢 Active\n")
		} else {
			sb.WriteString("• *Enabled:* 🔴 Off (`!party voicebot on` to enable)\n")
		}
		sb.WriteString(fmt.Sprintf("• *ASR Engine:* `%s`\n", telemetry.Engine))
		sb.WriteString(fmt.Sprintf("• *State:* `%s`\n", telemetry.State))
		sb.WriteString(fmt.Sprintf("• *Voice:* `%s`\n", telemetry.Voice))
		sb.WriteString(fmt.Sprintf("• *Gemini Model:* `%s`\n", telemetry.GeminiModel))
		sb.WriteString(fmt.Sprintf("• *Last Latency:* %d ms\n", telemetry.LastLatencyMs))
		sb.WriteString(fmt.Sprintf("• *Total Conversational Turns:* %d\n", telemetry.TotalTurns))
		if telemetry.LastTranscript != "" {
			sb.WriteString(fmt.Sprintf("• *Caller Said:* _\"%s\"_\n", telemetry.LastTranscript))
		}
		if telemetry.LastResponse != "" {
			sb.WriteString(fmt.Sprintf("• *AI Reply:* _\"%s\"_\n", telemetry.LastResponse))
		}
		return true, c.ReplyText(sb.String())

	case "voice":
		if len(parts) < 2 {
			return true, c.ReplyText("Usage: `!party voice <name>` (e.g. `!party voice elon_musk`). Type `!party voices` for the list.")
		}
		vName := strings.ToLower(parts[1])
		vb := mgr.GetVoiceBot()
		if vb == nil {
			return true, c.ReplyText("❌ VoiceBot not available.")
		}
		t := vb.GetTelemetry()
		mgr.UpdateVoiceBotConfig(meowcaller.VoiceBotConfig{
			Enabled:            t.Enabled,
			ServerURL:          t.ServerURL,
			Voice:              vName,
			GeminiModel:        t.GeminiModel,
			SilenceThresholdMs: 700,
		})
		return true, c.ReplyText(fmt.Sprintf("🎭 AI Voice persona switched to: *%s*", vName))

	case "voices":
		list := tts.GetAvailableVoices()
		return true, c.ReplyText(fmt.Sprintf("🎭 *Available AI Voices (%d):*\n\n`%s`\n\n_Use `!party voice <name>` to select._", len(list), strings.Join(list, "`, `")))


	default:
		// If user typed `!party <song name>`, treat as `!party start <song name>` if idle, or `!party add` if active
		defaultQuery := strings.TrimSpace(strings.TrimPrefix(text, allParts[0]))
		if defaultQuery == "" {
			return true, c.ReplyText("Usage: `!party start <song>` or `!party add <song>`. Type `!party help` for all commands.")
		}

		status := mgr.GetStatus()
		if status.IsPartyActive {
			// add to queue
			requester := c.SenderJID.User
			if pushName != "" {
				requester = pushName
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				track, err := mgr.AddToQueue(ctx, defaultQuery, requester)
				if err != nil {
					_ = c.ReplyText(fmt.Sprintf("❌ Failed: %v", err))
					return
				}
				_ = c.ReplyText(fmt.Sprintf("🎶 *Added to Queue:* \"%s\"", track.Title))
			}()
			return true, nil
		}

		// Otherwise start party with this song
		target := c.ChatJID.String()
		requester := c.SenderJID.User
		if pushName != "" {
			requester = pushName
		}
		_ = c.React("🎧")
		_ = c.ReplyText(fmt.Sprintf("🔍 *Starting Song Party with:* _\"%s\"_...", defaultQuery))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			track, err := mgr.StartParty(ctx, target, defaultQuery, requester, c.ChatJID.String())
			if err != nil {
				_ = c.ReplyText(fmt.Sprintf("❌ Failed to start party: %v", err))
				return
			}
			_ = c.ReplyText(fmt.Sprintf("🎉 *Song Party Started!* Now playing: \"%s\"", track.Title))
		}()
		return true, nil
	}
}

