import fs from "node:fs/promises";
import path from "node:path";
import { downloadMediaMessage } from "baileys";
import ffmpeg from "fluent-ffmpeg";
import { tmpdir } from "node:os";
import { PERSONAS as BASE_PERSONAS, VOICE_IDS as BASE_VOICE_IDS } from "../config/constants.js";

const DYNAMIC_STORE_PATH = path.join(process.cwd(), "data", "dynamic_store.json");
const STICKERS_DIR = path.join(process.cwd(), "stickers");

let dynamicStore = { personas: {}, voices: {}, stickerOverrides: {} };

// Load custom JSON data on boot
async function loadDynamicStore() {
  try {
    const raw = await fs.readFile(DYNAMIC_STORE_PATH, "utf-8");
    dynamicStore = JSON.parse(raw);
    if (!dynamicStore.stickerOverrides) dynamicStore.stickerOverrides = {};
  } catch (err) {
    dynamicStore = { personas: {}, voices: {}, stickerOverrides: {} };
    await saveDynamicStore();
  }
}

async function saveDynamicStore() {
  try {
    await fs.mkdir(path.dirname(DYNAMIC_STORE_PATH), { recursive: true });
    await fs.writeFile(DYNAMIC_STORE_PATH, JSON.stringify(dynamicStore, null, 2));
  } catch (err) {
    console.error("Failed to save dynamic store:", err);
  }
}

loadDynamicStore();

/**
 * Returns merged Personas (Base + Dynamic additions + Sticker overrides)
 *
 * FIX #2: The old version spread BASE_PERSONAS shallowly, so `merged[key]`
 * for any base persona was the SAME object reference as BASE_PERSONAS[key].
 * Writing `merged[personaKey].stickers = ...` therefore permanently mutated
 * the shared BASE_PERSONAS constant (and would throw if it were ever frozen).
 * Now every persona entry (base + dynamic) is cloned before being touched.
 */
export function getMergedPersonas() {
  const merged = {};

  for (const [key, val] of Object.entries({ ...BASE_PERSONAS, ...dynamicStore.personas })) {
    merged[key] = { ...val, stickers: { ...(val.stickers || {}) } };
  }

  // Apply dynamic sticker additions/overrides to any persona
  for (const [personaKey, customStickers] of Object.entries(dynamicStore.stickerOverrides || {})) {
    if (merged[personaKey]) {
      merged[personaKey].stickers = {
        ...merged[personaKey].stickers,
        ...customStickers,
      };
    }
  }

  return merged;
}

/**
 * Returns merged Voice IDs (Base + Dynamic additions)
 */
export function getMergedVoiceIds() {
  return { ...BASE_VOICE_IDS, ...dynamicStore.voices };
}

/**
 * Converts any media buffer (GIF/Video/Image/Sticker) into a 512x512 WebP sticker
 */
async function processAndSaveSticker(buffer, fileName, isVideoOrGif = false) {
  await fs.mkdir(STICKERS_DIR, { recursive: true });
  const tempInput = path.join(tmpdir(), `temp_input_${Date.now()}`);
  const finalPath = path.join(STICKERS_DIR, fileName);

  await fs.writeFile(tempInput, buffer);

  return new Promise((resolve, reject) => {
    let f = ffmpeg(tempInput);
    if (isVideoOrGif) f.inputOptions(["-ss 00:00:00", "-t 00:00:05"]); // Max 5s clip

    f.addOutputOptions([
      "-vcodec libwebp",
      "-vf scale='min(512\\,iw):min(512\\,ih)':force_original_aspect_ratio=decrease,fps=15, pad=512:512:(512-iw)/2:(512-ih)/2:color=00000000",
      "-loop 0",
      "-pix_fmt rgba",
      "-an",
      "-vsync 0",
    ])
      .toFormat("webp")
      .on("end", async () => {
        await fs.unlink(tempInput).catch(() => {});
        resolve(`./stickers/${fileName}`);
      })
      .on("error", async (err) => {
        await fs.unlink(tempInput).catch(() => {});
        reject(err);
      })
      .save(finalPath);
  });
}

/**
 * Self-DM / Admin Command Handler for dynamic management
 */
