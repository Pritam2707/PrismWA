import axios from "axios";
import { FISH_AUDIO_API_KEY, FISH_AUDIO_MODEL, VOICE_IDS } from "../config/constants.js";

/**
 * Generates TTS audio buffer using Fish Audio API
 * @param {string} text - Text to speak
 * @param {string} voiceKey - Key from VOICE_IDS (e.g., 'zero2_jp', 'elon_musk')
 * @returns {Promise<Buffer|null>} Audio buffer or null on failure
 */
export async function generateTts(text, voiceKey) {
  if (!FISH_AUDIO_API_KEY) {
    console.warn("FISH_AUDIO_API_KEY is missing in .env");
    return null;
  }

  const voiceId = VOICE_IDS[voiceKey];
  if (!voiceId) {
    console.warn(`No voice ID mapped for key: ${voiceKey}`);
    return null;
  }

  // Basic sanitization
  const sanitizedText = text
    .replace(/```[\s\S]*?```/g, "")
    .replace(/\*[^\*]+?\*/g, (m) => m.slice(1, -1))
    .trim();

  if (!sanitizedText) return null;

  try {
    const response = await axios({
      method: "POST",
      url: "https://api.fish.audio/v1/tts",
      headers: {
        Authorization: `Bearer ${FISH_AUDIO_API_KEY}`,
        model: FISH_AUDIO_MODEL,
        "Content-Type": "application/json",
      },
      data: {
        text: sanitizedText,
        reference_id: voiceId,
        format: "mp3",
      },
      responseType: "arraybuffer",
    });

    return Buffer.from(response.data);
  } catch (error) {
    console.error("Fish Audio TTS Error:", error.response?.data?.toString() || error.message);
    return null;
  }
}

/**
 * Command Handler for `!tts <voice_name> <text>`
 */
export async function handleTtsCommand(sock, jid, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const voiceKey = args[1]?.toLowerCase();
  const textToSay = args.slice(2).join(" ");

  if (!voiceKey || !textToSay) {
    const available = Object.keys(VOICE_IDS).map((name) => `• \`${name}\``).join("\n");
    await sock.sendMessage(
      jid,
      {
        text: `🗣️ *TTS Usage:*\n\`!tts <voicename> <text>\` \n\n*Example:*\n\`!tts elon_musk Hello world!\` \n\n*Available Voices:*\n${available}`,
      },
      { quoted: msg }
    );
    return;
  }

  if (!VOICE_IDS[voiceKey]) {
    const available = Object.keys(VOICE_IDS).map((name) => `• \`${name}\``).join("\n");
    await sock.sendMessage(
      jid,
      {
        text: `❌ Unknown voice name *"${voiceKey}"*.\n\n*Available Voices:*\n${available}`,
      },
      { quoted: msg }
    );
    return;
  }

  const audioBuffer = await generateTts(textToSay, voiceKey);
  if (audioBuffer) {
    // Send as standard MP3 audio file instead of a PTT voice note
    await sock.sendMessage(
      jid,
      {
        audio: audioBuffer,
        mimetype: "audio/mpeg",
        ptt: false,
        fileName: `${voiceKey}_tts.mp3`,
      },
      { quoted: msg }
    );
  } else {
    await sock.sendMessage(
      jid,
      { text: "❌ Failed to generate audio. Please check your `FISH_AUDIO_API_KEY` in .env." },
      { quoted: msg }
    );
  }
}

/**
 * Returns a formatted string listing all voice names without IDs
 */
export function getVoiceListText() {
  const voiceNames = Object.keys(VOICE_IDS);
  if (voiceNames.length === 0) {
    return "🎙️ *Available Voice Models:*\n\nNo voices configured in `config/constants.js`.";
  }

  const listLines = voiceNames.map((name) => `• *${name}*`).join("\n");
  return `🎙️ *Available Voice Models*\n\n${listLines}\n\n*Usage:* \`!tts <voicename> <text>\``;
}