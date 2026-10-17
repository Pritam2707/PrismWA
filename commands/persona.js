import { downloadMediaMessage } from "baileys";
import { solveWithGemini } from "../services/gemini.js";
import { containsMath, renderAnswerMedia, sanitizeMathNotation } from "../services/renderer.js";
import getLocalStickerBuffer from "../utils/sticker.js";
import { matchPersona, mentionPrefix, getImageMessage, sendRenderedMedia } from "../utils/helpers.js";

// Imported Feature Handlers for Function Calls
import { handlePlayCommand } from "./music.js";
import { handleTtsCommand } from "../services/tts.js";
import { handleImageSearchCommand } from "./imagesearch.js";
import { sendTypingStatus, sendReaction, clearReaction } from "../services/presence.js";


export async function handlePersonaCommand(sock, jid, senderId, isGroup, rawText, msg) {
  const matched = matchPersona(rawText);
  if (!matched || (msg.key.fromMe && !matched)) return false;

  const { key: personaKey, persona, rest: question } = matched;
  const historyKey = `${jid}::${senderId}::${personaKey}`;
  const imageRef = getImageMessage(msg);

  let imageBuffer = null;
  let mimeType = null;

  if (imageRef) {
    imageBuffer = await downloadMediaMessage({ message: imageRef }, "buffer", {});
    mimeType = imageRef.imageMessage.mimetype;
  }

  const mention = mentionPrefix(isGroup, senderId);
  const mentions = isGroup ? [senderId] : undefined;

  // De-bloat: Use native typing indicator and reaction instead of spamming text into chat
  sendTypingStatus(sock, jid);
  if (imageBuffer) {
    sendReaction(sock, jid, msg.key, "💭");
  }

  const result = await solveWithGemini({ historyKey, persona, text: question, imageBuffer, mimeType });
  if (imageBuffer) {
    clearReaction(sock, jid, msg.key);
  }


  // ---------------------------------------------------------------------------
  // 1. Handle Gemini Function Calls
  // ---------------------------------------------------------------------------
  if (result.type === "function_call") {
    const { name, args } = result;

    if (name === "play_music" && args?.query) {
      await sock.sendMessage(jid, { text: `${mention}🎵 *Searching and playing:* "${args.query}"...`, mentions }, { quoted: msg });
      await handlePlayCommand(sock, jid, `!play ${args.query}`, msg);
      return true;
    }

    if (name === "text_to_speech" && args?.text) {
      await handleTtsCommand(sock, jid, `!tts ${args.voice} ${args.text}`, msg);
      return true;
    }

    if (name === "search_image" && args?.query) {
      await handleImageSearchCommand(sock, jid, `!img ${args.query}`, msg);
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 2. Handle Text Responses (Math/LaTeX Rendering & Plain Text)
  // ---------------------------------------------------------------------------
  const answer = result.answer || "";
  const mood = result.mood;

  if (containsMath(answer)) {
    try {
      const media = await renderAnswerMedia(answer);
      const caption = `${mention}${persona.label}`.trim();
      await sendRenderedMedia(sock, jid, media, { caption, mentions, quoted: msg, fileName: `${personaKey}-answer.${media.ext}` });
    } catch (renderErr) {
      console.error("Render failed, falling back to plain text:", renderErr);
      await sock.sendMessage(jid, { text: `${mention}${sanitizeMathNotation(answer)}`, mentions }, { quoted: msg });
    }
  } else if (answer) {
    await sock.sendMessage(jid, { text: `${mention}${answer}`, mentions }, { quoted: msg });
  }

  // ---------------------------------------------------------------------------
  // 3. Handle Mood Stickers
  // ---------------------------------------------------------------------------
  if (mood && mood !== "none" && persona.stickers?.[mood]) {
    const stickerBuffer = await getLocalStickerBuffer(persona.stickers[mood]);
    if (stickerBuffer) {
      await sock.sendMessage(jid, { sticker: stickerBuffer }, { quoted: msg });
    }
  }

  return true;
}