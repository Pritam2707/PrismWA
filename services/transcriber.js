import { downloadMediaMessage } from "baileys";
import { GoogleGenAI } from "@google/genai";

const ai = new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY });
const GEMINI_MODEL = process.env.GEMINI_MODEL || "gemini-3.5-flash-lite";

// Gemini's inline (base64) payload has a practical ceiling — beyond this,
// requests should go through the Files API instead. We just refuse early
// with a clear message rather than letting the API throw a cryptic error.
const MAX_INLINE_BYTES = 19 * 1024 * 1024; // ~19MB, safely under the ~20MB cap

/**
 * Single source of truth for per-media-type behavior — fallback mimeType
 * and default prompt. Add new media types here instead of touching three
 * separate if-chains.
 */
const MEDIA_CONFIG = {
  audio: {
    detect: (m) => m.audioMessage,
    fallbackMime: "audio/ogg",
    prompt:
      "Transcribe the following voice note verbatim into plain text. Do not add intro/outro commentary, formatting headers, or fluff. If it is in another language, transcribe it in its original language.",
  },
  image: {
    detect: (m) => m.imageMessage,
    fallbackMime: "image/jpeg",
    prompt:
      "Describe and extract all visible text or key details from this image clearly and concisely.",
  },
  video: {
    detect: (m) => m.videoMessage,
    fallbackMime: "video/mp4",
    prompt:
      "Summarize the key events and spoken content in this video concisely.",
  },
  document: {
    detect: (m) => m.documentMessage,
    fallbackMime: "application/pdf",
    prompt:
      "Provide a clean, well-structured summary of the key information and main points inside this PDF document.",
  },
};

/**
 * Extracts and processes Audio, Image, Video, or PDF media messages using Gemini.
 */
export async function transcribeMediaMessage(msg, customPrompt = null) {
  try {
    const m = msg.message || msg;

    // Find which media type this message contains
    const entry = Object.entries(MEDIA_CONFIG).find(([, cfg]) => cfg.detect(m));

    if (!entry) {
      throw new Error("No supported media (Audio, Image, Video, PDF) found in message.");
    }

    const [mediaType, cfg] = entry;
    const mediaObj = cfg.detect(m);

    console.log(`📎 [Transcriber] Detected media type: ${mediaType}`);

    // Determine clean mimeType (strip codec params like "; codecs=opus")
    const rawMime = mediaObj.mimetype || "";
    let mimeType = rawMime.split(";")[0].trim() || cfg.fallbackMime;

    // Documents: only PDF is supported
    if (mediaType === "document") {
      const looksLikePdf =
        mimeType === "application/pdf" ||
        mediaObj.fileName?.toLowerCase().endsWith(".pdf");
      if (!looksLikePdf) {
        return "❌ Only PDF document processing is supported.";
      }
      mimeType = "application/pdf";
    }

    const promptText = customPrompt || cfg.prompt;

    // Download media buffer
    const mediaBuffer = await downloadMediaMessage(msg, "buffer", {});

    if (mediaBuffer.length > MAX_INLINE_BYTES) {
      const sizeMB = (mediaBuffer.length / (1024 * 1024)).toFixed(1);
      console.warn(`⚠️ [Transcriber] Media too large for inline processing: ${sizeMB}MB`);
      return `❌ This file is too large to process (${sizeMB}MB). Please send something under ~19MB.`;
    }

    console.log(
      `📤 [Transcriber] Sending ${mediaType} (${mimeType}, ${(mediaBuffer.length / 1024).toFixed(0)}KB) to Gemini...`
    );

    const response = await ai.models.generateContent({
      model: GEMINI_MODEL,
      contents: [
        {
          inlineData: {
            mimeType,
            data: mediaBuffer.toString("base64"),
          },
        },
        {
          text: promptText,
        },
      ],
    });

    const result = response.text?.trim() || "Could not process media.";
    console.log(`✅ [Transcriber] Gemini responded (${result.length} chars).`);
    return result;
  } catch (err) {
    console.error("❌ [Transcriber] Multimodal Media Processing Error:", err);
    return null;
  }
}