import axios from "axios";
import https from "https";
import { getFreeImageSearch } from "../services/search.js";
import { checkImageSafety } from "../services/moderation.js";

const MAX_QUERY_LENGTH = 100;

// Custom HTTPS Agent to safely fetch images from servers with broken/incomplete SSL chains
const insecureAgent = new https.Agent({
  rejectUnauthorized: false,
});

// Basic first-line-of-defense keyword blocklist for the query itself.
const NSFW_KEYWORDS = [
  "porn", "nsfw", "nude", "naked", "xxx", "sex", "hentai", "boobs",
];

function isQuerySuspicious(query) {
  const lower = query.toLowerCase();
  return NSFW_KEYWORDS.some((word) => lower.includes(word));
}

/**
 * Downloads an image safely into a Buffer, bypassing leaf certificate SSL errors
 */
async function fetchImageBuffer(url) {
  const response = await axios.get(url, {
    responseType: "arraybuffer",
    httpsAgent: insecureAgent,
    timeout: 10000,
    headers: {
      "User-Agent":
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36",
    },
  });
  return Buffer.from(response.data, "binary");
}

import { sendReaction } from "../services/presence.js";

export async function handleImageSearchCommand(sock, jid, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();

  // Extract query whether it comes from '!img <query>' or directly as 'query'
  let query = "";
  if (command === "!img" || command === "!image") {
    query = args.slice(1).join(" ").trim().slice(0, MAX_QUERY_LENGTH);
  } else {
    query = rawText.trim().slice(0, MAX_QUERY_LENGTH);
  }

  if (!query) {
    await sock.sendMessage(
      jid,
      { text: "🖼️ *Usage:* `!img <search query>`\n*Example:* `!img Apple`" },
      { quoted: msg }
    );
    return true;
  }

  if (isQuerySuspicious(query)) {
    await sock.sendMessage(
      jid,
      { text: "🚫 That search term isn't allowed." },
      { quoted: msg }
    );
    return true;
  }

  // De-bloat: Use emoji reaction on user message
  await sendReaction(sock, jid, msg.key, "🔍");

  try {
    const imageUrl = await getFreeImageSearch(query);

    if (!imageUrl) {
      await sendReaction(sock, jid, msg.key, "❌");
      await sock.sendMessage(
        jid,
        { text: `❌ No image results found for "${query}".` },
        { quoted: msg }
      );
      return true;
    }

    // Safety check on image URL
    const safe = await checkImageSafety(imageUrl).catch(() => null);

    if (safe === false) {
      await sendReaction(sock, jid, msg.key, "🚫");
      await sock.sendMessage(
        jid,
        { text: `🚫 The top result for "${query}" was flagged as inappropriate and was blocked.` },
        { quoted: msg }
      );
      return true;
    }

    // 🚨 Fetch image as Buffer using insecureAgent to resolve SSL leaf certificate errors
    const imageBuffer = await fetchImageBuffer(imageUrl);

    await sock.sendMessage(
      jid,
      {
        image: imageBuffer,
        caption: `🖼️ *Image Result:* ${query}`,
      },
      { quoted: msg }
    );
    await sendReaction(sock, jid, msg.key, "✅");
  } catch (err) {
    console.error("Image Command Error:", err);
    await sendReaction(sock, jid, msg.key, "❌");
    await sock.sendMessage(
      jid,
      { text: "❌ Failed to fetch image. The target URL might be protected or unreachable." },
      { quoted: msg }
    );
  }

  return true;
}