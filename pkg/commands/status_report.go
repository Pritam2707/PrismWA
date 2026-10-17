package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"whatsapp-bot/pkg/config"
	"whatsapp-bot/pkg/gemini"
	"whatsapp-bot/pkg/store"
)

// RegisterStatusReportAndDeadlineCommands registers all status reporting and deadline countdown commands
func RegisterStatusReportAndDeadlineCommands(r *CommandRegistry) {
	// Daily Status Reports & Gamified Streaks
	r.RegisterPrefix("!report", HandleReport)
	r.RegisterPrefix("!statusreport", HandleReport)
	r.RegisterPrefix("!dailyreport", HandleReport)
	r.RegisterPrefix("!reportboard", HandleLeaderboard)
	r.RegisterPrefix("!reportleaderboard", HandleLeaderboard)
	r.RegisterPrefix("!streaks", HandleLeaderboard)
	r.RegisterPrefix("!streak", HandleMyStatus)
	r.RegisterPrefix("!points", HandleLeaderboard)
	r.RegisterPrefix("!mystatus", HandleMyStatus)
	r.RegisterPrefix("!myreport", HandleMyStatus)
	r.RegisterPrefix("!reportgroup", HandleReportGroup)
	r.RegisterPrefix("!generatereport", HandleGenerateReport)
	r.RegisterPrefix("!summarizereport", HandleGenerateReport)

	// Group PFP Deadline Countdown
	r.RegisterPrefix("!setdeadline", HandleSetDeadline)
	r.RegisterPrefix("!deadline", HandleDeadline)
	r.RegisterPrefix("!updatepfp", HandleUpdatePFP)
	r.RegisterPrefix("!cleardeadline", HandleClearDeadline)
}

// HandleReport handles !report or !status achievement submissions
func HandleReport(c *CommandContext) (bool, error) {
	if store.GlobalReportsStore == nil {
		return true, c.ReplyText("Status reports store is not initialized.")
	}

	parts := strings.SplitN(c.RawText, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return true, c.ReplyText("📝 *Daily Achievement Report*\n\nPlease provide what you accomplished today!\n\n*Usage:* `!report Finished chapter 3 of Physics & built user login`\n💡 *Tip:* You can DM me this command directly to keep your achievements private until the midnight group summary!")
	}

	achievement := strings.TrimSpace(parts[1])
	senderJID := c.SenderJID.ToNonAD().String()
	senderName := c.SenderJID.User
	if c.RawMsg != nil && c.RawMsg.Info.PushName != "" {
		senderName = c.RawMsg.Info.PushName
	}

	groupJID := ""
	if c.IsGroup {
		groupJID = c.ChatJID.ToNonAD().String()
	}

	entry, stats, pointsEarned, err := store.GlobalReportsStore.SubmitReport(senderJID, senderName, groupJID, achievement)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to save report: %v", err))
	}

	_ = c.React("🌟")

	header := "🌟 *Daily Achievement Recorded!*"
	pointsNote := fmt.Sprintf("+%d pts", pointsEarned)
	if pointsEarned == 2 {
		header = "🌟 *Achievement Added to Today's Report!*"
		pointsNote = "+2 pts (update bonus)"
	}

	msg := fmt.Sprintf("%s\n\n"+
		"👤 *Participant:* %s\n"+
		"🎯 *Points:* %s\n"+
		"🏆 *Total Points:* %d pts\n"+
		"🔥 *Current Streak:* %d days (Best: %d days)\n\n"+
		"📋 *Today's Logged Achievements:*\n%s\n\n"+
		"Great work! Your combined update will be included in the midnight AI summary.",
		header, senderName, pointsNote, stats.Points, stats.CurrentStreak, stats.BestStreak, entry.Content)

	if c.IsGroup {
		msg += "\n\n💡 _Tip: You can also DM me this command directly to submit silently._"
	}

	return true, c.ReplyText(msg)
}

