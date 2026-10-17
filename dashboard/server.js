import express from "express";
import path from "node:path";
import fs from "node:fs";
import fsp from "node:fs/promises";
import os from "node:os";
import {
  isAutoReplyEnabled,
  getAutoReplyMode,
  setAutoReplyMode,
  getAutoReplyMessage,
  setAutoReplyState,
  setAutoReplyCustomMessage,
  addAutoReplyTarget,
  removeAutoReplyTarget,
  getAutoReplyTargets,
  addAutoReplyTrigger,
  removeAutoReplyTrigger,
  getAutoReplyTriggers,
  flushAutoReplyNotes,
} from "../services/autoreplyStore.js";
import {
  messageStore,
  activeAntiDeleteChats,
  activeFeatureChats,
  contactStore,
  setContact,
  getDisplayName,
  savePersistentData,
} from "../store/messageStore.js";
import { getMergedPersonas, getMergedVoiceIds } from "../commands/custom.js";
import { solveWithGemini } from "../services/gemini.js";
import {
  logEvent,
  getRecentLogs,
  clearLogs,
  registerSseClient,
} from "./logger.js";

// Global Bot Context passed from index.js
let botContext = {
  sock: null,
  getIsSleeping: () => false,
  wakeUpBot: () => {},
  enterSleepMode: () => {},
  qrCodeData: null,
  connectionState: "disconnected",
  startTime: Date.now(),
};

export function setBotContext(ctx) {
  botContext = { ...botContext, ...ctx };
}

const DATA_DIR = path.join(process.cwd(), "data");
const MEDIA_DIR = path.join(DATA_DIR, "media");
const STICKERS_DIR = path.join(process.cwd(), "stickers");
const STATS_FILE = path.join(DATA_DIR, "chat_stats.json");
const NOTES_FILE = path.join(process.cwd(), "notes.json");
const DELETED_MSG_FILE = path.join(DATA_DIR, "deleted_messages.json");
const DELETED_VO_FILE = path.join(DATA_DIR, "deleted_viewonce.json");
const REMINDERS_FILE = path.join(DATA_DIR, "reminders.json");
const DYNAMIC_STORE_PATH = path.join(DATA_DIR, "dynamic_store.json");
const WELCOME_DATA_PATH = path.join(DATA_DIR, "welcome_settings.json");
const MUTE_FILE = path.join(DATA_DIR, "muted_users.json");

/**
 * Helper: Safely calculate directory size
 */
async function getDirSize(dirPath) {
  if (!fs.existsSync(dirPath)) return 0;
  let totalSize = 0;
  try {
    const files = await fsp.readdir(dirPath, { withFileTypes: true });
    for (const file of files) {
      const fullPath = path.join(dirPath, file.name);
      if (file.isDirectory()) {
        totalSize += await getDirSize(fullPath);
      } else {
        const stats = await fsp.stat(fullPath);
        totalSize += stats.size;
      }
    }
  } catch (err) {
    return 0;
  }
  return totalSize;
}

/**
 * Helper: Safely calculate file size
 */
async function getFileSize(filePath) {
  if (!fs.existsSync(filePath)) return 0;
  try {
    const stats = await fsp.stat(filePath);
    return stats.size;
  } catch {
    return 0;
  }
}

/**
 * Helper: Read JSON safely
 */
function readJsonSafe(filePath, fallback = {}) {
  try {
    if (fs.existsSync(filePath)) {
      return JSON.parse(fs.readFileSync(filePath, "utf-8"));
    }
  } catch {}
  return fallback;
}

/**
 * Helper: Format Bytes
 */
function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

/**
 * Helper: Format Phone Numbers into human readable text
 */
function formatPhone(jid) {
  if (!jid) return "";
  const raw = jid.split("@")[0].split(":")[0];
  if (raw.length === 12 && raw.startsWith("91")) {
    return `+91 ${raw.slice(2, 7)} ${raw.slice(7)}`;
  }
  if (raw.length >= 10) {
    return `+${raw}`;
  }
  return raw;
}

/**
 * Helper: Resolve real friendly name for chat or contact
 */
