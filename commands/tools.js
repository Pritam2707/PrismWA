import { downloadMediaMessage } from "baileys";
import { GoogleGenAI } from "@google/genai";
import { createCanvas, loadImage } from "@napi-rs/canvas";
import sharp from "sharp";
import { GEMINI_API_KEY, GEMINI_MODEL } from "../config/constants.js";
import { getIncomingText, unwrapViewOnce, getImageMessage } from "../utils/helpers.js";

const ai = new GoogleGenAI({ apiKey: GEMINI_API_KEY });
const FAST_MODEL = process.env.GEMINI_FAST_MODEL || GEMINI_MODEL || "gemini-3.5-flash-lite";

/**
 * Superpower Tools & Daily Utilities Handler
 */
export async function handleToolsCommand(sock, jid, senderId, rawText, msg) {
  if (!rawText || typeof rawText !== "string") return false;

  const trimmed = rawText.trim();
  const args = trimmed.split(/\s+/);
  const command = (args[0] || "").toLowerCase();
  const isGroup = jid.endsWith("@g.us");

  // ---------------------------------------------------------------------------
  // 1. OCR & TEXT EXTRACTOR (!ocr, !read, !scan)
  // ---------------------------------------------------------------------------
  if (["!ocr", "!read", "!scan"].includes(command)) {
    const imgRef = getImageMessage(msg);
    if (!imgRef) {
      await sock.sendMessage(
        jid,
        { text: "📷 *Usage:* Reply to or send an image with `!ocr` or `!read` to extract text from handwriting, documents, or screenshots." },
        { quoted: msg }
      );
      return true;
    }

    try {
      const buffer = await downloadMediaMessage({ message: imgRef }, "buffer", {});
      const mimeType = imgRef.imageMessage?.mimetype || "image/jpeg";

      const prompt = "Extract all text present in this image cleanly. Format it neatly with appropriate paragraphs and line breaks. If there are tables or lists, preserve their structure.";

      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [
          {
            role: "user",
            parts: [
              { inlineData: { mimeType, data: buffer.toString("base64") } },
              { text: prompt },
            ],
          },
        ],
      });

      const extracted = response.text?.trim() || "No readable text detected in this image.";
      await sock.sendMessage(
        jid,
        { text: `📄 *[EXTRACTED TEXT]:*\n\n${extracted}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ OCR failed: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 2. SUMMARIZER / TL;DR (!sum, !summarize, !tldr)
  // ---------------------------------------------------------------------------
  if (["!sum", "!summarize", "!tldr"].includes(command)) {
    const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
    const quotedMsg = contextInfo?.quotedMessage;
    const quotedText = quotedMsg ? getIncomingText({ message: quotedMsg }) : "";
    const inputText = args.slice(1).join(" ").trim() || quotedText;

    if (!inputText || inputText.length < 20) {
      await sock.sendMessage(
        jid,
        { text: "📝 *Usage:* Reply to any long message or type `!summarize <long text>` to get a quick 3-bullet executive summary." },
        { quoted: msg }
      );
      return true;
    }

    try {
      const prompt = `Summarize the following text clearly:\n\n1. A one-sentence high-level summary.\n2. Key Bullet Points (maximum 4-5 points).\n3. Action item or takeaway (if any).\n\nText:\n"${inputText}"`;

      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
      });

      const summary = response.text?.trim() || "Unable to summarize this text.";
      await sock.sendMessage(
        jid,
        { text: `📋 *[EXECUTIVE SUMMARY]*\n\n${summary}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Summarization failed: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 3. TRANSLATOR (!tr, !translate)
  // ---------------------------------------------------------------------------
  if (["!tr", "!translate"].includes(command)) {
    const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
    const quotedMsg = contextInfo?.quotedMessage;
    const quotedText = quotedMsg ? getIncomingText({ message: quotedMsg }) : "";

    let targetLang = args[1] || "English";
    let textToTranslate = args.slice(2).join(" ").trim();

    if (!textToTranslate && quotedText) {
      textToTranslate = quotedText;
    } else if (!textToTranslate && !quotedText && args.length > 1) {
      targetLang = "English";
      textToTranslate = args.slice(1).join(" ").trim();
    }

    if (!textToTranslate) {
      await sock.sendMessage(
        jid,
        {
          text: `🌐 *Translator Help:*\n\n` +
            `• Reply to a message with \`!tr <language>\` (e.g. \`!tr hi\`, \`!tr es\`, \`!tr fr\`)\n` +
            `• Direct translate: \`!tr Hindi Hello, how are you today?\`\n` +
            `• Common codes: \`hi\` (Hindi), \`en\` (English), \`es\` (Spanish), \`ja\` (Japanese), \`fr\` (French), \`de\` (German), \`ar\` (Arabic), \`bn\` (Bengali)`,
        },
        { quoted: msg }
      );
      return true;
    }

    try {
      const prompt = `Translate the following text accurately and naturally into "${targetLang}". Return strictly the translated text, preserving the tone and formatting:\n\n"${textToTranslate}"`;

      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
      });

      const translated = response.text?.trim() || "Translation unavailable.";
      await sock.sendMessage(
        jid,
        { text: `🌐 *[TRANSLATION -> ${targetLang.toUpperCase()}]:*\n\n${translated}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Translation failed: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 4. NATIVE WHATSAPP POLL CREATOR (!poll)
  // ---------------------------------------------------------------------------
  if (command === "!poll") {
    // Parse quotes or commas: !poll "Question" "Option 1" "Option 2" ...
    const rawArgs = trimmed.slice(5).trim();
    const matches = rawArgs.match(/"([^"]+)"|'([^']+)'|([^,]+)/g);

    let parts = [];
    if (matches) {
      parts = matches.map((s) => s.replace(/^["']|["']$/g, "").trim()).filter(Boolean);
    }

    if (parts.length < 3) {
      await sock.sendMessage(
        jid,
        {
          text: `📊 *Interactive Poll Creator:*\n\n` +
            `*Usage:* \`!poll "Question" "Option 1" "Option 2" ...\`\n\n` +
            `*Example:* \`!poll "Weekend Plan?" "Gaming 🎮" "Movie 🎬" "Study 📚"\``,
        },
        { quoted: msg }
      );
      return true;
    }

    const question = parts[0];
    const options = parts.slice(1, 12); // WhatsApp supports up to 12 options

    try {
      await sock.sendMessage(
        jid,
        {
          poll: {
            name: `📊 ${question}`,
            values: options,
            selectableCount: 1,
          },
        },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Failed to create poll: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 5. SMART CALCULATOR & UNIT CONVERTER (!calc, !math)
  // ---------------------------------------------------------------------------
  if (["!calc", "!math"].includes(command)) {
    const expr = args.slice(1).join(" ").trim();
    if (!expr) {
      await sock.sendMessage(
        jid,
        { text: "🔢 *Usage:* `!calc <expression>`\n\n*Examples:*\n• `!calc 15% of $4500`\n• `!calc 50 USD in INR`\n• `!calc 120 km/h in mph`\n• `!calc 2^10 + sqrt(144)`" },
        { quoted: msg }
      );
      return true;
    }

    try {
      const prompt = `You are a precision calculator. Evaluate this math or conversion expression: "${expr}". Give the exact final numerical or converted answer prominently, followed by a one-line step breakdown if non-trivial. Keep response concise under 3 lines.`;

      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
      });

      const answer = response.text?.trim() || "Calculation error.";
      await sock.sendMessage(
        jid,
        { text: `🧮 *[CALCULATOR]*\n\n${answer}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Calculator error: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 6. DICTIONARY & DEFINITIONS (!define, !dict)
  // ---------------------------------------------------------------------------
  if (["!define", "!dict", "!vocab"].includes(command)) {
    const word = args[1]?.trim();
    if (!word) {
      await sock.sendMessage(jid, { text: "📖 *Usage:* `!define <word>`\n*Example:* `!define serendipity`" }, { quoted: msg });
      return true;
    }

    try {
      const prompt = `Provide the dictionary definition for "${word}":\n• Phonetic pronunciation / Part of Speech\n• Clear definition\n• 2-3 common Synonyms\n• One clear example sentence in bold.`;

      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
      });

      const definition = response.text?.trim() || "No definition found.";
      await sock.sendMessage(
        jid,
        { text: `📖 *[DICTIONARY: ${word.toUpperCase()}]*\n\n${definition}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Dictionary error: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 7. REAL-TIME WEATHER FORECAST (!weather)
  // ---------------------------------------------------------------------------
  if (["!weather", "!forecast"].includes(command)) {
    const city = args.slice(1).join(" ").trim() || "Delhi";

    try {
      const prompt = `Look up and give the current real-time weather and today's forecast for "${city}". Include:\n• Temperature & Condition (e.g. 28°C Sunny)\n• Humidity, Wind Speed, Feels Like\n• Quick dress/activity tip for today.`;

      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
      });

      const weather = response.text?.trim() || "Weather information unavailable.";
      await sock.sendMessage(
        jid,
        { text: `🌤️ *[WEATHER FORECAST: ${city.toUpperCase()}]*\n\n${weather}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Weather query failed: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 8. GROUP TAG-ALL ANNOUNCEMENT (!tagall, !everyone)
  // ---------------------------------------------------------------------------
  if (["!tagall", "!everyone", "!hidetag"].includes(command)) {
    if (!isGroup) {
      await sock.sendMessage(jid, { text: "⚠️ `!tagall` can only be used in group chats." }, { quoted: msg });
      return true;
    }

    try {
      const groupMetadata = await sock.groupMetadata(jid);
      const participants = groupMetadata.participants || [];

      const senderParticipant = participants.find((p) => p.id === senderId);
      const isAdmin = senderParticipant?.admin === "admin" || senderParticipant?.admin === "superadmin" || msg.key.fromMe;

      if (!isAdmin) {
        await sock.sendMessage(jid, { text: "🚫 Only group admins can use `!tagall`." }, { quoted: msg });
        return true;
      }

      const announcement = args.slice(1).join(" ").trim();
      const mentions = participants.map((p) => p.id);

      const tagText =
        `📢 *[GROUP ANNOUNCEMENT]*\n\n` +
        `${announcement || "Attention all members!"}\n\n` +
        `👥 *Tagged ${participants.length} members.*`;

      await sock.sendMessage(jid, { text: tagText, mentions }, { quoted: msg });
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Tag-all failed: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  // ---------------------------------------------------------------------------
  // 9. QUOTES, FACTS & MOTIVATION (!quote, !fact, !motivation)
  // ---------------------------------------------------------------------------
  if (["!quote", "!motivation", "!fact", "!mindblown"].includes(command)) {
    const isFact = command === "!fact" || command === "!mindblown";
    const prompt = isFact
      ? "Share a fascinating, verified, and mind-blowing science or historical fact in 2-3 engaging sentences."
      : "Share a powerful, inspiring motivational quote with the author's name, and a 1-line thought.";

    try {
      const response = await ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: prompt }] }],
      });

      const text = response.text?.trim() || "Stay curious and keep pushing forward!";
      await sock.sendMessage(
        jid,
        { text: `${isFact ? "💡 *[DID YOU KNOW?]*" : "✨ *[DAILY INSPIRATION]*"}\n\n${text}` },
        { quoted: msg }
      );
      return true;
    } catch (err) {
      return false;
    }
  }

  // ---------------------------------------------------------------------------
  // 10. MEME GENERATOR (!meme <top> | <bottom>)
  // ---------------------------------------------------------------------------
  if (command === "!meme") {
    const imgRef = getImageMessage(msg);
    if (!imgRef) {
      await sock.sendMessage(
        jid,
        { text: "🎭 *Usage:* Reply to an image with `!meme <top text> | <bottom text>`" },
        { quoted: msg }
      );
      return true;
    }

    const memeTextRaw = args.slice(1).join(" ");
    const [topTextRaw, bottomTextRaw] = memeTextRaw.split("|").map((s) => (s || "").trim().toUpperCase());

    try {
      const buffer = await downloadMediaMessage({ message: imgRef }, "buffer", {});
      const image = await loadImage(buffer);

      const canvas = createCanvas(image.width, image.height);
      const ctx = canvas.getContext("2d");

      // Draw original image
      ctx.drawImage(image, 0, 0);

      // Text styling (Classic Impact font style)
      const fontSize = Math.max(24, Math.floor(image.width / 12));
      ctx.font = `bold ${fontSize}px sans-serif`;
      ctx.fillStyle = "white";
      ctx.strokeStyle = "black";
      ctx.lineWidth = Math.max(3, Math.floor(fontSize / 8));
      ctx.textAlign = "center";

      if (topTextRaw) {
        ctx.strokeText(topTextRaw, image.width / 2, fontSize + 10);
        ctx.fillText(topTextRaw, image.width / 2, fontSize + 10);
      }

      if (bottomTextRaw) {
        ctx.strokeText(bottomTextRaw, image.width / 2, image.height - 20);
        ctx.fillText(bottomTextRaw, image.width / 2, image.height - 20);
      }

      const memeBuffer = canvas.toBuffer("image/jpeg");
      await sock.sendMessage(jid, { image: memeBuffer, caption: "🎭 *Custom Meme Generated*" }, { quoted: msg });
      return true;
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Meme creation failed: ${err.message}` }, { quoted: msg });
      return true;
    }
  }

  return false;
}
