import "dotenv/config";
import makeWASocket, { useMultiFileAuthState, DisconnectReason } from "baileys";
import { Boom } from "@hapi/boom";
import qrcode from "qrcode-terminal";
import pino from "pino";

// Command & Feature Handlers
import { handleTtsCommand, getVoiceListText } from "./services/tts.js";
import { handlePlayCommand } from "./commands/music.js";
import { handleHelpCommand } from "./commands/help.js";
import { handleSystemCommands } from "./commands/system.js";
import {
  handleViewOnceCommands,
  recordDeletedMessage,
} from "./commands/viewonce.js";
import { handlePersonaCommand } from "./commands/persona.js";
import {
  handleSchedulerCommands,
  restoreScheduledReminders,
} from "./commands/scheduler.js";
import { handleImageSearchCommand } from "./commands/imagesearch.js";
import { handleNotesCommand } from "./commands/notes.js";
import { checkAndHandleMutedUser, handleMuteCommand } from "./commands/mute.js";
import { handleQrCommand } from "./commands/qrcode.js";
import { trackMessageStats, handleStatsCommands } from "./commands/stats.js";
import { handleTranscribeCommand } from "./commands/transcribe.js";
import { handleGenerateImageCommand } from "./commands/generateImage.js";
import { handleStickerCommand } from "./commands/sticker.js";
import {
  handleWelcomeCommand,
  triggerGroupWelcome,
} from "./commands/welcome.js";
import { handleAddCustomCommand } from "./commands/custom.js";
import { handleStorageCommand } from "./commands/storage.js";
import { handleVoiceCommand } from "./commands/voice.js"; // Voice Conversion Handler
import { handleToolsCommand } from "./commands/tools.js"; // Superpower Utilities (OCR, Translator, Polls, Calc, Weather, Memes)

// Auto Reply Imports
import { loadAutoReplyStore } from "./services/autoreplyStore.js";
import { handleAutoReplyCommand, processAutoReplyListener } from "./handlers/autoreplyHandler.js";

// Universal Gemini AI Natural Language Dispatcher
import { processNaturalLanguageCommand } from "./services/commandDispatcher.js";

// Anti-Spam Middleware
import { checkSpam } from "./utils/antiSpam.js";

// Dashboard Server & Logger
import { startDashboardServer, setBotContext } from "./dashboard/server.js";
import { logEvent } from "./dashboard/logger.js";

// Helpers & Store
import { getIncomingText } from "./utils/helpers.js";
import {
  wakeUpBot,
  enterSleepMode,
  getIsSleeping,
  resetInactivityTimer,
} from "./utils/sleepManager.js";
import {
  messageStore,
  activeAntiDeleteChats,
  activeFeatureChats,
  contactStore,
  setContact,
  getDisplayName,
  storeMessage,
  startCacheCleanupInterval,
} from "./store/messageStore.js";

/**
 * Command registry for the "optional features" gated block.
 */
const FEATURE_COMMANDS = [
  {
    match: (t) => t.startsWith("!help") || t.startsWith("!menu"),
    handler: async ({ sock, jid, msg, rawText }) => {
      await handleHelpCommand(sock, jid, msg, rawText);
      return true;
    },
  },
  {
    match: (t) => t.startsWith("!tts"),
    handler: async ({ sock, jid, rawText, msg }) => {
      await handleTtsCommand(sock, jid, rawText, msg);
      return true;
    },
  },
  {
    match: (t) => t === "!voices",
    handler: async ({ sock, jid, msg }) => {
      await sock.sendMessage(
        jid,
        { text: getVoiceListText() },
        { quoted: msg },
      );
      return true;
    },
  },


  {
    match: (t) => t.startsWith("!img") || t.startsWith("!image"),
    handler: async ({ sock, jid, rawText, msg }) => {
      await handleImageSearchCommand(sock, jid, rawText, msg);
      return true;
    },
  },
  {
    match: (t) =>
      t.startsWith("!play") ||
      t.startsWith("!song") ||
      t.startsWith("!music") ||
      t.startsWith("!mp3") ||
      t.startsWith("!yt") ||
      t.startsWith("!ig") ||
      t.startsWith("!reddit") ||
      t.startsWith("!video") ||
      t.startsWith("!mp4") ||
      t.startsWith("!reel") ||
      t.startsWith("!tiktok") ||
      t.startsWith("!dl"),
    handler: async ({ sock, jid, rawText, msg }) => {
      await handlePlayCommand(sock, jid, rawText, msg);
      return true;
    },
  },
];

