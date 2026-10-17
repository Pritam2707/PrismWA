import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

// File persistence setup
const DATA_DIR = path.join(process.cwd(), "data");
const REMINDERS_FILE = path.join(DATA_DIR, "reminders.json");

if (!existsSync(DATA_DIR)) {
  mkdirSync(DATA_DIR, { recursive: true });
}

// In-memory active timer instances
const activeTimerMap = new Map();

function loadReminders() {
  try {
    if (existsSync(REMINDERS_FILE)) {
      return JSON.parse(readFileSync(REMINDERS_FILE, "utf-8"));
    }
  } catch (e) {
    console.error("Failed to read reminders file:", e);
  }
  return [];
}

function saveReminders(reminders) {
  try {
    writeFileSync(REMINDERS_FILE, JSON.stringify(reminders, null, 2), "utf-8");
  } catch (e) {
    console.error("Failed to save reminders file:", e);
  }
}

function removeReminderFromDisk(id) {
  const reminders = loadReminders().filter((r) => r.id !== id);
  saveReminders(reminders);
  activeTimerMap.delete(id);
}

function parseDuration(timeStr) {
  const match = timeStr.match(/^(\d+)(s|m|h|d)$/i);
  if (!match) return null;

  const value = parseInt(match[1], 10);
  const unit = match[2].toLowerCase();

  switch (unit) {
    case "s": return value * 1000;
    case "m": return value * 60 * 1000;
    case "h": return value * 60 * 60 * 1000;
    case "d": return value * 24 * 60 * 60 * 1000;
    default: return null;
  }
}

/**
 * Schedule or execute a reminder timer
 */
function scheduleTimer(sock, reminder) {
  const now = Date.now();
  const delay = Math.max(0, reminder.triggerAt - now);

  const timerId = setTimeout(async () => {
    try {
      const senderTag = `@${reminder.senderId.split("@")[0]}`;
      const missedNotice = reminder.triggerAt < now ? " _(Delivered after bot restart)_" : "";

      await sock.sendMessage(reminder.jid, {
        text: `⏰ *REMINDER FOR ${senderTag}*${missedNotice}\n\n📝 ${reminder.message}`,
        mentions: [reminder.senderId],
      });
    } catch (e) {
      console.error("Failed to send scheduled reminder message:", e);
    } finally {
      removeReminderFromDisk(reminder.id);
    }
  }, delay);

  activeTimerMap.set(reminder.id, timerId);
}

/**
 * Call this function once when the bot connects/restarts
 */
export function restoreScheduledReminders(sock) {
  const pendingReminders = loadReminders();
  if (pendingReminders.length === 0) return;

  console.log(`⏰ Restoring ${pendingReminders.length} pending reminder(s) from storage...`);

  for (const reminder of pendingReminders) {
    scheduleTimer(sock, reminder);
  }
}

export async function handleSchedulerCommands(sock, jid, senderId, rawText, msg) {
  const text = rawText.trim();
  const lowerText = text.toLowerCase();

  if (lowerText.startsWith("!remind") || lowerText.startsWith("!schedule")) {
    const parts = text.split(/\s+/);
    const timeArg = parts[1];
    const reminderMessage = parts.slice(2).join(" ");

    if (!timeArg || !reminderMessage) {
      await sock.sendMessage(
        jid,
        {
          text: "⚠️ *Usage:* `!remind <time> <message>`\n\n*Examples:*\n• `!remind 10m Take a break`\n• `!remind 1h Call mom`\n• `!remind 30s Check oven`\n\n*(Supported units: s = seconds, m = minutes, h = hours, d = days)*",
        },
        { quoted: msg }
      );
      return true;
    }

    const durationMs = parseDuration(timeArg);
    if (!durationMs) {
      await sock.sendMessage(
        jid,
        { text: "❌ Invalid time format! Use numbers followed by `s`, `m`, `h`, or `d` (e.g. `15m`, `2h`)." },
        { quoted: msg }
      );
      return true;
    }

    const id = `rem_${Date.now()}_${Math.random().toString(36).substring(2, 7)}`;
    const triggerAt = Date.now() + durationMs;

    const newReminder = {
      id,
      jid,
      senderId,
      message: reminderMessage,
      timeArg,
      createdAt: Date.now(),
      triggerAt,
    };

    // Save to local disk
    const currentReminders = loadReminders();
    currentReminders.push(newReminder);
    saveReminders(currentReminders);

    // Schedule active timer
    scheduleTimer(sock, newReminder);

    await sock.sendMessage(
      jid,
      { text: `✅ *Reminder Set!* I will remind you in *${timeArg}*.` },
      { quoted: msg }
    );
    return true;
  }

  // List active reminders for this chat (!reminders)
  if (lowerText === "!reminders" || lowerText === "!listreminders") {
    const chatReminders = loadReminders().filter((r) => r.jid === jid);

    if (chatReminders.length === 0) {
      await sock.sendMessage(jid, { text: "⏰ No active reminders scheduled for this chat." }, { quoted: msg });
      return true;
    }

    let listText = `⏰ *Scheduled Reminders for this Chat (${chatReminders.length}):*\n\n`;
    chatReminders.forEach((r, idx) => {
      const remainingSecs = Math.max(0, Math.round((r.triggerAt - Date.now()) / 1000));
      const mins = Math.floor(remainingSecs / 60);
      const secs = remainingSecs % 60;
      const timeStr = mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;

      listText += `*${idx + 1}.* "${r.message}"\n👤 @${r.senderId.split("@")[0]} | ⏳ Due in: ${timeStr}\n\n`;
    });

    await sock.sendMessage(
      jid,
      { text: listText, mentions: chatReminders.map((r) => r.senderId) },
      { quoted: msg }
    );
    return true;
  }

  return false;
}