import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { contactStore } from "../store/messageStore.js";

const DATA_DIR = path.join(process.cwd(), "data");
const STATS_FILE = path.join(DATA_DIR, "chat_stats.json");

if (!existsSync(DATA_DIR)) {
  mkdirSync(DATA_DIR, { recursive: true });
}

// ---------------------------------------------------------------------------
// Memory Cache & Debounced Disk Persistence
// ---------------------------------------------------------------------------
let statsCache = null;
let saveTimeout = null;

function loadStats() {
  if (statsCache) return statsCache;
  try {
    if (existsSync(STATS_FILE)) {
      statsCache = JSON.parse(readFileSync(STATS_FILE, "utf-8"));
      return statsCache;
    }
  } catch (e) {
    console.error("❌ Failed to read chat stats file:", e);
  }
  statsCache = {};
  return statsCache;
}

function queueSaveStats() {
  if (saveTimeout) clearTimeout(saveTimeout);
  saveTimeout = setTimeout(() => {
    try {
      if (statsCache) {
        writeFileSync(STATS_FILE, JSON.stringify(statsCache, null, 2), "utf-8");
      }
    } catch (e) {
      console.error("❌ Failed to save chat stats file:", e);
    }
  }, 3000);
}

// ---------------------------------------------------------------------------
// Helpers: Progress Bar, Badges & Activity Health
// ---------------------------------------------------------------------------

function makeProgressBar(percent, length = 8) {
  const filled = Math.min(length, Math.max(0, Math.round((percent / 100) * length)));
  const empty = length - filled;
  return "█".repeat(filled) + "░".repeat(empty);
}

function getRankTitle(count, totalMessages) {
  const share = totalMessages > 0 ? (count / totalMessages) * 100 : 0;
  if (share >= 35) return "👑 Chat Sovereign";
  if (share >= 20) return "🔥 Yapper Supreme";
  if (share >= 10) return "⚡ Key Contributor";
  if (count >= 100) return "💬 Regular Chatter";
  if (count >= 20) return "🌱 Active Member";
  return "👻 Lurker";
}

function getChatActivityStatus(totalMsgs, startTime) {
  const daysActive = Math.max(1, (Date.now() - startTime) / (1000 * 60 * 60 * 24));
  const dailyAverage = totalMsgs / daysActive;

  if (dailyAverage > 500) return { label: "🔥 HYPERACTIVE", avg: dailyAverage.toFixed(0) };
  if (dailyAverage > 150) return { label: "⚡ VERY ACTIVE", avg: dailyAverage.toFixed(0) };
  if (dailyAverage > 40) return { label: "💬 MODERATE", avg: dailyAverage.toFixed(0) };
  return { label: "💤 QUIET CHAT", avg: dailyAverage.toFixed(0) };
}

// ---------------------------------------------------------------------------
// Middleware: Call this in index.js for every incoming message
// ---------------------------------------------------------------------------

export function trackMessageStats(jid, senderId, msg) {
  const db = loadStats();

  if (!db[jid]) {
    db[jid] = {
      createdAt: Date.now(),
      totalMessages: 0,
      breakdown: {
        text: 0,
        media: 0,
        audio: 0,
        stickers: 0,
      },
      users: {},
    };
  }

  const chat = db[jid];
  chat.totalMessages = (chat.totalMessages || 0) + 1;
  if (!chat.breakdown) {
    chat.breakdown = { text: 0, media: 0, audio: 0, stickers: 0 };
  }

  // Initialize user object if new
  if (!chat.users[senderId]) {
    chat.users[senderId] = {
      count: 0,
      media: 0,
      audio: 0,
      stickers: 0,
      lastActive: Date.now(),
    };
  }

  const u = chat.users[senderId];
  u.count += 1;
  u.lastActive = Date.now();

  // Categorize incoming message types
  const m = msg?.message || {};
  if (m.imageMessage || m.videoMessage || m.documentMessage) {
    chat.breakdown.media += 1;
    u.media = (u.media || 0) + 1;
  } else if (m.audioMessage) {
    chat.breakdown.audio += 1;
    u.audio = (u.audio || 0) + 1;
  } else if (m.stickerMessage) {
    chat.breakdown.stickers += 1;
    u.stickers = (u.stickers || 0) + 1;
  } else {
    chat.breakdown.text += 1;
  }

  queueSaveStats();
}

// ---------------------------------------------------------------------------
// Main Command Handler
// ---------------------------------------------------------------------------