// HandleLeaderboard displays the points and streaks ranking
func HandleLeaderboard(c *CommandContext) (bool, error) {
	if store.GlobalReportsStore == nil {
		return true, c.ReplyText("Reports store is not initialized.")
	}

	leaderboard := store.GlobalReportsStore.GetLeaderboard()
	if len(leaderboard) == 0 {
		return true, c.ReplyText("🏆 *Daily Report Leaderboard*\n\nNo submissions yet! Submit your achievements using `!report <text>` to climb the leaderboard.")
	}

	var sb strings.Builder
	sb.WriteString("🏆 *DAILY REPORT LEADERBOARD* 🏆\n\n")

	var mentions []string
	limit := 10
	if len(leaderboard) < limit {
		limit = len(leaderboard)
	}

	for i := 0; i < limit; i++ {
		st := leaderboard[i]
		medal := fmt.Sprintf("%d.", i+1)
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}

		displayName := st.Name
		if displayName == "" {
			displayName = extractUser(st.UserJID)
		}

		sb.WriteString(fmt.Sprintf("%s *%s*\n   ⚡ *%d* pts | 🔥 *%d* day streak | 📊 %d reports\n",
			medal, displayName, st.Points, st.CurrentStreak, st.TotalSubmissions))
	}

	sb.WriteString("\n_Submit your daily progress with `!report <text>` to earn points and maintain streaks!_")
	return true, c.ReplyTextWithMentions(sb.String(), mentions)
}

// HandleMyStatus shows personal stats, streaks, and reports
func HandleMyStatus(c *CommandContext) (bool, error) {
	if store.GlobalReportsStore == nil {
		return true, c.ReplyText("Reports store is not initialized.")
	}

	senderJID := c.SenderJID.ToNonAD().String()
	stats, found := store.GlobalReportsStore.GetUserStats(senderJID)
	if !found {
		return true, c.ReplyText("📊 *Your Status & Streaks*\n\nYou haven't submitted any reports yet!\nSend `!report <what you did today>` to get started and earn points.")
	}

	todayStr := config.Now().Format("2006-01-02")
	submittedToday := (stats.LastReportDate == todayStr)
	todayStatus := "❌ Not submitted yet"
	if submittedToday {
		todayStatus = "✅ Submitted for today"
	}

	displayName := stats.Name
	if displayName == "" {
		displayName = c.SenderJID.User
	}

	msg := fmt.Sprintf("📊 *STATUS REPORT: %s*\n\n"+
		"🔥 *Current Streak:* %d days\n"+
		"⚡ *Best Streak:* %d days\n"+
		"🏆 *Total Points:* %d pts\n"+
		"📈 *Total Submissions:* %d\n"+
		"📅 *Today's Status:* %s\n\n"+
		"Keep it up! Submit with `!report <text>` anytime.",
		displayName, stats.CurrentStreak, stats.BestStreak, stats.Points, stats.TotalSubmissions, todayStatus)

	return true, c.ReplyText(msg)
}

// HandleReportGroup configures groups that receive the midnight AI summary
func HandleReportGroup(c *CommandContext) (bool, error) {
	if store.GlobalReportsStore == nil {
		return true, c.ReplyText("Reports store is not initialized.")
	}

	parts := strings.Fields(c.RawText)
	action := "list"
	if len(parts) >= 2 {
		action = strings.ToLower(parts[1])
	}

	switch action {
	case "add":
		if !c.IsSenderAdmin() && !c.IsSenderOwner() {
			return true, c.ReplyText("Only group admins or bot owner can configure report target groups.")
		}
		targetJID := c.ChatJID.ToNonAD().String()
		if len(parts) >= 3 {
			targetJID = strings.TrimSpace(parts[2])
		} else if !c.IsGroup {
			return true, c.ReplyText("Please specify a group JID or run this command directly inside the group: `!reportgroup add <groupJID>`")
		}

		store.GlobalReportsStore.AddReportGroup(targetJID)
		return true, c.ReplyText(fmt.Sprintf("✅ Added group `%s` to Daily Status Report distribution.\nMidnight AI summaries will be posted here automatically at 00:00.", targetJID))

	case "remove", "delete":
		if !c.IsSenderAdmin() && !c.IsSenderOwner() {
			return true, c.ReplyText("Only group admins or bot owner can configure report target groups.")
		}
		targetJID := c.ChatJID.ToNonAD().String()
		if len(parts) >= 3 {
			targetJID = strings.TrimSpace(parts[2])
		} else if !c.IsGroup {
			return true, c.ReplyText("Please specify a group JID or run this command directly inside the group: `!reportgroup remove <groupJID>`")
		}

		store.GlobalReportsStore.RemoveReportGroup(targetJID)
		return true, c.ReplyText(fmt.Sprintf("🗑️ Removed group `%s` from Daily Status Report distribution.", targetJID))

	case "list":
		groups := store.GlobalReportsStore.GetReportGroups()
		if len(groups) == 0 {
			return true, c.ReplyText("📋 *Daily Report Target Groups*\n\nNo groups currently configured.\nAdmins can use `!reportgroup add` inside a group to register it for midnight AI summaries.")
		}
		var sb strings.Builder
		sb.WriteString("📋 *Daily Report Target Groups*\n\nThe midnight AI summary will be sent to the following groups:\n")
		for i, g := range groups {
			sb.WriteString(fmt.Sprintf("%d. `%s`\n", i+1, g))
		}
		return true, c.ReplyText(sb.String())

	default:
		return true, c.ReplyText("Usage:\n• `!reportgroup add` (Register current group)\n• `!reportgroup remove` (Unregister current group)\n• `!reportgroup list` (View all target groups)")
	}
}

