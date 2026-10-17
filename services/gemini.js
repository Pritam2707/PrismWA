import { GoogleGenAI } from "@google/genai";
import {
  GEMINI_MODEL,
  GEMINI_API_KEY,
  BASE_INSTRUCTION,
  MAX_HISTORY_TURNS,
  MAX_STORED_CHATS,
  VOICE_IDS,
} from "../config/constants.js";
import { getFreeSearchResults } from "./search.js";

const ai = new GoogleGenAI({ apiKey: GEMINI_API_KEY });
const historyStore = new Map();

const FAST_MODEL = process.env.GEMINI_FAST_MODEL || GEMINI_MODEL || "gemini-3.5-flash-lite";

const SEARCH_TIMEOUT_MS = 8000;
const DECISION_TIMEOUT_MS = 3000;
const GEMINI_TIMEOUT_MS = 25000;
const SEARCH_CACHE_TTL_MS = 5 * 60 * 1000;

const searchCache = new Map();

const AVAILABLE_TTS_VOICES = Object.keys(VOICE_IDS || {});
const DEFAULT_VOICE = AVAILABLE_TTS_VOICES[0] || "trump";

function withTimeout(promise, ms, label) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => reject(new Error(`${label} timed out after ${ms}ms`)), ms);
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

async function aiDecidesIfSearchNeeded(text) {
  if (!text || text.length < 5) return false;

  try {
    const decisionPrompt = `Analyze this message: "${text}"\nDo we need live web search for real-time news, current dates, or factual external lookup? Reply strictly ONE WORD: "YES" or "NO".`;

    const decisionResponse = await withTimeout(
      ai.models.generateContent({
        model: FAST_MODEL,
        contents: [{ role: "user", parts: [{ text: decisionPrompt }] }],
        config: { maxOutputTokens: 2, temperature: 0 },
      }),
      DECISION_TIMEOUT_MS,
      "fast-intent-check"
    );

    const choice = decisionResponse.text?.trim().toUpperCase();
    return choice?.includes("YES") ?? false;
  } catch (err) {
    return false;
  }
}

function getCachedSearch(query) {
  const key = query.toLowerCase().trim();
  const cached = searchCache.get(key);
  if (cached && cached.expiresAt > Date.now()) return cached.result;
  if (cached) searchCache.delete(key);
  return undefined;
}

function setCachedSearch(query, result) {
  const key = query.toLowerCase().trim();
  searchCache.set(key, { result, expiresAt: Date.now() + SEARCH_CACHE_TTL_MS });
}

async function fetchSearchContext(text) {
  const cached = getCachedSearch(text);
  if (cached !== undefined) return cached;

  try {
    const result = await withTimeout(getFreeSearchResults(text), SEARCH_TIMEOUT_MS, "web-search");
    setCachedSearch(text, result);
    return result;
  } catch (err) {
    return null;
  }
}

export function getHistory(key) {
  return historyStore.get(key) || [];
}

export function pushHistory(key, userParts, modelText) {
  const history = getHistory(key);
  history.push({ role: "user", parts: userParts });
  history.push({ role: "model", parts: [{ text: modelText }] });
  const trimmed = history.slice(-MAX_HISTORY_TURNS * 2);

  historyStore.delete(key);
  if (historyStore.size >= MAX_STORED_CHATS) {
    const oldestKey = historyStore.keys().next().value;
    historyStore.delete(oldestKey);
  }
  historyStore.set(key, trimmed);
}

