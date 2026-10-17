import { GoogleGenAI } from "@google/genai";
import axios from "axios";
import https from "https";
import { GEMINI_API_KEY } from "../config/constants.js";

const MAX_PROMPT_LENGTH = 400;
const GEMINI_TIMEOUT_MS = 15000;
const RENDER_TIMEOUT_MS = 30000;
const CACHE_TTL_MS = 10 * 60 * 1000; // 10 minutes
const MAX_RETRIES = 1; // one retry on top of the initial attempt

const ai = new GoogleGenAI({ apiKey: GEMINI_API_KEY });

// FIX: `rejectUnauthorized: false` disabled TLS certificate validation on
// every single request, permanently, exposing image bytes to MITM tampering.
// A keep-alive agent with a sane timeout/family setting addresses the
// original "handshake hang" problem without turning off cert verification.
const httpsAgent = new https.Agent({
  keepAlive: true,
  family: 4, // avoid slow/broken IPv6 handshakes on some hosts
});

// Simple in-memory cache to avoid re-spending Gemini quota / re-rendering
// identical prompts back to back (e.g. accidental double-sends, retries).
const promptCache = new Map(); // key -> { value, expiresAt }

function cacheGet(key) {
  const entry = promptCache.get(key);
  if (!entry) return null;
  if (Date.now() > entry.expiresAt) {
    promptCache.delete(key);
    return null;
  }
  return entry.value;
}

function cacheSet(key, value) {
  promptCache.set(key, { value, expiresAt: Date.now() + CACHE_TTL_MS });
}

/**
 * Truncates to MAX_PROMPT_LENGTH without cutting a word in half.
 */
function truncatePrompt(text, maxLen) {
  if (text.length <= maxLen) return text;
  const slice = text.slice(0, maxLen);
  const lastSpace = slice.lastIndexOf(" ");
  return lastSpace > 0 ? slice.slice(0, lastSpace) : slice;
}

/**
 * Wraps a promise with a timeout so a hung API call can't stall the whole pipeline.
 */
function withTimeout(promise, ms, label) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${label} timed out after ${ms}ms`)), ms);
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

/**
 * Retries an async function once on failure, with a short backoff delay.
 */
async function withRetry(fn, retries, label) {
  let lastErr;
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      return await fn();
    } catch (err) {
      lastErr = err;
      if (attempt < retries) {
        console.warn(`⚠️ ${label} failed (attempt ${attempt + 1}/${retries + 1}): ${err.message}. Retrying...`);
        await new Promise((r) => setTimeout(r, 1500));
      }
    }
  }
  throw lastErr;
}

/**
 * Uses Gemini 3.5 Flash Lite (500 free daily calls) to engineer a high-detail image prompt
 */
async function expandPromptWithGemini(userPrompt, isAnime = false) {
  const cacheKey = `gemini:${isAnime ? "anime" : "real"}:${userPrompt}`;
  const cached = cacheGet(cacheKey);
  if (cached) return cached;

  try {
    const styleInstruction = isAnime
      ? "Format this as an expert prompt for an Anime/Manga art generator. Include details about vibrant colors, clean linework, lighting, digital art style, and composition."
      : "Format this as an expert photorealistic/digital art prompt. Include details about camera angle, cinematic lighting, textures, mood, and resolution.";

    const systemPrompt =
      `You are an expert AI image prompt engineer. ` +
      `Expand the user's input into a vivid, descriptive, 2 to 3 sentence image generation prompt. ` +
      `${styleInstruction} ` +
      `CRITICAL RULE: Return ONLY the final expanded prompt text. Do NOT add conversation, quotes, or markdown code blocks.`;

    const response = await withRetry(
      () =>
        withTimeout(
          ai.models.generateContent({
            model: "gemini-3.5-flash-lite",
            contents: [
              { text: systemPrompt },
              { text: `User Idea: "${userPrompt}"` },
            ],
            config: {
              maxOutputTokens: 200,
              // NOTE: temperature/top_p/top_k are deprecated and ignored by
              // this model generation, so they're intentionally omitted here.
            },
          }),
          GEMINI_TIMEOUT_MS,
          "Gemini prompt expansion"
        ),
      MAX_RETRIES,
      "Gemini prompt expansion"
    );

    const enhancedText = response.text?.trim();
    if (enhancedText) {
      console.log(`🧠 [Gemini Prompt Engineer]: "${enhancedText}"`);
      cacheSet(cacheKey, enhancedText);
      return enhancedText;
    }
  } catch (err) {
    console.warn(`⚠️ Gemini prompt expansion failed (${err.message}). Using original prompt.`);
  }

  // Fallback to basic prompt string if Gemini fails
  return userPrompt;
}