// HandleGenerateReport manually triggers generating today's / yesterday's AI report summary
func HandleGenerateReport(c *CommandContext) (bool, error) {
	dateStr := config.Now().Format("2006-01-02")
	parts := strings.Fields(c.RawText)
	if len(parts) >= 2 && strings.HasPrefix(parts[1], "20") {
		dateStr = parts[1]
	}

	_ = c.ReplyText(fmt.Sprintf("⏳ Generating AI Status Report for %s...", dateStr))

	summary, err := GenerateDailyReportSummary(c.Ctx, dateStr)
	if err != nil {
		return true, c.ReplyText(fmt.Sprintf("Failed to generate summary: %v", err))
	}

	return true, c.ReplyText(summary)
}

// GenerateDailyReportSummary compiles reports for a given date and generates an AI summary
func GenerateDailyReportSummary(ctx context.Context, dateStr string) (string, error) {
	if store.GlobalReportsStore == nil {
		return "", fmt.Errorf("reports store is not initialized")
	}

	reports := store.GlobalReportsStore.GetReportsForDate(dateStr)
	if len(reports) == 0 {
		return fmt.Sprintf("📋 *Daily Status Report (%s)*\n\nNo achievements were submitted for this date.", dateStr), nil
	}

	// Format raw entries
	var rawEntries strings.Builder
	for i, r := range reports {
		name := r.SenderName
		if name == "" {
			name = r.UserJID
		}
		rawEntries.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, name, r.Content))
	}

	if gemini.GlobalClient != nil {
		prompt := fmt.Sprintf(
			"Below are the daily achievements submitted by team members for the date %s:\n\n%s\n\n"+
				"Please create an inspiring, structured Daily Status Report Summary. "+
				"Organize by:\n"+
				"1. 🌟 Executive Highlights\n"+
				"2. 👥 Individual Achievements (categorize or highlight key deliverables)\n"+
				"3. 🚀 Daily Motivation / Focus for tomorrow\n"+
				"Use WhatsApp formatting (*bold*, _italic_, bullet points). Keep it engaging, professional, and clear. Avoid filler words.",
			dateStr, rawEntries.String(),
		)

		req := gemini.GenerateRequest{
			Contents: []gemini.Content{
				{
					Parts: []gemini.Part{
						{Text: prompt},
					},
				},
			},
		}

		genCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()

		aiResp, err := gemini.GlobalClient.Generate(genCtx, req)
		if err == nil && strings.TrimSpace(aiResp) != "" {
			return fmt.Sprintf("🌅 *DAILY ACHIEVEMENTS SUMMARY (%s)* 🌅\n\n%s", dateStr, strings.TrimSpace(aiResp)), nil
		}
	}

	// Fallback if AI client is unavailable
	var fallback strings.Builder
	fallback.WriteString(fmt.Sprintf("📋 *DAILY ACHIEVEMENTS SUMMARY (%s)*\n\n", dateStr))
	for _, r := range reports {
		name := r.SenderName
		if name == "" {
			name = r.UserJID
		}
		fallback.WriteString(fmt.Sprintf("• *%s*: %s\n", name, r.Content))
	}
	fallback.WriteString("\nGreat work everyone! Keep up the momentum.")
	return fallback.String(), nil
}