export async function handleStatsCommands(sock, jid, senderId, rawText, msg) {
  const text = rawText.trim().toLowerCase();
  const args = text.split(/\s+/);
  const command = args[0];

  if (!["!stats", "!topmembers", "!mystats", "!leaderboard", "!resetstats"].includes(command)) {
    return false;
  }

  const db = loadStats();

  // Handle Stats Reset (OWNER ONLY)
  if (command === "!resetstats") {
    if (!msg.key.fromMe) {
      await sock.sendMessage(jid, { text: "❌ Only the bot owner can reset chat statistics." }, { quoted: msg });
      return true;
    }
    delete db[jid];
    queueSaveStats();
    await sock.sendMessage(jid, { text: "🧹 *Chat statistics have been reset for this group!*" }, { quoted: msg });
    return true;
  }

  const chatData = db[jid];

  if (!chatData || chatData.totalMessages === 0) {
    await sock.sendMessage(
      jid,
      { text: "📊 *No message statistics recorded for this chat yet!* Send a message to start." },
      { quoted: msg }
    );
    return true;
  }

  // 1. Personal Stats (!mystats)
  if (command === "!mystats") {
    const u = chatData.users[senderId] || { count: 0, media: 0, audio: 0, stickers: 0 };
    const count = u.count || 0;
    const total = chatData.totalMessages;
    const percent = total > 0 ? ((count / total) * 100).toFixed(1) : "0.0";
    const bar = makeProgressBar(parseFloat(percent), 10);
    const rank = getRankTitle(count, total);
    const senderName = contactStore.get(senderId) || `@${senderId.split("@")[0]}`;

    let myStatsText =
      `╭━━━ 👤 *PERSONAL ACTIVITY CARD* ━━━╮\n\n` +
      `👤 *Member:* ${senderName}\n` +
      `🏅 *Rank Title:* \`${rank}\`\n\n` +
      `💬 *Total Sent:* *${count.toLocaleString()}* msgs\n` +
      `📊 *Chat Contribution:* *${percent}%*\n` +
      `\`[${bar}]\`\n\n` +
      `📁 *Images/Videos:* ${u.media || 0}\n` +
      `🎙️ *Voice Notes:* ${u.audio || 0}\n` +
      `🎨 *Stickers:* ${u.stickers || 0}\n\n` +
      `🌐 *Group Total:* ${total.toLocaleString()} msgs\n` +
      `╰────────────────────────────╯`;

    await sock.sendMessage(jid, { text: myStatsText, mentions: [senderId] }, { quoted: msg });
    return true;
  }

  // 2. Full Media Details Subcommand (!stats full / !stats details)
  if (args[1] === "full" || args[1] === "details") {
    const b = chatData.breakdown || { text: 0, media: 0, audio: 0, stickers: 0 };
    const total = chatData.totalMessages;

    const textPct = ((b.text / total) * 100).toFixed(1);
    const mediaPct = ((b.media / total) * 100).toFixed(1);
    const audioPct = ((b.audio / total) * 100).toFixed(1);
    const stickerPct = ((b.stickers / total) * 100).toFixed(1);

    let detailsText =
      `╭━━━ 📑 *DETAILED CONTENT BREAKDOWN* ━━━╮\n\n` +
      `💬 *Text Messages:* ${b.text.toLocaleString()} _(${textPct}%)_\n` +
      `\`[${makeProgressBar(parseFloat(textPct), 8)}]\`\n\n` +
      `🖼️ *Photos & Videos:* ${b.media.toLocaleString()} _(${mediaPct}%)_\n` +
      `\`[${makeProgressBar(parseFloat(mediaPct), 8)}]\`\n\n` +
      `🎙️ *Voice Audios:* ${b.audio.toLocaleString()} _(${audioPct}%)_\n` +
      `\`[${makeProgressBar(parseFloat(audioPct), 8)}]\`\n\n` +
      `🎨 *Stickers:* ${b.stickers.toLocaleString()} _(${stickerPct}%)_\n` +
      `\`[${makeProgressBar(parseFloat(stickerPct), 8)}]\`\n\n` +
      `🏆 *Total Tracked Items:* ${total.toLocaleString()}\n` +
      `╰────────────────────────────╯`;

    await sock.sendMessage(jid, { text: detailsText }, { quoted: msg });
    return true;
  }

  // 3. Default Leaderboard & Chat Activity Dashboard (!stats, !topmembers, !leaderboard)
  const sortedUsers = Object.entries(chatData.users)
    .sort(([, a], [, b]) => (b.count || b) - (a.count || a))
    .slice(0, 10);

  const mentions = sortedUsers.map(([uId]) => uId);
  const activity = getChatActivityStatus(chatData.totalMessages, chatData.createdAt || Date.now());

  let leaderText =
    `╭━━━ 📊 *CHAT ACTIVITY DASHBOARD* ━━━╮\n\n` +
    `💬 *Total Messages:* \`${chatData.totalMessages.toLocaleString()}\`\n` +
    `👥 *Active Members:* \`${Object.keys(chatData.users).length}\` tracked\n` +
    `⚡ *Activity Level:* \`${activity.label}\` (~${activity.avg} msgs/day)\n\n` +
    `🏆 *TOP 10 MOST ACTIVE MEMBERS:*\n` +
    `────────────────────────────\n`;

  sortedUsers.forEach(([uId, data], index) => {
    const count = typeof data === "number" ? data : data.count;
    const percent = ((count / chatData.totalMessages) * 100).toFixed(1);
    const bar = makeProgressBar(parseFloat(percent), 6);
    const name = contactStore.get(uId) || `@${uId.split("@")[0]}`;
    const medal = index === 0 ? "🥇" : index === 1 ? "🥈" : index === 2 ? "🥉" : `*${index + 1}.*`;

    leaderText += `${medal} *${name}*\n`;
    leaderText += `   └ \`[${bar}]\` *${count.toLocaleString()}* msgs _(${percent}%)_\n\n`;
  });

  leaderText += `👉 *Type \`!mystats\` for your card or \`!stats full\` for content breakdown.*`;

  await sock.sendMessage(jid, { text: leaderText, mentions }, { quoted: msg });
  return true;
}