export async function handleAddCustomCommand(sock, jid, senderId, rawText, msg) {
  // FIX #1: The old check `if (!isSelfDm && !msg.key.fromMe) return false;`
  // was dead logic — isSelfDm can only be true when msg.key.fromMe is already
  // true, so the condition always simplified to just `!msg.key.fromMe`. That
  // let these admin commands run from ANY chat the bot's account sent from
  // (including groups), not only the bot's own self-DM. Now we actually
  // enforce isSelfDm.
  const isSelfDm = msg.key.fromMe && jid === sock.user?.id?.split(":")[0] + "@s.whatsapp.net";
  if (!isSelfDm) return false;

  const args = rawText.trim().split(/\s+/);
  const command = args[0]?.toLowerCase();

  if (!["!addpersona", "!addvoice", "!addsticker", "!customs"].includes(command)) {
    return false;
  }

  // Help Menu
  if (command === "!customs") {
    const menu =
      "🛠️ *Dynamic Customs Manager*\n\n" +
      "1️⃣ *Add Persona:*\n" +
      "`!addpersona <key> | <label> | <flavor_text>`\n\n" +
      "2️⃣ *Add Voice ID:*\n" +
      "`!addvoice <key> | <voice_id_hex>`\n\n" +
      "3️⃣ *Add Persona Sticker (Attach/Reply Media):*\n" +
      "`!addsticker <persona_key> <mood_tag>`\n" +
      "*Example:* `!addsticker zero2 smile` (Reply to GIF/Image/Sticker)";

    await sock.sendMessage(jid, { text: menu }, { quoted: msg });
    return true;
  }

  // 1. Add Custom Persona
  if (command === "!addpersona") {
    const payload = rawText.replace("!addpersona", "").trim();
    const parts = payload.split("|").map((p) => p.trim());

    if (parts.length < 3) {
      await sock.sendMessage(jid, { text: "⚠️ Use: `!addpersona <key> | <Label> | <Flavor text>`" }, { quoted: msg });
      return true;
    }

    const key = parts[0].toLowerCase().replace(/[^a-z0-9_]/g, "");
    dynamicStore.personas[key] = {
      prefix: `!${key}`,
      label: parts[1],
      flavor: parts.slice(2).join("|"),
      stickers: {},
    };

    await saveDynamicStore();
    await sock.sendMessage(jid, { text: `✅ *Persona \`${key}\` added!*` }, { quoted: msg });
    return true;
  }

  // 2. Add Custom Voice Model
  if (command === "!addvoice") {
    const payload = rawText.replace("!addvoice", "").trim();
    const parts = payload.split("|").map((p) => p.trim());

    if (parts.length < 2) {
      await sock.sendMessage(jid, { text: "⚠️ Use: `!addvoice <key> | <voice_id_hex>`" }, { quoted: msg });
      return true;
    }

    const key = parts[0].toLowerCase().replace(/[^a-z0-9_]/g, "");
    dynamicStore.voices[key] = parts[1];
    await saveDynamicStore();

    await sock.sendMessage(jid, { text: `🎙️ *Voice \`${key}\` added!*` }, { quoted: msg });
    return true;
  }

  // 3. Add / Update Persona Sticker
  if (command === "!addsticker") {
    const personaKey = args[1]?.toLowerCase();
    const moodTag = args[2]?.toLowerCase();

    const allPersonas = getMergedPersonas();

    if (!personaKey || !moodTag) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ Usage: Reply to an Image/GIF/Video/Sticker with:\n`!addsticker <persona_key> <mood_tag>`\n\n*Example:* `!addsticker zero2 happy`" },
        { quoted: msg }
      );
      return true;
    }

    if (!allPersonas[personaKey]) {
      await sock.sendMessage(jid, { text: `❌ Persona \`${personaKey}\` does not exist.` }, { quoted: msg });
      return true;
    }

    const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
    const quotedMsg = contextInfo?.quotedMessage;

    const imgMsg = msg.message?.imageMessage || quotedMsg?.imageMessage;
    const vidMsg = msg.message?.videoMessage || quotedMsg?.videoMessage;
    const stkMsg = msg.message?.stickerMessage || quotedMsg?.stickerMessage;

    if (!imgMsg && !vidMsg && !stkMsg) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ Please reply to or attach an Image, GIF, Video, or Sticker when running `!addsticker`." },
        { quoted: msg }
      );
      return true;
    }

    try {
      await sock.sendMessage(jid, { text: "⏳ *Processing media and converting to WebP sticker...*" }, { quoted: msg });

      // FIX #4: `vidMsg?.gifPlayback` is redundant once `!!vidMsg` is already
      // in the expression — if vidMsg is falsy, `vidMsg?.gifPlayback` is also
      // falsy, so the `||` never added any information. Simplified below.
      const isVideoOrGif = !!vidMsg;

      // FIX #3: The old code called downloadMediaMessage with a bare
      // `{ message: mediaTarget }` stub that had no `key` (remoteJid/id/
      // participant). Baileys needs the key to re-request media from the
      // socket if it has expired or needs a reupload — without it, download
      // of otherwise-valid-but-expired media silently fails. We now pass the
      // real message when the media is attached directly, or reconstruct a
      // proper message-with-key when the media came from a quoted/replied message.
      const mediaMessageContent = imgMsg
        ? { imageMessage: imgMsg }
        : vidMsg
        ? { videoMessage: vidMsg }
        : { stickerMessage: stkMsg };

      const isReplyMedia = !!quotedMsg && (quotedMsg.imageMessage || quotedMsg.videoMessage || quotedMsg.stickerMessage);

      const mediaTarget = isReplyMedia
        ? {
            key: {
              remoteJid: jid,
              id: contextInfo.stanzaId,
              participant: contextInfo.participant,
              fromMe: false,
            },
            message: mediaMessageContent,
          }
        : msg;

      const mediaBuffer = await downloadMediaMessage(mediaTarget, "buffer", {});
      const fileName = `${personaKey}-${moodTag}.webp`;

      const relativeStickerPath = await processAndSaveSticker(mediaBuffer, fileName, isVideoOrGif);

      // Save override in dynamicStore
      if (!dynamicStore.stickerOverrides[personaKey]) {
        dynamicStore.stickerOverrides[personaKey] = {};
      }
      dynamicStore.stickerOverrides[personaKey][moodTag] = relativeStickerPath;

      await saveDynamicStore();

      await sock.sendMessage(
        jid,
        {
          text: `✅ *Sticker Updated Successfully!*\n\n• *Persona:* \`${personaKey}\`\n• *Mood Tag:* \`[MOOD: ${moodTag}]\`\n• *Path:* \`${relativeStickerPath}\``,
        },
        { quoted: msg }
      );
    } catch (err) {
      console.error("Failed to add persona sticker:", err);
      await sock.sendMessage(jid, { text: `❌ Failed to process sticker: ${err.message}` }, { quoted: msg });
    }

    return true;
  }

  return true;
}