async function resolveChatName(jid) {
  if (!jid) return "Unknown";
  if (contactStore.has(jid)) return contactStore.get(jid);

  if (jid.endsWith("@g.us") && botContext.sock) {
    try {
      const meta = await botContext.sock.groupMetadata(jid);
      if (meta?.subject) {
        setContact(jid, meta.subject);
        return meta.subject;
      }
    } catch {}
  }

  return getDisplayName(jid);
}

export function startDashboardServer(port = 3000) {
  const app = express();

  app.use(express.json({ limit: "10mb" }));

  // Serve static assets from dashboard/public
  const publicDir = path.join(process.cwd(), "dashboard", "public");
  app.use(express.static(publicDir));

  // Serve media cache safely for audit vault thumbnail previews
  app.use("/data/media", express.static(MEDIA_DIR));
  app.use("/stickers", express.static(STICKERS_DIR));

  // ---------------------------------------------------------------------------
  // 1. SYSTEM & BOT STATUS
  // ---------------------------------------------------------------------------
  app.get("/api/status", (req, res) => {
    const memory = process.memoryUsage();
    const uptimeSecs = Math.floor((Date.now() - botContext.startTime) / 1000);
    const sockUser = botContext.sock?.user;
    const reminders = readJsonSafe(REMINDERS_FILE, []);
    const muted = readJsonSafe(MUTE_FILE, {});
    let totalMutedCount = 0;
    for (const list of Object.values(muted)) {
      if (Array.isArray(list)) totalMutedCount += list.length;
    }

    res.json({
      status: "ok",
      bot: {
        isSleeping: botContext.getIsSleeping ? botContext.getIsSleeping() : false,
        connectionState: botContext.connectionState || "unknown",
        qrCode: botContext.qrCodeData || null,
        user: sockUser
          ? {
              id: sockUser.id,
              name: sockUser.name || "PrismBot",
              phone: formatPhone(sockUser.id),
            }
          : null,
        uptimeSecs,
        totalCachedMessages: messageStore.size,
        activeGamesCount: 0,
        antiDeleteChatsCount: activeAntiDeleteChats.size,
        featureChatsCount: activeFeatureChats.size,
        knownContactsCount: contactStore.size,
        remindersCount: reminders.length,
        mutedCount: totalMutedCount,
      },
      system: {
        platform: os.platform(),
        arch: os.arch(),
        nodeVersion: process.version,
        cpuCount: os.cpus().length,
        freeMemMB: Math.round(os.freemem() / (1024 * 1024)),
        totalMemMB: Math.round(os.totalmem() / (1024 * 1024)),
        heapUsedMB: Math.round(memory.heapUsed / (1024 * 1024)),
        rssMB: Math.round(memory.rss / (1024 * 1024)),
      },
    });
  });

  // ---------------------------------------------------------------------------
  // 2. BOT CONTROLS & PERMISSIONS
  // ---------------------------------------------------------------------------
  app.post("/api/control/toggle-sleep", (req, res) => {
    const isSleeping = botContext.getIsSleeping ? botContext.getIsSleeping() : false;
    if (isSleeping) {
      botContext.wakeUpBot?.();
      logEvent("SYSTEM", "Bot manually woken up from Dashboard GUI");
    } else {
      botContext.enterSleepMode?.();
      logEvent("SYSTEM", "Bot manually placed into Sleep Mode from Dashboard GUI");
    }
    res.json({ success: true, isSleeping: botContext.getIsSleeping ? botContext.getIsSleeping() : false });
  });

  app.post("/api/control/toggle-antidelete", (req, res) => {
    const { jid, enable } = req.body;
    if (!jid) return res.status(400).json({ error: "Missing JID" });

    const name = getDisplayName(jid);
    if (enable) {
      activeAntiDeleteChats.add(jid);
      logEvent("SYSTEM", `Anti-Delete enabled for ${name} (${jid}) via Dashboard`);
    } else {
      activeAntiDeleteChats.delete(jid);
      logEvent("SYSTEM", `Anti-Delete disabled for ${name} (${jid}) via Dashboard`);
    }
    savePersistentData();
    res.json({ success: true, activeAntiDeleteChats: Array.from(activeAntiDeleteChats) });
  });

  app.post("/api/control/toggle-features", (req, res) => {
    const { jid, enable } = req.body;
    if (!jid) return res.status(400).json({ error: "Missing JID" });

    const name = getDisplayName(jid);
    if (enable) {
      activeFeatureChats.add(jid);
      logEvent("SYSTEM", `Interactive features enabled for ${name} (${jid}) via Dashboard`);
    } else {
      activeFeatureChats.delete(jid);
      logEvent("SYSTEM", `Interactive features disabled for ${name} (${jid}) via Dashboard`);
    }
    savePersistentData();
    res.json({ success: true, activeFeatureChats: Array.from(activeFeatureChats) });
  });

  // ---------------------------------------------------------------------------
  // 3. CHAT DIRECTORY & CONTACT NICKNAMES
  // ---------------------------------------------------------------------------
  app.get("/api/chats", async (req, res) => {
    const statsData = readJsonSafe(STATS_FILE, {});
    const chatMap = new Map();

    // Collect all known chat JIDs
    const allJids = new Set([
      ...Object.keys(statsData),
      ...activeAntiDeleteChats,
      ...activeFeatureChats,
      ...contactStore.keys(),
    ]);

    for (const jid of allJids) {
      const stats = statsData[jid] || {};
      const isGroup = jid.endsWith("@g.us");
      const name = await resolveChatName(jid);

      chatMap.set(jid, {
        jid,
        name,
        phone: formatPhone(jid),
        isGroup,
        totalMessages: stats.totalMessages || 0,
        antiDelete: activeAntiDeleteChats.has(jid),
        features: activeFeatureChats.has(jid),
        lastActive: stats.createdAt || Date.now(),
      });
    }

    // Sort: most active first
    const list = Array.from(chatMap.values()).sort((a, b) => b.totalMessages - a.totalMessages);
    res.json(list);
  });

  // Rename contact / set friendly nickname
  app.post("/api/contacts/rename", async (req, res) => {
    const { jid, name } = req.body;
    if (!jid || !name) return res.status(400).json({ error: "Missing JID or name" });

    setContact(jid, name.trim());
    logEvent("SUCCESS", `Renamed contact/chat ${jid} to "${name.trim()}" via Dashboard`);
    res.json({ success: true, jid, name: name.trim() });
  });

  app.get("/api/contacts", (req, res) => {
    const list = [];
    for (const [jid, name] of contactStore.entries()) {
      list.push({
        jid,
        name,
        phone: formatPhone(jid),
        isGroup: jid.endsWith("@g.us"),
      });
    }
    res.json(list);
  });

  // ---------------------------------------------------------------------------
  // 4. AUTO-REPLY / AWAY MANAGER
  // ---------------------------------------------------------------------------
  app.get("/api/autoreply", (req, res) => {
    const rawData = readJsonSafe(path.join(process.cwd(), "autoreply.json"), {
      enabled: isAutoReplyEnabled(),
      mode: getAutoReplyMode(),
      message: getAutoReplyMessage(),
      targets: getAutoReplyTargets(),
      triggers: Object.fromEntries(getAutoReplyTriggers()),
      notes: [],
    });

    res.json({
      enabled: isAutoReplyEnabled(),
      mode: getAutoReplyMode(),
      message: getAutoReplyMessage(),
      targets: getAutoReplyTargets().map((t) => ({ jid: t, name: getDisplayName(t), phone: formatPhone(t) })),
      triggers: Object.fromEntries(getAutoReplyTriggers()),
      notes: (rawData.notes || []).map((n) => ({
        ...n,
        senderName: getDisplayName(n.senderId || n.jid),
        chatName: getDisplayName(n.jid),
      })),
    });
  });

  app.post("/api/autoreply", async (req, res) => {
    const { enabled, mode, message, addTarget, removeTarget, addTrigger, removeTrigger } = req.body;

    if (typeof enabled === "boolean") {
      await setAutoReplyState(enabled);
    }
    if (mode && ["global", "target", "keyword"].includes(mode)) {
      await setAutoReplyMode(mode);
    }
    if (message && typeof message === "string") {
      await setAutoReplyCustomMessage(message);
    }
    if (addTarget) {
      await addAutoReplyTarget(addTarget);
    }
    if (removeTarget) {
      await removeAutoReplyTarget(removeTarget);
    }
    if (addTrigger && addTrigger.phrase && addTrigger.reply) {
      await addAutoReplyTrigger(addTrigger.phrase, addTrigger.reply);
    }
    if (removeTrigger) {
      await removeAutoReplyTrigger(removeTrigger);
    }

    logEvent("SYSTEM", "Auto-Reply settings updated from Dashboard GUI");
    res.json({ success: true });
  });

  app.post("/api/autoreply/flush", async (req, res) => {
    const flushed = await flushAutoReplyNotes();
    logEvent("SYSTEM", `Flushed ${flushed.length} auto-reply visitor notes from Dashboard`);
    res.json({ success: true, count: flushed.length, notes: flushed });
  });

  // ---------------------------------------------------------------------------
  // 5. PERSONAS & LIVE TEST PLAYGROUND
  // ---------------------------------------------------------------------------
  app.get("/api/personas", (req, res) => {
    const personas = getMergedPersonas();
    const voices = getMergedVoiceIds();
    res.json({ personas, voices });
  });

  app.post("/api/personas", async (req, res) => {
    const { key, label, flavor, voiceKey, voiceId } = req.body;
    if (!key) return res.status(400).json({ error: "Missing persona key" });

    const raw = readJsonSafe(DYNAMIC_STORE_PATH, { personas: {}, voices: {}, stickerOverrides: {} });

    if (label && flavor) {
      raw.personas[key] = {
        prefix: `!${key}`,
        label,
        flavor,
        stickers: {},
      };
    }

    if (voiceKey && voiceId) {
      raw.voices[voiceKey] = voiceId;
    }

    await fsp.writeFile(DYNAMIC_STORE_PATH, JSON.stringify(raw, null, 2), "utf-8");
    logEvent("SUCCESS", `Custom Persona/Voice '${key}' added via Dashboard`);
    res.json({ success: true });
  });

  // Interactive Live Persona Testing directly in dashboard
  app.post("/api/personas/test", async (req, res) => {
    const { personaKey, prompt } = req.body;
    if (!personaKey || !prompt) return res.status(400).json({ error: "Missing persona or prompt" });

    const personas = getMergedPersonas();
    const persona = personas[personaKey] || personas.zero2;

    try {
      const result = await solveWithGemini({
        historyKey: `dashboard_test_${personaKey}`,
        persona,
        text: prompt,
      });

      res.json({
        success: true,
        answer: result.answer,
        mood: result.mood,
        stickerPath: persona.stickers?.[result.mood] || null,
        type: result.type,
      });
    } catch (err) {
      res.status(500).json({ error: err.message });
    }
  });

  // ---------------------------------------------------------------------------
  // 6. CHAT ACTIVITY ANALYTICS WITH RESOLVED NAMES
  // ---------------------------------------------------------------------------
  app.get("/api/stats", (req, res) => {
    const statsData = readJsonSafe(STATS_FILE, {});
    const enriched = {};

    for (const [jid, data] of Object.entries(statsData)) {
      const resolvedUsers = {};
      if (data.users) {
        for (const [uid, userStats] of Object.entries(data.users)) {
          const count = typeof userStats === "number" ? userStats : userStats.count;
          resolvedUsers[uid] = {
            count,
            name: getDisplayName(uid),
            phone: formatPhone(uid),
          };
        }
      }

      enriched[jid] = {
        ...data,
        chatName: getDisplayName(jid),
        isGroup: jid.endsWith("@g.us"),
        phone: formatPhone(jid),
        users: resolvedUsers,
      };
    }

    res.json(enriched);
  });

  app.delete("/api/stats/:jid", async (req, res) => {
    const { jid } = req.params;
    const statsData = readJsonSafe(STATS_FILE, {});
    delete statsData[jid];
    await fsp.writeFile(STATS_FILE, JSON.stringify(statsData, null, 2), "utf-8");
    logEvent("SYSTEM", `Reset stats for chat ${getDisplayName(jid)} via Dashboard`);
    res.json({ success: true });
  });

  // ---------------------------------------------------------------------------
  // 7. STORAGE & DISK CLEANER
  // ---------------------------------------------------------------------------
  app.get("/api/storage", async (req, res) => {
    const [
      mediaSize,
      stickersSize,
      deletedMsgsSize,
      deletedVoSize,
      dynamicStoreSize,
      statsSize,
      notesSize,
    ] = await Promise.all([
      getDirSize(MEDIA_DIR),
      getDirSize(STICKERS_DIR),
      getFileSize(DELETED_MSG_FILE),
      getFileSize(DELETED_VO_FILE),
      getFileSize(DYNAMIC_STORE_PATH),
      getFileSize(STATS_FILE),
      getFileSize(NOTES_FILE),
    ]);

    const totalCalculated =
      mediaSize + stickersSize + deletedMsgsSize + deletedVoSize + dynamicStoreSize + statsSize + notesSize;

    res.json({
      media: { size: mediaSize, formatted: formatBytes(mediaSize) },
      stickers: { size: stickersSize, formatted: formatBytes(stickersSize) },
      deletedMsgs: { size: deletedMsgsSize, formatted: formatBytes(deletedMsgsSize) },
      deletedVo: { size: deletedVoSize, formatted: formatBytes(deletedVoSize) },
      dynamicStore: { size: dynamicStoreSize, formatted: formatBytes(dynamicStoreSize) },
      stats: { size: statsSize, formatted: formatBytes(statsSize) },
      notes: { size: notesSize, formatted: formatBytes(notesSize) },
      total: { size: totalCalculated, formatted: formatBytes(totalCalculated) },
    });
  });

  app.post("/api/storage/clean", async (req, res) => {
    const { target } = req.body;
    let clearedCount = 0;

    if (target === "media" || target === "all") {
      if (fs.existsSync(MEDIA_DIR)) {
        const files = await fsp.readdir(MEDIA_DIR);
        for (const file of files) {
          await fsp.unlink(path.join(MEDIA_DIR, file)).catch(() => {});
          clearedCount++;
        }
      }
    }

    if (target === "stickers" || target === "all") {
      if (fs.existsSync(STICKERS_DIR)) {
        const files = await fsp.readdir(STICKERS_DIR);
        for (const file of files) {
          await fsp.unlink(path.join(STICKERS_DIR, file)).catch(() => {});
        }
      }
    }

    if (target === "deleted" || target === "all") {
      await fsp.writeFile(DELETED_MSG_FILE, "{}", "utf-8");
      await fsp.writeFile(DELETED_VO_FILE, "{}", "utf-8");
    }

    logEvent("SUCCESS", `Storage cleanup executed (target: ${target}) via Dashboard`);
    res.json({ success: true, clearedFiles: clearedCount });
  });

  // ---------------------------------------------------------------------------
  // 8. DELETED MESSAGES & VIEW ONCE AUDIT VAULT
  // ---------------------------------------------------------------------------
  app.get("/api/audit/deleted", (req, res) => {
    const deletedMsgs = readJsonSafe(DELETED_MSG_FILE, {});
    const deletedVo = readJsonSafe(DELETED_VO_FILE, {});

    const enrichVaultItems = (db) => {
      const out = {};
      if (Array.isArray(db)) {
        for (const item of db) {
          const jid = item.chatJid || item.chatJID || item.key?.remoteJid || "unknown";
          if (!out[jid]) out[jid] = [];
          const senderJid = item.senderJid || item.senderId || item.sender || item.key?.participant || jid;
          out[jid].push({
            ...item,
            chatName: getDisplayName(jid),
            senderName: item.pushName || item.senderName || getDisplayName(senderJid) || formatPhone(senderJid),
            phone: formatPhone(senderJid),
            text: item.text || item.message?.conversation || item.message?.extendedTextMessage?.text || "",
          });
        }
        return out;
      }
      for (const [jid, items] of Object.entries(db || {})) {
        if (!Array.isArray(items)) continue;
        out[jid] = items.map((item) => {
          const senderJid = item.senderJid || item.senderId || item.sender || item.key?.participant || jid;
          return {
            ...item,
            chatName: getDisplayName(jid),
            senderName: item.pushName || item.senderName || getDisplayName(senderJid) || formatPhone(senderJid),
            phone: formatPhone(senderJid),
            text: item.text || item.message?.conversation || item.message?.extendedTextMessage?.text || "",
          };
        });
      }
      return out;
    };

    res.json({
      deletedMsgs: enrichVaultItems(deletedMsgs),
      deletedVo: enrichVaultItems(deletedVo),
    });
  });

  app.delete("/api/audit/deleted/:type/:jid", async (req, res) => {
    const { type, jid } = req.params;
    const targetFile = type === "vo" ? DELETED_VO_FILE : DELETED_MSG_FILE;
    const db = readJsonSafe(targetFile, {});

    if (db[jid]) {
      db[jid].forEach((item) => {
        if (item.localFilePath && fs.existsSync(item.localFilePath)) {
          fs.unlinkSync(item.localFilePath);
        }
      });
      delete db[jid];
      await fsp.writeFile(targetFile, JSON.stringify(db, null, 2), "utf-8");
    }

    res.json({ success: true });
  });

  // ---------------------------------------------------------------------------
  // 9. GROUP WELCOMER STUDIO
  // ---------------------------------------------------------------------------
  app.get("/api/welcome", (req, res) => {
    const welcomeData = readJsonSafe(WELCOME_DATA_PATH, {});
    const enriched = {};
    for (const [jid, cfg] of Object.entries(welcomeData)) {
      enriched[jid] = {
        ...cfg,
        groupName: getDisplayName(jid),
        jid,
      };
    }
    res.json(enriched);
  });

  app.post("/api/welcome", async (req, res) => {
    const { jid, enabled, text, mediaPath } = req.body;
    if (!jid) return res.status(400).json({ error: "Missing group JID" });

    const welcomeData = readJsonSafe(WELCOME_DATA_PATH, {});
    welcomeData[jid] = {
      ...(welcomeData[jid] || {}),
      enabled: typeof enabled === "boolean" ? enabled : true,
      text: text !== undefined ? text : welcomeData[jid]?.text || "👋 Welcome @user to {group}!",
      mediaPath: mediaPath || welcomeData[jid]?.mediaPath || null,
      updatedAt: new Date().toISOString(),
    };

    await fsp.writeFile(WELCOME_DATA_PATH, JSON.stringify(welcomeData, null, 2), "utf-8");
    logEvent("SUCCESS", `Welcome banner updated for ${getDisplayName(jid)} via Dashboard`);
    res.json({ success: true, settings: welcomeData[jid] });
  });

  // ---------------------------------------------------------------------------
  // 10. MODERATION & MUTED USERS MANAGER
  // ---------------------------------------------------------------------------
  app.get("/api/moderation/mutes", (req, res) => {
    const mutedDB = readJsonSafe(MUTE_FILE, {});
    const list = [];

    for (const [jid, senders] of Object.entries(mutedDB)) {
      if (Array.isArray(senders)) {
        for (const senderId of senders) {
          list.push({
            jid,
            chatName: getDisplayName(jid),
            senderId,
            senderName: getDisplayName(senderId),
            phone: formatPhone(senderId),
            isGroup: jid.endsWith("@g.us"),
          });
        }
      }
    }
    res.json(list);
  });

  app.post("/api/moderation/mute", async (req, res) => {
    const { jid, senderId } = req.body;
    if (!jid || !senderId) return res.status(400).json({ error: "Missing JID or sender ID" });

    const cleanSender = senderId.includes("@") ? senderId : `${senderId.replace(/\D/g, "")}@s.whatsapp.net`;
    const mutedDB = readJsonSafe(MUTE_FILE, {});
    if (!mutedDB[jid]) mutedDB[jid] = [];
    if (!mutedDB[jid].includes(cleanSender)) {
      mutedDB[jid].push(cleanSender);
      await fsp.writeFile(MUTE_FILE, JSON.stringify(mutedDB, null, 2), "utf-8");
      logEvent("SYSTEM", `Muted user ${getDisplayName(cleanSender)} in ${getDisplayName(jid)} via Dashboard`);
    }
    res.json({ success: true });
  });

  app.delete("/api/moderation/mute/:jid/:senderId", async (req, res) => {
    const { jid, senderId } = req.params;
    const mutedDB = readJsonSafe(MUTE_FILE, {});
    if (mutedDB[jid]) {
      mutedDB[jid] = mutedDB[jid].filter((s) => s !== senderId);
      await fsp.writeFile(MUTE_FILE, JSON.stringify(mutedDB, null, 2), "utf-8");
      logEvent("SYSTEM", `Unmuted user ${getDisplayName(senderId)} in ${getDisplayName(jid)} via Dashboard`);
    }
    res.json({ success: true });
  });

  // ---------------------------------------------------------------------------
  // 11. NOTES & VIRTUAL FILE SYSTEM EXPLORER
  // ---------------------------------------------------------------------------
  app.get("/api/notes", (req, res) => {
    const notesData = readJsonSafe(NOTES_FILE, {});
    res.json(notesData);
  });

  app.post("/api/notes", async (req, res) => {
    const { path: notePath, content } = req.body;
    if (!notePath || !content) return res.status(400).json({ error: "Missing path or content" });

    const notes = readJsonSafe(NOTES_FILE, {});
    const cleanPath = notePath.startsWith("root/") ? notePath : `root/public/${notePath}`;

    notes[cleanPath] = {
      content,
      mediaType: "TXT",
      mimetype: "text/plain",
      fileName: path.basename(cleanPath),
      mediaData: null,
      isPrivate: cleanPath.startsWith("root/private/"),
      timestamp: new Date().toISOString(),
    };

    await fsp.writeFile(NOTES_FILE, JSON.stringify(notes, null, 2), "utf-8");
    logEvent("SUCCESS", `Note '${cleanPath}' saved via Dashboard GUI`);
    res.json({ success: true });
  });

  app.delete("/api/notes", async (req, res) => {
    const { path: targetPath } = req.body;
    if (!targetPath) return res.status(400).json({ error: "Missing target note path" });

    const notes = readJsonSafe(NOTES_FILE, {});
    if (notes[targetPath]) {
      delete notes[targetPath];
    } else {
      const prefix = targetPath.endsWith("/") ? targetPath : `${targetPath}/`;
      for (const key of Object.keys(notes)) {
        if (key.startsWith(prefix)) {
          delete notes[key];
        }
      }
    }

    await fsp.writeFile(NOTES_FILE, JSON.stringify(notes, null, 2), "utf-8");
    logEvent("SYSTEM", `Note '${targetPath}' removed via Dashboard GUI`);
    res.json({ success: true });
  });

  // ---------------------------------------------------------------------------
  // 12. REMINDERS & SCHEDULER
  // ---------------------------------------------------------------------------
  app.get("/api/reminders", (req, res) => {
    const reminders = readJsonSafe(REMINDERS_FILE, []);
    const enriched = reminders.map((r) => ({
      ...r,
      chatName: getDisplayName(r.jid),
      senderName: getDisplayName(r.senderId),
    }));
    res.json(enriched);
  });

  app.post("/api/reminders", async (req, res) => {
    const { jid, text, minutes } = req.body;
    if (!jid || !text || !minutes) return res.status(400).json({ error: "Missing jid, text, or minutes" });

    const reminders = readJsonSafe(REMINDERS_FILE, []);
    const delayMs = parseInt(minutes, 10) * 60 * 1000;
    const item = {
      id: Date.now().toString(),
      jid,
      senderId: jid,
      text,
      targetTime: Date.now() + delayMs,
      createdAt: Date.now(),
    };

    reminders.push(item);
    await fsp.writeFile(REMINDERS_FILE, JSON.stringify(reminders, null, 2), "utf-8");
    logEvent("SUCCESS", `Reminder scheduled for ${getDisplayName(jid)} in ${minutes}m via Dashboard`);
    res.json({ success: true, item });
  });

  app.delete("/api/reminders/:id", async (req, res) => {
    const { id } = req.params;
    let reminders = readJsonSafe(REMINDERS_FILE, []);
    reminders = reminders.filter((r) => r.id !== id);
    await fsp.writeFile(REMINDERS_FILE, JSON.stringify(reminders, null, 2), "utf-8");
    res.json({ success: true });
  });

  // ---------------------------------------------------------------------------
  // 13. DIRECT MESSENGER & BULK BROADCAST SYSTEM
  // ---------------------------------------------------------------------------
  app.post("/api/message/send", async (req, res) => {
    const { jid, text } = req.body;
    if (!jid || !text) return res.status(400).json({ error: "Missing target JID or text message" });

    if (!botContext.sock) {
      return res.status(503).json({ error: "WhatsApp socket is currently disconnected" });
    }

    try {
      const cleanJid = jid.includes("@") ? jid : `${jid.replace(/\D/g, "")}@s.whatsapp.net`;
      await botContext.sock.sendMessage(cleanJid, { text });
      logEvent("MSG", `Direct message sent to ${getDisplayName(cleanJid)} (${cleanJid}) from Dashboard`);
      res.json({ success: true });
    } catch (err) {
      logEvent("ERROR", `Failed to send message to ${jid}: ${err.message}`);
      res.status(500).json({ error: err.message });
    }
  });

  app.post("/api/message/broadcast", async (req, res) => {
    const { text, targetType } = req.body;
    if (!text) return res.status(400).json({ error: "Missing announcement text" });

    if (!botContext.sock) {
      return res.status(503).json({ error: "WhatsApp socket is currently disconnected" });
    }

    const statsData = readJsonSafe(STATS_FILE, {});
    const allKnown = new Set([
      ...Object.keys(statsData),
      ...activeFeatureChats,
      ...activeAntiDeleteChats,
    ]);

    const targets = [];
    for (const jid of allKnown) {
      const isGroup = jid.endsWith("@g.us");
      if (targetType === "groups" && isGroup) targets.push(jid);
      else if (targetType === "dms" && !isGroup) targets.push(jid);
      else if (targetType === "all") targets.push(jid);
    }

    let sent = 0, failed = 0;
    // Broadcast safely with interval
    for (const jid of targets) {
      try {
        await botContext.sock.sendMessage(jid, { text: `📢 *[BROADCAST ANNOUNCEMENT]*\n\n${text}` });
        sent++;
        await new Promise((r) => setTimeout(r, 1200)); // Rate limit safety
      } catch (e) {
        failed++;
      }
    }

    logEvent("SUCCESS", `Broadcast finished: ${sent} sent, ${failed} failed (Target: ${targetType})`);
    res.json({ success: true, sent, failed, total: targets.length });
  });

  // ---------------------------------------------------------------------------
  // 14. LOGS & REAL-TIME SSE STREAM
  // ---------------------------------------------------------------------------
  app.get("/api/logs", (req, res) => {
    res.json(getRecentLogs());
  });

  app.delete("/api/logs", (req, res) => {
    clearLogs();
    res.json({ success: true });
  });

  app.get("/api/logs/stream", (req, res) => {
    res.setHeader("Content-Type", "text/event-stream");
    res.setHeader("Cache-Control", "no-cache");
    res.setHeader("Connection", "keep-alive");
    res.flushHeaders?.();

    // Send recent history on connect
    const recent = getRecentLogs();
    res.write(`data: ${JSON.stringify({ type: "INIT", logs: recent })}\n\n`);

    registerSseClient(res);
  });

  // Fallback route for SPA (Express 5 / path-to-regexp compatible)
  app.get("/{*splat}", (req, res) => {
    res.sendFile(path.join(publicDir, "index.html"));
  });

  const server = app.listen(port, () => {
    console.log(`\n==================================================`);
    console.log(`🌐 [PRISMBOT DASHBOARD] Live at: http://localhost:${port}`);
    console.log(`⚡ Lightweight, low-memory Termux mode active.`);
    console.log(`==================================================\n`);
  });

  return server;
}
