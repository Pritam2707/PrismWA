import { transcribeMediaMessage } from "../services/transcriber.js";
import { sendReaction } from "../services/presence.js";

const TRIGGERS = new Set(["!tr", "!transcribe", "!read"]);

/**
 * Handles !tr / !transcribe / !read — transcribes or analyzes replied/attached
 * media (audio, image, video, PDF) via Gemini.
 */
export async function handleTranscribeCommand(sock, jid, rawText, msg) {
  const [rawCommand, ...rest] = rawText.trim().split(/\s+/);
  const command = rawCommand.toLowerCase();

  if (!TRIGGERS.has(command)) return false;

  const customInstruction = rest.join(" ").trim();

  const quotedMessage =
    msg.message?.extendedTextMessage?.contextInfo?.quotedMessage;

  const targetMsg = quotedMessage
    ? { message: quotedMessage }
    : msg.message?.audioMessage ||
      msg.message?.imageMessage ||
      msg.message?.videoMessage ||
      msg.message?.documentMessage
    ? msg
    : null;

  if (!targetMsg) {
    await sock.sendMessage(
      jid,
      {
        text:
          "🎙️📄 *Usage:* Reply to a Voice Note, Image, Video, or PDF Document with `!tr` to analyze/transcribe it.\n" +
          "_Add an instruction to customize it, e.g._ `!tr summarize in 3 points`",
      },
      { quoted: msg }
    );
    return true;
  }

  // De-bloat: Use emoji reaction on user's message
  await sendReaction(sock, jid, msg.key, "📝");

  try {
    const resultText = await transcribeMediaMessage(
      targetMsg,
      customInstruction || null
    );

    if (resultText) {
      await sock.sendMessage(
        jid,
        { text: `📝 *Media Analysis / Transcript:*\n\n${resultText}` },
        { quoted: msg }
      );
      await sendReaction(sock, jid, msg.key, "✅");
    } else {
      await sendReaction(sock, jid, msg.key, "❌");
      await sock.sendMessage(
        jid,
        { text: "❌ Could not process content from this media file." },
        { quoted: msg }
      );
    }
  } catch (err) {
    console.error("Transcription Command Error:", err);
    await sendReaction(sock, jid, msg.key, "❌");
    await sock.sendMessage(
      jid,
      { text: "❌ Failed to process the media file." },
      { quoted: msg }
    );
  }

  return true;
}