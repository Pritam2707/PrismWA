import { downloadMediaMessage } from "@whiskeysockets/baileys";
import { existsSync, mkdirSync, readFileSync, writeFileSync, unlinkSync } from "node:fs";
import path from "node:path";
import { resendExtractedMessage, getIncomingText } from "../utils/helpers.js";
import { messageStore, contactStore } from "../store/messageStore.js";

const DATA_DIR = path.join(process.cwd(), "data");
const MEDIA_DIR = path.join(DATA_DIR, "media");
const DELETED_MSG_FILE = path.join(DATA_DIR, "deleted_messages.json");
const DELETED_VO_FILE = path.join(DATA_DIR, "deleted_viewonce.json");

// Ensure data directories exist
if (!existsSync(DATA_DIR)) mkdirSync(DATA_DIR, { recursive: true });
if (!existsSync(MEDIA_DIR)) mkdirSync(MEDIA_DIR, { recursive: true });

// JSON Store Helpers
function loadJsonData(filePath) {
  try {
    if (existsSync(filePath)) {
      return JSON.parse(readFileSync(filePath, "utf-8"));
    }
  } catch (e) {
    console.error(`Failed to read file ${filePath}:`, e);
  }
  return {};
}

function saveJsonData(filePath, data) {
  try {
    writeFileSync(filePath, JSON.stringify(data, null, 2), "utf-8");
  } catch (e) {
    console.error(`Failed to save file ${filePath}:`, e);
  }
}

// Helper: Safely delete local file
function safeUnlink(filePath) {
  if (filePath && existsSync(filePath)) {
    try {
      unlinkSync(filePath);
    } catch (e) {
      console.error(`Failed to delete local media ${filePath}:`, e.message);
    }
  }
}

/**
 * Helper: Check if command executor has permission (Owner/Host only)
 */
function isAuthorizedOwner(sock, msg) {
  if (msg.key?.fromMe) return true;

  const botJid = sock.user?.id ? sock.user.id.split(":")[0] + "@s.whatsapp.net" : null;
  const senderJid = msg.key?.participant || msg.key?.remoteJid;

  return !!(botJid && senderJid && (senderJid === botJid || senderJid.startsWith(botJid.split("@")[0])));
}

// Check if message is View Once
export function isViewOnce(msg) {
  if (!msg) return false;
  const m = msg.message || msg;
  return !!(
    m.viewOnceMessage ||
    m.viewOnceMessageV2 ||
    m.viewOnceMessageV2Extension ||
    m.imageMessage?.viewOnce ||
    m.videoMessage?.viewOnce ||
    m.audioMessage?.viewOnce
  );
}

// Get recent View Once from cache
function findLatestViewOnce(jid) {
  const entries = Array.from(messageStore.values()).reverse();
  for (const { msg } of entries) {
    if (msg.key?.remoteJid === jid && isViewOnce(msg)) {
      return msg;
    }
  }
  return null;
}

// Clean Sender Display Name
function getSenderFormatted(senderJid) {
  if (contactStore.has(senderJid)) {
    return contactStore.get(senderJid);
  }
  const phone = senderJid.split("@")[0];
  return `+${phone}`;
}

// Format message content type snippet for clean UI list
function getMessageSnippet(item) {
  const unwrapped = item.message?.viewOnceMessage?.message || item.message || {};
  const text = getIncomingText({ message: unwrapped }) || "";
  
  if (unwrapped.imageMessage) return text ? `📷 "${text.slice(0, 20)}..."` : "📷 [Photo]";
  if (unwrapped.videoMessage) return text ? `🎥 "${text.slice(0, 20)}..."` : "🎥 [Video]";
  if (unwrapped.audioMessage) return "🎵 [Voice Note]";
  if (unwrapped.stickerMessage) return "🎭 [Sticker]";
  if (text) return `💬 "${text.slice(0, 25)}${text.length > 25 ? "..." : ""}"`;
  return "💬 [Message]";
}

/**
 * Downloads media payload directly to disk (including stickers) to prevent expiration
 */