/**
 * Renders the image via Pollinations AI using the Gemini-engineered prompt
 */
async function renderImage(enhancedPrompt, isAnime = false) {
  const encoded = encodeURIComponent(enhancedPrompt);
  const seed = Math.floor(Math.random() * 1000000);
  const model = isAnime ? "midijourney" : "flux";

  const url = `https://image.pollinations.ai/prompt/${encoded}?width=1024&height=1024&model=${model}&nologo=true&seed=${seed}`;

  const response = await withRetry(
    () =>
      axios.get(url, {
        responseType: "arraybuffer",
        httpsAgent,
        timeout: RENDER_TIMEOUT_MS,
        headers: {
          "User-Agent":
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36",
        },
      }),
    MAX_RETRIES,
    "Pollinations render"
  );

  // FIX: with responseType "arraybuffer" in Node, axios already hands back a
  // Buffer — re-wrapping it in Buffer.from(data, "binary") just did a
  // pointless extra copy (and the "binary" encoding arg was silently ignored
  // since the input wasn't a string anyway).
  return Buffer.isBuffer(response.data) ? response.data : Buffer.from(response.data);
}

import { sendReaction, clearReaction } from "../services/presence.js";

export async function handleGenerateImageCommand(sock, jid, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();

  const GEN_COMMANDS = ["!gen", "!generate", "!imagine", "!draw", "!anime"];

  if (!GEN_COMMANDS.includes(command)) {
    return false;
  }

  const isAnime = command === "!anime";
  const userPrompt = truncatePrompt(args.slice(1).join(" ").trim(), MAX_PROMPT_LENGTH);

  if (!userPrompt) {
    let helpText =
      "🎨 *AI Image Generator Commands:* \n\n" +
      "• `!gen <prompt>` : Generate realistic or digital art\n" +
      "• `!anime <prompt>` : Generate high-quality 2D anime art\n\n" +
      "*Examples:*\n" +
      "• `!gen a cat floating in space`\n" +
      "• `!anime domain expansion`";

    await sock.sendMessage(jid, { text: helpText }, { quoted: msg });
    return true;
  }

  // De-bloat: Use emoji reaction on the user's message instead of sending an extra text message
  await sendReaction(sock, jid, msg.key, isAnime ? "🌸" : "🎨");

  try {
    // 1. Expand prompt using Gemini 3.5 Flash Lite
    const masterPrompt = await expandPromptWithGemini(userPrompt, isAnime);

    // 2. Render image using Pollinations
    const imageBuffer = await renderImage(masterPrompt, isAnime);

    await sock.sendMessage(
      jid,
      {
        image: imageBuffer,
        caption: `✨ *Prompt:* ${userPrompt}`,
      },
      { quoted: msg }
    );
    await sendReaction(sock, jid, msg.key, "✅");
  } catch (err) {
    console.error("Image Generation Pipeline Error:", err.message);
    await sendReaction(sock, jid, msg.key, "❌");
    await sock.sendMessage(
      jid,
      { text: "❌ *Failed to generate image.* Image servers are currently busy, please try again." },
      { quoted: msg }
    );
  }

  return true;
}