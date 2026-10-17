import { downloadMediaMessage } from "baileys";
import { writeFile, unlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { EDIT_COOLDOWN_MS } from "../config/constants.js";
import { getMergedPersonas } from "../commands/custom.js";

export function unwrapViewOnce(msg) {
  if (!msg) return null;
  let m = msg.message || msg;
  // Recursively unwrap nested envelopes (ephemeral, view-once, documents with caption, edited)
  while (
    m?.ephemeralMessage?.message ||
    m?.viewOnceMessage?.message ||
    m?.viewOnceMessageV2?.message ||
    m?.viewOnceMessageV2Extension?.message ||
    m?.documentWithCaptionMessage?.message ||
    m?.editedMessage?.message?.protocolMessage?.editedMessage
  ) {
    m =
      m.ephemeralMessage?.message ||
      m.viewOnceMessage?.message ||
      m.viewOnceMessageV2?.message ||
      m.viewOnceMessageV2Extension?.message ||
      m.documentWithCaptionMessage?.message ||
      m.editedMessage?.message?.protocolMessage?.editedMessage;
  }
  return m;
}

export function matchPersona(text) {
  const lower = text.toLowerCase();
  const personas = getMergedPersonas();

  for (const [key, persona] of Object.entries(personas)) {
    if (lower.startsWith(persona.prefix.toLowerCase())) {
      return { key, persona, rest: text.slice(persona.prefix.length).trim() };
    }
  }
  return null;
}

export function mentionPrefix(isGroup, senderId) {
  return isGroup ? `@${senderId.split("@")[0]} ` : "";
}

export function getIncomingText(msg) {
  const m = unwrapViewOnce(msg);
  if (!m) return "";
  return (
    m.conversation ||
    m.extendedTextMessage?.text ||
    m.imageMessage?.caption ||
    m.videoMessage?.caption ||
    m.documentMessage?.caption ||
    m.buttonsResponseMessage?.selectedButtonId ||
    m.buttonsResponseMessage?.selectedDisplayText ||
    m.templateButtonReplyMessage?.selectedId ||
    m.listResponseMessage?.singleSelectReply?.selectedRowId ||
    ""
  );
}

export function getImageMessage(msg) {
  const m = unwrapViewOnce(msg);
  if (!m) return null;
  if (m.imageMessage) return { imageMessage: m.imageMessage };
  const quoted = m.extendedTextMessage?.contextInfo?.quotedMessage;
  const unwrappedQuoted = unwrapViewOnce(quoted);
  if (unwrappedQuoted?.imageMessage) return { imageMessage: unwrappedQuoted.imageMessage };
  return null;
}

export async function resendExtractedMessage(sock, jid, targetMsg, quoted, prefixTag = "") {
  try {
    const unwrapped = unwrapViewOnce(targetMsg);
    if (!unwrapped) return false;

    const caption = (getIncomingText({ message: unwrapped }) || "").trim();
    const fullCaption = prefixTag ? `${prefixTag}\n${caption}`.trim() : caption;
    const downloadableObj = { key: targetMsg.key || {}, message: unwrapped };

    if (unwrapped.imageMessage) {
      const buffer = await downloadMediaMessage(downloadableObj, "buffer", {});
      await sock.sendMessage(jid, { image: buffer, caption: fullCaption }, { quoted });
      return true;
    } else if (unwrapped.videoMessage) {
      const buffer = await downloadMediaMessage(downloadableObj, "buffer", {});
      await sock.sendMessage(jid, { video: buffer, caption: fullCaption }, { quoted });
      return true;
    } else if (unwrapped.audioMessage) {
      const buffer = await downloadMediaMessage(downloadableObj, "buffer", {});
      await sock.sendMessage(jid, { audio: buffer, ptt: unwrapped.audioMessage.ptt, mimetype: unwrapped.audioMessage.mimetype }, { quoted });
      return true;
    } else if (caption) {
      await sock.sendMessage(jid, { text: fullCaption }, { quoted });
      return true;
    }
    return false;
  } catch (err) {
    console.error("Error in resendExtractedMessage:", err);
    return false;
  }
}

export async function sendOrEditGameMessage(sock, jid, game, contentOptions, quoted) {
  const now = Date.now();
  const canEdit = game.lastMessageKey && (now - game.lastUpdatedTime >= EDIT_COOLDOWN_MS);

  if (canEdit) {
    try {
      await sock.sendMessage(jid, { text: contentOptions.text, edit: game.lastMessageKey });
      game.lastUpdatedTime = now;
      return;
    } catch (e) {
      console.warn("Message edit failed, sending fresh message instead:", e);
    }
  }

  const sent = await sock.sendMessage(jid, contentOptions, { quoted });
  game.lastMessageKey = sent?.key;
  game.lastUpdatedTime = now;
}

export async function sendRenderedMedia(sock, jid, media, { caption, mentions, quoted }) {
  const tmpPath = path.join(tmpdir(), `${randomUUID()}.${media.ext}`);
  await writeFile(tmpPath, media.buffer);
  try {
    await sock.sendMessage(jid, { image: { url: tmpPath }, caption, mentions }, { quoted });
  } finally {
    await unlink(tmpPath).catch(() => {});
  }
}