async function saveMediaLocally(msg) {
  try {
    const m = msg.message || msg;
    const hasMedia = !!(
      m.imageMessage ||
      m.videoMessage ||
      m.audioMessage ||
      m.stickerMessage ||
      m.documentMessage ||
      m.viewOnceMessage?.message?.imageMessage ||
      m.viewOnceMessage?.message?.videoMessage ||
      m.viewOnceMessage?.message?.audioMessage ||
      m.viewOnceMessage?.message?.stickerMessage
    );

    if (!hasMedia) return null;

    const buffer = await downloadMediaMessage(msg, "buffer", {}, { reconnect: async () => {} });

    let ext = ".bin";
    if (m.imageMessage || m.viewOnceMessage?.message?.imageMessage) ext = ".jpg";
    else if (m.videoMessage || m.viewOnceMessage?.message?.videoMessage) ext = ".mp4";
    else if (m.audioMessage || m.viewOnceMessage?.message?.audioMessage) ext = ".mp3";
    else if (m.stickerMessage || m.viewOnceMessage?.message?.stickerMessage) ext = ".webp";

    const fileName = `${msg.key.id}${ext}`;
    const filePath = path.join(MEDIA_DIR, fileName);

    writeFileSync(filePath, buffer);
    return filePath;
  } catch (err) {
    console.error("Failed to download media locally:", err.message);
    return null;
  }
}

/**
 * Call from delete listener in index.js to persist deleted items
 */
export async function recordDeletedMessage(originalMsg) {
  if (!originalMsg || !originalMsg.key) return;

  const jid = originalMsg.key.remoteJid;
  if (!jid) return;

  const isVO = isViewOnce(originalMsg);
  const targetFile = isVO ? DELETED_VO_FILE : DELETED_MSG_FILE;
  const db = loadJsonData(targetFile);

  if (!db[jid]) db[jid] = [];

  const msgId = originalMsg.key.id;
  const exists = db[jid].some((item) => item.key?.id === msgId);

  if (!exists) {
    const senderJid = originalMsg.key.participant || jid;
    const localFilePath = await saveMediaLocally(originalMsg);

    db[jid].push({
      key: originalMsg.key,
      message: originalMsg.message,
      localFilePath,
      senderJid,
      senderName: getSenderFormatted(senderJid),
      deletedAt: Date.now(),
    });

    saveJsonData(targetFile, db);
  }
}

/**
 * Common Delete Logic Processor
 */
async function handleDeleteOperations(sock, jid, db, targetFile, deleteArg, msg, label) {
  // Strict Permission Check: Only the bot owner can trigger deletions
  if (!isAuthorizedOwner(sock, msg)) {
    await sock.sendMessage(
      jid,
      { text: "⛔ *Access Denied:* Only the bot owner is permitted to delete saved log records." },
      { quoted: msg }
    );
    return true;
  }

  const chatList = db[jid] || [];

  if (chatList.length === 0) {
    await sock.sendMessage(jid, { text: `📂 *No recorded ${label} entries to delete.*` }, { quoted: msg });
    return true;
  }

  // 1. Delete All (`!deleted del all` / `!vv del all`)
  if (deleteArg === "all") {
    chatList.forEach((item) => safeUnlink(item.localFilePath));
    delete db[jid];
    saveJsonData(targetFile, db);

    await sock.sendMessage(
      jid,
      { text: `🗑️ *Cleared all recorded ${label} entries for this chat!*` },
      { quoted: msg }
    );
    return true;
  }

  // 2. Delete Range (`!deleted del 1-9` / `!vv del 1-9`)
  if (deleteArg.includes("-")) {
    const [startStr, endStr] = deleteArg.split("-");
    const start = parseInt(startStr, 10);
    const end = parseInt(endStr, 10);

    if (isNaN(start) || isNaN(end) || start < 1 || end > chatList.length || start > end) {
      await sock.sendMessage(
        jid,
        { text: `⚠️ *Invalid range!* Ensure range is between \`1-${chatList.length}\`.` },
        { quoted: msg }
      );
      return true;
    }

    const itemsToDelete = chatList.slice(start - 1, end);
    itemsToDelete.forEach((item) => safeUnlink(item.localFilePath));

    db[jid].splice(start - 1, end - start + 1);
    saveJsonData(targetFile, db);

    await sock.sendMessage(
      jid,
      { text: `🗑️ *Deleted entries [${start}] to [${end}] inclusive!*` },
      { quoted: msg }
    );
    return true;
  }

  // 3. Delete Single Index (`!deleted del 5` / `!vv del 5`)
  const targetIdx = parseInt(deleteArg, 10) - 1;
  if (isNaN(targetIdx) || !chatList[targetIdx]) {
    await sock.sendMessage(
      jid,
      { text: `⚠️ *Invalid index!* Enter a valid number between \`1-${chatList.length}\`.` },
      { quoted: msg }
    );
    return true;
  }

  const removedItem = chatList[targetIdx];
  safeUnlink(removedItem.localFilePath);

  db[jid].splice(targetIdx, 1);
  saveJsonData(targetFile, db);

  await sock.sendMessage(
    jid,
    { text: `🗑️ *Successfully removed entry [${targetIdx + 1}]!*` },
    { quoted: msg }
  );
  return true;
}