export async function solveWithGemini({ historyKey, persona, text, imageBuffer, mimeType }) {
  const userParts = [];

  if (imageBuffer) {
    userParts.push({
      inlineData: {
        mimeType: mimeType || "image/jpeg",
        data: imageBuffer.toString("base64"),
      },
    });
  }

  let promptText = text?.length
    ? text
    : imageBuffer
      ? "Solve the question in the image."
      : "(sent trigger word, say hi and ask what's up)";

  if (text && !imageBuffer) {
    const needsSearch = await aiDecidesIfSearchNeeded(text);

    if (needsSearch) {
      console.log(`🧠 Flash Decision: Live search required for -> "${text}"`);
      const searchContext = await fetchSearchContext(text);

      if (searchContext && searchContext.trim().length > 0) {
        promptText = `[REAL-TIME SEARCH CONTEXT INJECTED]:\n${searchContext}\n\n[USER QUESTION]: ${text}\n\nAnswer directly using the facts provided while staying in character (${persona.flavor}).`;
      }
    } else {
      console.log(`⚡ Flash Decision: No search needed for -> "${text}"`);
    }
  }

  userParts.push({ text: promptText });
  const contents = [...getHistory(historyKey), { role: "user", parts: userParts }];

  let response;
  try {
    const voicesInstruction = AVAILABLE_TTS_VOICES.length > 0
      ? `Available voice names for TTS: (${AVAILABLE_TTS_VOICES.join(", ")}).`
      : "";

    const toolExamples = `
[TOOL USAGE EXAMPLES]:
- User: "Say hello using Donald Trump voice"
  -> Call tool: text_to_speech(voice="trump", text="Hello everyone!")

- User: "Play shape of you"
  -> Call tool: play_music(query="shape of you")

- User: "Show me a photo of a Ferrari"
  -> Call tool: search_image(query="Ferrari")
`;

    response = await withTimeout(
      ai.models.generateContent({
        model: GEMINI_MODEL,
        contents,
        config: {
          systemInstruction: `${BASE_INSTRUCTION}\n\n${persona.flavor}\n\n${voicesInstruction}\n\n${toolExamples}`,
          tools: [
            {
              functionDeclarations: [
                {
                  name: "play_music",
                  description: "Plays, downloads, or searches for audio/music when explicitly requested.",
                  parameters: {
                    type: "OBJECT",
                    properties: { query: { type: "STRING" } },
                    required: ["query"],
                  },
                },
                {
                  name: "text_to_speech",
                  description: "Converts text into spoken voice audio when asked to speak or read out loud.",
                  parameters: {
                    type: "OBJECT",
                    properties: {
                      voice: {
                        type: "STRING",
                        enum: AVAILABLE_TTS_VOICES,
                        description: "The name of the voice character/model to speak with.",
                      },
                      text: { type: "STRING", description: "The message text to be spoken out loud." },
                    },
                    required: ["voice", "text"],
                  },
                },
                {
                  name: "search_image",
                  description: "Searches for pictures when asked to show or find an image.",
                  parameters: {
                    type: "OBJECT",
                    properties: { query: { type: "STRING" } },
                    required: ["query"],
                  },
                },
              ],
            },
          ],
          toolConfig: { functionCallingConfig: { mode: "AUTO" } },
          safetySettings: [
            { category: "HARM_CATEGORY_HARASSMENT", threshold: "BLOCK_NONE" },
            { category: "HARM_CATEGORY_HATE_SPEECH", threshold: "BLOCK_NONE" },
            { category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", threshold: "BLOCK_NONE" },
            { category: "HARM_CATEGORY_DANGEROUS_CONTENT", threshold: "BLOCK_NONE" },
          ],
        },
      }),
      GEMINI_TIMEOUT_MS,
      "generate-response"
    );
  } catch (err) {
    console.error("❌ Gemini generation error:", err.message);
    return {
      type: "text",
      answer: "Sorry, something went wrong generating a reply. Try again in a bit.",
      mood: "none",
    };
  }

  const functionCalls = response.functionCalls;
  if (functionCalls && functionCalls.length > 0) {
    const call = functionCalls[0];

    if (call.name === "text_to_speech" && call.args) {
      if (!call.args.voice) {
        call.args.voice = DEFAULT_VOICE;
      }
    }

    console.log(`🛠️ [GEMINI TOOL CALL DETECTED]: ${call.name}`, call.args);
    return {
      type: "function_call",
      name: call.name,
      args: call.args,
    };
  }

  const rawAnswer = response.text?.trim() || "Sorry, I couldn't generate a reply for that.";
  const moodMatch = rawAnswer.match(/\[MOOD:\s*(\w+)\]/i);
  const mood = moodMatch ? moodMatch[1].toLowerCase() : "none";
  const cleanAnswer = rawAnswer.replace(/\[MOOD:\s*\w+\]/gi, "").trim();

  pushHistory(
    historyKey,
    [{ text: text?.length ? text : imageBuffer ? "[image question]" : "[trigger only]" }],
    cleanAnswer
  );

  return { type: "text", answer: cleanAnswer, mood };
}