async function startBot() {
  const DASHBOARD_PORT = process.env.PORT || process.env.DASHBOARD_PORT || 3000;
  
  // Start high-performance, lightweight web dashboard
  try {
    startDashboardServer(DASHBOARD_PORT);
  } catch (err) {
    console.error("Dashboard Server Warning:", err.message);
  }

  startCacheCleanupInterval(getIsSleeping);

  // Initialize persistent stores
  await loadAutoReplyStore();

  const { state, saveCreds } = await useMultiFileAuthState("auth_info");
  const sock = makeWASocket({
    auth: state,
    logger: pino({ level: "silent" }),
    printQRInTerminal: false,
    keepAliveIntervalMs: 30000,
    markOnlineOnConnect: false,
  });

  // Link global bot context for dashboard control
  setBotContext({
    sock,
    getIsSleeping,
    wakeUpBot,
    enterSleepMode,
    connectionState: "connecting",
    startTime: Date.now(),
  });

  sock.ev.on("connection.update", (update) => {
    const { connection, lastDisconnect, qr } = update;
    if (qr) {
      qrcode.generate(qr, { small: true });
      setBotContext({ qrCodeData: qr, connectionState: "qr_pending" });
    }

    if (connection === "close") {
      setBotContext({ connectionState: "disconnected" });
      logEvent("SYSTEM", "WhatsApp connection closed. Reconnecting...");
      const statusCode = new Boom(lastDisconnect?.error)?.output?.statusCode;
      if (statusCode !== DisconnectReason.loggedOut) startBot();
    } else if (connection === "open") {
      console.log("Connected to WhatsApp.");
      setBotContext({ connectionState: "open", qrCodeData: null, sock });
      logEvent("SUCCESS", "Connected to WhatsApp successfully");
      resetInactivityTimer();
      restoreScheduledReminders(sock);

      // Pre-fetch active group names in background
      for (const jid of activeFeatureChats) {
        if (jid.endsWith("@g.us") && !contactStore.has(jid)) {
          sock.groupMetadata(jid).then((meta) => {
            if (meta?.subject) setContact(jid, meta.subject);
          }).catch(() => {});
        }
      }
    }
  });

  sock.ev.on("creds.update", saveCreds);

  // Sync WhatsApp contact names
  sock.ev.on("contacts.upsert", (contacts) => {
    for (const c of contacts) {
      const name = c.name || c.notify || c.verifiedName;
      if (c.id && name) setContact(c.id, name);
    }
  });

  sock.ev.on("contacts.update", (updates) => {
    for (const u of updates) {
      const name = u.name || u.notify || u.verifiedName;
      if (u.id && name) setContact(u.id, name);
    }
  });

  sock.ev.on("chats.upsert", (chats) => {
    for (const c of chats) {
      if (c.id && c.name) setContact(c.id, c.name);
    }
  });

  sock.ev.on("chats.update", (updates) => {
    for (const u of updates) {
      if (u.id && u.name) setContact(u.id, u.name);
    }
  });

  sock.ev.on("groups.update", (updates) => {
    for (const u of updates) {
      if (u.id && u.subject) setContact(u.id, u.subject);
    }
  });

  // Group Member Join Event (Welcomer Handler)
  sock.ev.on("group-participants.update", async (update) => {
    if (update.action === "add") {
      await triggerGroupWelcome(sock, update.id, update.participants);
    }
  });

  // Auto Anti-Delete Event Handler
  sock.ev.on("messages.update", async (updates) => {
    if (getIsSleeping()) return;
    for (const update of updates) {
      if (update.update?.message === null) {
        const stored = messageStore.get(update.key.id);
        const jid = stored?.msg?.key?.remoteJid || update.key.remoteJid;

        if (stored && stored.msg && jid) {
          recordDeletedMessage(stored.msg);
          logEvent("SYSTEM", `Anti-Delete intercepted deleted message from ${jid}`);
        }
      }
    }
  });

  // Main Incoming Message Handler
  sock.ev.on("messages.upsert", async ({ messages, type }) => {
    if (type !== "notify") return;

    for (const msg of messages) {
      try {
        if (!msg.message) continue;

        const rawText = getIncomingText(msg).trim();
        const lowerText = rawText.toLowerCase();
        const jid = msg.key.remoteJid;
        if (jid === "status@broadcast") continue;

        storeMessage(msg);

        const protocolMsg = msg.message?.protocolMessage;
        if (
          protocolMsg &&
          (protocolMsg.type === 0 || protocolMsg.type === "REVOKE")
        ) {
          const deletedId = protocolMsg.key?.id;
          if (deletedId && messageStore.has(deletedId)) {
            const stored = messageStore.get(deletedId);
            recordDeletedMessage(stored.msg);
          }
          continue;
        }

        const isGroup = jid.endsWith("@g.us");
        const senderId = msg.key.participant || jid;

        trackMessageStats(jid, senderId);

        if (msg.pushName) {
          setContact(senderId, msg.pushName);
          if (!isGroup) setContact(jid, msg.pushName);
        }

        // 0. Auto-Delete Muted Users Middleware
        const isMuted = await checkAndHandleMutedUser(sock, jid, senderId, msg);
        if (isMuted) continue;

        // Auto-reply listener check for incoming trigger messages
        await processAutoReplyListener(sock, msg);

        // 1. System Commands
        if (await handleSystemCommands(sock, jid, rawText, msg)) continue;

        // De-bloat: Silent Wake-up Guard without extra chat spam
        if (getIsSleeping()) {
          if (rawText.startsWith("!")) {
            wakeUpBot();
          } else {
            continue;
          }
        } else {
          resetInactivityTimer(() => enterSleepMode());
        }

        // De-bloat: Anti-Spam Check with minimal chat noise
        if (rawText.startsWith("!") && !msg.key.fromMe) {
          const spamCheck = checkSpam(senderId);

          if (spamCheck.isSpam) {
            if (spamCheck.reason === "JUST_MUTED") {
              await sock.sendMessage(
                jid,
                {
                  text: `🚫 *@${senderId.split("@")[0]} You have been temporarily muted for 1m due to command spamming.*`,
                  mentions: [senderId],
                },
                { quoted: msg },
              );
            }
            // If COOLDOWN, silently drop duplicate spam without sending extra text into chat
            continue;
          }
        }

        // Web Dashboard Command
        if (lowerText === "!dashboard" || lowerText === "!panel" || lowerText === "!web") {
          await sock.sendMessage(
            jid,
            {
              text: `🌐 *PrismBot Web Dashboard:*\nhttp://localhost:${DASHBOARD_PORT}\n\n_Manage bot settings, auto-reply, personas, and live metrics._`,
            },
            { quoted: msg }
          );
          continue;
        }

        // Global Help & Command Menu (Always available in all chats)
        if (lowerText.startsWith("!help") || lowerText.startsWith("!menu")) {
          await handleHelpCommand(sock, jid, msg, rawText);
          continue;
        }

        if (await handleAutoReplyCommand(sock, jid, senderId, rawText, msg))
          continue;
        if (await handleStorageCommand(sock, jid, senderId, rawText, msg)) continue;
        if (await handleAddCustomCommand(sock, jid, senderId, rawText, msg))
          continue;

        // Voice Conversion Queue Commands (!vc, !voice, !cover)
        if (await handleVoiceCommand(sock, jid, senderId, rawText, msg))
          continue;

        // 2. View Once Commands
        if (await handleViewOnceCommands(sock, jid, rawText, msg)) continue;

        // 3. Admin Mute Commands
        if (await handleMuteCommand(sock, jid, senderId, rawText, msg))
          continue;

        // 4. QR Code Generator
        if (await handleQrCommand(sock, jid, rawText, msg)) continue;

        // 5. Chat Activity Stats
        if (await handleStatsCommands(sock, jid, senderId, rawText, msg))
          continue;

        // 6. Media Utilities (Sticker, ToImg, RemBG)
        if (await handleStickerCommand(sock, jid, rawText, msg)) continue;

        // 7. Group Welcome Manager Command
        if (await handleWelcomeCommand(sock, jid, senderId, rawText, msg))
          continue;

        // 8. Image Generation
        if (await handleGenerateImageCommand(sock, jid, rawText, msg)) continue;

        // 8a. Superpower Utilities (OCR, Summarizer, Translator, Polls, Calc, Weather, Memes, TagAll)
        if (await handleToolsCommand(sock, jid, senderId, rawText, msg)) continue;

        // 9. Optional Features Guard (DMs always enabled; Groups check activeFeatureChats)
        const isFeaturesActive = !isGroup || activeFeatureChats.has(jid);
        if (!isFeaturesActive) {
          if (rawText.startsWith("!")) {
            // Silently drop persona spam in disabled group chats
          }
          continue;
        }

        // Multimodal Media Transcriber
        if (await handleTranscribeCommand(sock, jid, rawText, msg)) continue;

        // Reminders & Scheduler
        if (await handleSchedulerCommands(sock, jid, senderId, rawText, msg))
          continue;

        // Notes System
        if (await handleNotesCommand(sock, jid, rawText, msg)) continue;

        // Registry commands
        const ctx = { sock, jid, senderId, isGroup, rawText, msg };
        const matchedCommand = FEATURE_COMMANDS.find((c) => c.match(lowerText));
        if (matchedCommand) {
          const handled = await matchedCommand.handler(ctx);
          if (handled) continue;
        }

        // 10. AI Persona Handler
        if (
          await handlePersonaCommand(sock, jid, senderId, isGroup, rawText, msg)
        )
          continue;

        // 11. Prefix Command Fallback (De-bloat: only hint in private DMs; silently ignore unknown prefixes in groups)
        if (rawText.startsWith("!")) {
          if (!isGroup) {
            const unknownCmd = rawText.split(/\s+/)[0];
            await sock.sendMessage(
              jid,
              {
                text: `⚠️ Unknown command \`${unknownCmd}\`. Type \`!help\` or visit dashboard (\`!dashboard\`).`,
              },
              { quoted: msg },
            );
          }
          continue;
        }

        // 12. Non-Prefixed Plain English Fallback -> Gemini Universal AI Router
        if (rawText.startsWith("@prism") ) {
          const currentPath = "root/";
          await processNaturalLanguageCommand(sock, jid, senderId, rawText, msg, currentPath);
        }

      } catch (err) {
        console.error("Error handling message:", err);
      }
    }
  });
}

startBot();