// Format Clean UI List
function buildCleanUiList(chatList, title, getCmd, delCmd) {
  let listText = `╭━━━ 🗑️ *${title}* ━━━╮\n\n`;

  chatList.forEach((item, index) => {
    const timeStr = new Date(item.deletedAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    const snippet = getMessageSnippet(item);

    listText += `*#${index + 1}* • *${item.senderName}* \`(${timeStr})\`\n`;
    listText += `└ ${snippet}\n\n`;
  });

  listText += `─────────────────────────\n`;
  listText += `👉 *Get:* \`${getCmd} <num>\`\n`;
  listText += `❌ *Delete (Owner Only):* \`${delCmd} <num>\` | \`${delCmd} <1-9>\` | \`${delCmd} all\``;

  return listText;
}

// Main Command Handler
export async function handleViewOnceCommands(sock, jid, rawText, msg) {
  const text = rawText.trim().toLowerCase();
  const args = text.split(/\s+/);

  // -------------------------------------------------------------
  // 1. View Once Commands (!vv, !vv list, !vv get, !vv del, !vv rm)
  // -------------------------------------------------------------
  if (text.startsWith("!vv")) {
    const db = loadJsonData(DELETED_VO_FILE);
    const chatVOList = db[jid] || [];

    // Subcommand: !vv del <all|index|range>
    if (args[1] === "del" || args[1] === "rm") {
      const deleteArg = args[2];
      if (!deleteArg) {
        await sock.sendMessage(
          jid,
          { text: "⚠️ Usage: `!vv del all` | `!vv del 5` | `!vv del 1-9`" },
          { quoted: msg }
        );
        return true;
      }
      return await handleDeleteOperations(sock, jid, db, DELETED_VO_FILE, deleteArg, msg, "View Once");
    }

    // Subcommand: !vv list
    if (args[1] === "list") {
      if (chatVOList.length === 0) {
        await sock.sendMessage(
          jid,
          { text: "📂 *No persistent View Once messages recorded for this chat.*" },
          { quoted: msg }
        );
        return true;
      }

      const listText = buildCleanUiList(chatVOList, "SAVED VIEW ONCE LOG", "!vv get", "!vv del");
      await sock.sendMessage(
        jid,
        { text: listText, mentions: chatVOList.map((i) => i.senderJid) },
        { quoted: msg }
      );
      return true;
    }

    // Subcommand: !vv get <number>
    if (args[1] === "get" || (args[1] && !isNaN(args[1]))) {
      const targetIdx = parseInt(args[1] === "get" ? args[2] : args[1], 10) - 1;

      if (isNaN(targetIdx) || !chatVOList[targetIdx]) {
        await sock.sendMessage(
          jid,
          { text: "⚠️ *Invalid index!* Use `!vv list` to view valid entry numbers." },
          { quoted: msg }
        );
        return true;
      }

      const savedItem = chatVOList[targetIdx];
      const tag = `*[SAVED DELETED VIEW ONCE]*\n👤 *From:* ${savedItem.senderName}`;

      if (savedItem.localFilePath && existsSync(savedItem.localFilePath)) {
        const fileBuffer = readFileSync(savedItem.localFilePath);
        const ext = path.extname(savedItem.localFilePath);

        if (ext === ".jpg") {
          await sock.sendMessage(jid, { image: fileBuffer, caption: tag }, { quoted: msg });
        } else if (ext === ".mp4") {
          await sock.sendMessage(jid, { video: fileBuffer, caption: tag }, { quoted: msg });
        } else if (ext === ".mp3") {
          await sock.sendMessage(jid, { audio: fileBuffer, mimetype: "audio/mp4" }, { quoted: msg });
        } else if (ext === ".webp") {
          await sock.sendMessage(jid, { text: tag }, { quoted: msg });
          await sock.sendMessage(jid, { sticker: fileBuffer }, { quoted: msg });
        }
      } else {
        const success = await resendExtractedMessage(sock, jid, savedItem, msg, tag);
        if (!success) {
          await sock.sendMessage(jid, { text: "❌ Failed to retrieve media payload." }, { quoted: msg });
        }
      }
      return true;
    }

    // Fallback: Immediate !vv recovery
    const quotedId = msg.message?.extendedTextMessage?.contextInfo?.stanzaId;
    const quotedParticipant = msg.message?.extendedTextMessage?.contextInfo?.participant;
    let targetMsg = null;

    if (quotedId && messageStore.has(quotedId)) {
      targetMsg = messageStore.get(quotedId).msg;
    }

    if (!targetMsg) {
      const quoted = msg.message?.extendedTextMessage?.contextInfo?.quotedMessage;
      if (quoted) {
        targetMsg = {
          key: { id: quotedId, remoteJid: jid, participant: quotedParticipant },
          message: quoted,
        };
      }
    }

    if (!targetMsg || !isViewOnce(targetMsg)) {
      targetMsg = findLatestViewOnce(jid);
    }

    if (targetMsg) {
      const success = await resendExtractedMessage(sock, jid, targetMsg, msg, "*[RESTORED VIEW ONCE]*");
      if (!success) {
        await sock.sendMessage(
          jid,
          { text: "Failed to retrieve View Once media. Cache may have expired or message contains no media." },
          { quoted: msg }
        );
      }
    } else {
      await sock.sendMessage(
        jid,
        { text: "No recent View Once message found in cache for this chat." },
        { quoted: msg }
      );
    }
    return true;
  }

  // -------------------------------------------------------------
  // 2. Persistent Deleted Messages (!deleted, !deleted list, !deleted del, !deleted rm, !deleted get)
  // -------------------------------------------------------------
  if (text.startsWith("!deleted")) {
    const db = loadJsonData(DELETED_MSG_FILE);
    const chatDeletedList = db[jid] || [];

    // Subcommand: !deleted del <all|index|range>
    if (args[1] === "del" || args[1] === "rm") {
      const deleteArg = args[2];
      if (!deleteArg) {
        await sock.sendMessage(
          jid,
          { text: "⚠️ Usage: `!deleted del all` | `!deleted del 5` | `!deleted del 1-9`" },
          { quoted: msg }
        );
        return true;
      }
      return await handleDeleteOperations(sock, jid, db, DELETED_MSG_FILE, deleteArg, msg, "Deleted Message");
    }

    // Subcommand: !deleted list
    if (args[1] === "list" || args.length === 1) {
      if (chatDeletedList.length === 0) {
        await sock.sendMessage(
          jid,
          { text: "📂 *No stored deleted messages found for this chat.*" },
          { quoted: msg }
        );
        return true;
      }

      const listText = buildCleanUiList(chatDeletedList, "DELETED MESSAGES LOG", "!deleted get", "!deleted del");
      await sock.sendMessage(
        jid,
        { text: listText, mentions: chatDeletedList.map((i) => i.senderJid) },
        { quoted: msg }
      );
      return true;
    }

    // Subcommand: !deleted get <number>
    if (args[1] === "get" || (args[1] && !isNaN(args[1]))) {
      const targetIdx = parseInt(args[1] === "get" ? args[2] : args[1], 10) - 1;

      if (isNaN(targetIdx) || !chatDeletedList[targetIdx]) {
        await sock.sendMessage(
          jid,
          { text: "⚠️ *Invalid index!* Type `!deleted list` to check valid numbers." },
          { quoted: msg }
        );
        return true;
      }

      const savedItem = chatDeletedList[targetIdx];
      const tag = `*[RECOVERED DELETED MESSAGE]*\n👤 *From:* ${savedItem.senderName}`;

      if (savedItem.localFilePath && existsSync(savedItem.localFilePath)) {
        const fileBuffer = readFileSync(savedItem.localFilePath);
        const ext = path.extname(savedItem.localFilePath);

        if (ext === ".jpg") {
          await sock.sendMessage(jid, { image: fileBuffer, caption: tag }, { quoted: msg });
        } else if (ext === ".mp4") {
          await sock.sendMessage(jid, { video: fileBuffer, caption: tag }, { quoted: msg });
        } else if (ext === ".mp3") {
          await sock.sendMessage(jid, { audio: fileBuffer, mimetype: "audio/mp4" }, { quoted: msg });
        } else if (ext === ".webp") {
          await sock.sendMessage(jid, { text: tag }, { quoted: msg });
          await sock.sendMessage(jid, { sticker: fileBuffer }, { quoted: msg });
        }
      } else {
        await resendExtractedMessage(sock, jid, savedItem, msg, tag);
      }
      return true;
    }

    return true;
  }

  return false;
}