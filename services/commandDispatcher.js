import { parseUniversalCommand } from "./geminiRouter.js";

// Active Command Handlers
import { handlePersonaCommand } from "../commands/persona.js";
import { handleStickerCommand } from "../commands/sticker.js";
import { handleGenerateImageCommand } from "../commands/generateImage.js";
import { handlePlayCommand } from "../commands/music.js";
import { handleImageSearchCommand } from "../commands/imagesearch.js";
import { handleNotesCommand } from "../commands/notes.js";
import { handleAutoReplyCommand } from "../handlers/autoreplyHandler.js";
import { isAutoReplyEnabled } from "./autoreplyStore.js";
import { handleSchedulerCommands } from "../commands/scheduler.js";
import { handleViewOnceCommands } from "../commands/viewonce.js";
import { handleStatsCommands } from "../commands/stats.js";
import { handleWelcomeCommand } from "../commands/welcome.js";
import { handleStorageCommand } from "../commands/storage.js";
import { handleMuteCommand } from "../commands/mute.js";
import { handleQrCommand } from "../commands/qrcode.js";
import { handleTtsCommand } from "./tts.js";
import { handleHelpCommand } from "../commands/help.js";
import { handleSystemCommands } from "../commands/system.js";

export async function processNaturalLanguageCommand(sock, jid, senderId, rawText, msg, currentPath) {
  // Parse intent via Gemini Router
  const intent = await parseUniversalCommand(rawText, currentPath, isAutoReplyEnabled());

  // 🔍 LOG GEMINI ROUTER OUTPUT
  console.log("\n================ [ GEMINI ROUTER OUTPUT ] ================");
  console.log("📥 Raw User Input     :", rawText);
  console.log("🎯 Extracted Action   :", intent?.action);
  console.log("🎙️ Detected Voice Key :", intent?.voice_key || "None");
  console.log("📝 Generated Text     :", intent?.text || intent?.prompt || intent?.replyText || "None");
  console.log("==========================================================\n");

  // Ignore background human chatter
  if (!intent || intent.action === "none") return false;

  switch (intent.action) {
    // --- 1. AI PERSONAS ---
    case "persona": {
      if (!intent.character) return false;
      let charKey = intent.character.toLowerCase().replace(/[^a-z0-9]/g, "");
      if (charKey === "zerotwo") charKey = "zero2";
      const personaCmd = `!${charKey} ${intent.prompt || "Hello!"}`.trim();
      const isGroup = jid.endsWith("@g.us");
      console.log(`🚀 [EXECUTING PERSONA COMMAND]: "${personaCmd}"`);
      return await handlePersonaCommand(sock, jid, senderId, isGroup, personaCmd, msg);
    }

    // --- 2. STICKERS & IMAGE GEN ---
    case "sticker":
      console.log(`🚀 [EXECUTING COMMAND]: "!s"`);
      return await handleStickerCommand(sock, jid, "!s", msg);

    case "toimg":
      console.log(`🚀 [EXECUTING COMMAND]: "!toimg"`);
      return await handleStickerCommand(sock, jid, "!toimg", msg);

    case "tovideo":
      console.log(`🚀 [EXECUTING COMMAND]: "!tovideo"`);
      return await handleStickerCommand(sock, jid, "!tovideo", msg);

    case "gen_image": {
      if (!intent.prompt) return false;
      const cmd = `!gen ${intent.prompt}`;
      console.log(`🚀 [EXECUTING COMMAND]: "${cmd}"`);
      return await handleGenerateImageCommand(sock, jid, cmd, msg);
    }

    // --- 3. DOWNLOADER & MEDIA ---
    case "download_audio": {
      const target = intent.query || intent.url;
      if (!target) return false;
      const cmd = `!play ${target}`;
      console.log(`🚀 [EXECUTING COMMAND]: "${cmd}"`);
      return await handlePlayCommand(sock, jid, cmd, msg);
    }

    case "download_video": {
      const target = intent.query || intent.url;
      if (!target) return false;
      const cmd = `!video ${target}`;
      console.log(`🚀 [EXECUTING COMMAND]: "${cmd}"`);
      return await handlePlayCommand(sock, jid, cmd, msg);
    }

    case "search_yt": {
      if (!intent.query) return false;
      const cmd = `!yt ${intent.query}`;
      console.log(`🚀 [EXECUTING COMMAND]: "${cmd}"`);
      return await handlePlayCommand(sock, jid, cmd, msg);
    }

    case "search_img": {
      if (!intent.query) return false;
      const cmd = `!img ${intent.query}`;
      console.log(`🚀 [EXECUTING COMMAND]: "${cmd}"`);
      return await handleImageSearchCommand(sock, jid, cmd, msg);
    }

    case "download_social": {
      if (!intent.url) return false;
      const cmd = `!dl ${intent.url}`;
      console.log(`🚀 [EXECUTING COMMAND]: "${cmd}"`);
      return await handlePlayCommand(sock, jid, cmd, msg);
    }

    // --- 4. NOTES & REMINDERS ---
    case "notes":
      return await handleNotesCommand(sock, jid, "!notes", msg);

    case "cd":
      return await handleNotesCommand(sock, jid, `!cd ${intent.target || ".."}`, msg);

    case "addnote": {
      const noteCmd = `!addnote ${intent.path || "public/note"} ${intent.content || ""}`.trim();
      return await handleNotesCommand(sock, jid, noteCmd, msg);
    }

    case "getnote":
      return await handleNotesCommand(sock, jid, `!getnote ${intent.target || 1}`, msg);

    case "remind": {
      const timeStr = intent.time || "10m";
      const body = intent.text || intent.content || "Reminder";
      return await handleSchedulerCommands(sock, jid, senderId, `!remind ${timeStr} ${body}`, msg);
    }

    // --- 5. VIEW-ONCE & DELETED RECOVERY ---
    case "vv_get": {
      const subTarget = intent.target || "list";
      return await handleViewOnceCommands(sock, jid, `!vv ${subTarget}`, msg);
    }

    case "deleted_get": {
      const subTarget = intent.target || "list";
      return await handleViewOnceCommands(sock, jid, `!deleted ${subTarget}`, msg);
    }

    // --- 6. CHAT STATS & RANKINGS ---
    case "stats":
      return await handleStatsCommands(sock, jid, senderId, "!stats", msg);

    case "mystats":
      return await handleStatsCommands(sock, jid, senderId, "!mystats", msg);

    // --- 7. AUTO-REPLY (ALL MODES & MANAGEMENT) ---
    case "autoreply_on": {
      const mode = intent.mode ? ` ${intent.mode.toLowerCase()}` : "";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply on${mode}`.trim(), msg);
    }

    case "autoreply_off":
      return await handleAutoReplyCommand(sock, jid, senderId, "!autoreply off", msg);

    case "autoreply_mode": {
      const targetMode = intent.mode || intent.target || "global";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply mode ${targetMode}`, msg);
    }

    case "autoreply_set_message": {
      const customMsg = intent.text || intent.content || "";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply set ${customMsg}`.trim(), msg);
    }

    case "autoreply_add_target": {
      const targetUser = intent.target || intent.query || "";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply addtarget ${targetUser}`.trim(), msg);
    }

    case "autoreply_del_target": {
      const targetUser = intent.target || intent.query || "";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply deltarget ${targetUser}`.trim(), msg);
    }

    case "autoreply_add_trigger": {
      const keyword = intent.keyword || intent.query || "";
      const reply = intent.text || intent.replyText || "";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply addtrigger ${keyword} | ${reply}`.trim(), msg);
    }

    case "autoreply_del_trigger": {
      const keyword = intent.keyword || intent.query || "";
      return await handleAutoReplyCommand(sock, jid, senderId, `!autoreply deltrigger ${keyword}`.trim(), msg);
    }

    case "autoreply_status":
      return await handleAutoReplyCommand(sock, jid, senderId, "!autoreply", msg);

    // --- 8. UTILITIES & ADMIN TOOLS ---
    case "welcome": {
      const param = intent.text || intent.target || "";
      return await handleWelcomeCommand(sock, jid, senderId, `!welcome ${param}`.trim(), msg);
    }

    case "storage": {
      const param = intent.target || "";
      return await handleStorageCommand(sock, jid, senderId, `!storage ${param}`.trim(), msg);
    }

    case "mute": {
      const user = intent.target || "";
      return await handleMuteCommand(sock, jid, senderId, `!mute ${user}`.trim(), msg);
    }

    case "unmute": {
      const user = intent.target || "";
      return await handleMuteCommand(sock, jid, senderId, `!unmute ${user}`.trim(), msg);
    }

    case "qrcode": {
      const textToEncode = intent.text || intent.content || intent.query;
      if (!textToEncode) return false;
      return await handleQrCommand(sock, jid, `!qr ${textToEncode}`, msg);
    }

    case "tts": {
      let rawSpeechText = intent.text || intent.replyText || intent.content || intent.query || intent.prompt || "";

      if (!rawSpeechText) {
        await sock.sendMessage(
          jid, 
          { text: "⚠️ Please specify what text or topic you want me to speak!" }, 
          { quoted: msg }
        );
        return true;
      }

      // 1. Sanitize voice key
      let cleanVoiceKey = intent.voice_key 
        ? intent.voice_key.toLowerCase().replace(/[^a-z0-9_]/g, "") 
        : "";

      // 2. Clean speech text
      let cleanSpeechText = rawSpeechText
        .replace(/["']/g, "")
        .replace(/[*#]/g, "")
        .replace(/([a-zA-Z0-9]+)[,.!?]/g, "$1")
        .replace(/\n+/g, " ")
        .trim();

      // 3. Manufacture command string
      let ttsCmd = "!tts";
      if (cleanVoiceKey) {
        ttsCmd += ` ${cleanVoiceKey}`;
      }
      ttsCmd += ` ${cleanSpeechText}`;

      console.log(`🚀 [EXECUTING TTS COMMAND]: "${ttsCmd}"\n`);
      return await handleTtsCommand(sock, jid, ttsCmd, msg);
    }

    case "help":
      await handleHelpCommand(sock, jid, msg, intent.query ? `!help ${intent.query}` : "!help");
      return true;


    case "system": {
      const sysCmd = intent.target || "botstatus";
      return await handleSystemCommands(sock, jid, `!${sysCmd}`, msg);
    }

    // --- 9. GENERAL CHAT ---
    case "chat":
      if (intent.replyText) {
        await sock.sendMessage(jid, { text: intent.replyText }, { quoted: msg });
        return true;
      }
      break;

    default:
      return false;
  }

  return false;
}