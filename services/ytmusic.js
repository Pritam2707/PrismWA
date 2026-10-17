import yts from "yt-search";
import { spawn } from "node:child_process";
import path from "node:path";
import { GoogleGenAI } from "@google/genai";
import { GEMINI_API_KEY, GEMINI_MODEL } from "../config/constants.js";

const ai = new GoogleGenAI({ apiKey: GEMINI_API_KEY });
const FAST_MODEL = process.env.GEMINI_MODEL || "gemini-3.6-flash";

const YTDLP_BINARY_PATH =
  process.platform === "win32"
    ? path.join(process.cwd(), "node_modules", "yt-dlp-exec", "bin", "yt-dlp.exe")
    : "yt-dlp";

/**
 * Intelligent Music Intent & Query Resolver
 * Detects if a query is a mood/genre/vibe/recommendation request rather than an exact song title,
 * and resolves it to an iconic, critically acclaimed, real song title.
 */
export async function resolveIntelligentMusicQuery(rawQuery) {
  if (!rawQuery || typeof rawQuery !== "string") {
    return { resolvedQuery: rawQuery, isRecommendation: false };
  }

  const clean = rawQuery.trim();
  if (/^https?:\/\//i.test(clean)) {
    return { resolvedQuery: clean, isRecommendation: false };
  }

  // Keywords that signal mood, genre, or non-literal descriptive intent
  const descriptivePatterns = [
    /\b(sad|depressed|crying|heartbreak|breakup|pain|emotional)\b/i,
    /\b(romantic|love|crush|couple|sweet|valentine)\b/i,
    /\b(happy|upbeat|party|dance|energy|hype|energetic|banger)\b/i,
    /\b(workout|gym|running|motivation|hardstyle|phonk|beast)\b/i,
    /\b(chill|lofi|relax|relaxing|sleep|study|night|peaceful|peace)\b/i,
    /\b(nostalgic|retro|90s|80s|2000s|oldies|throwback)\b/i,
    /\b(something (by|like|from|with)|a song (by|like|from)|best song|top song|popular song|good song)\b/i,
    /\b(hindi song|punjabi song|tamil song|telugu song|kpop|anime song|rap song|rock song|pop song)\b/i,
  ];

  const isDescriptive = descriptivePatterns.some((pattern) => pattern.test(clean));

  // If query is very specific with artist and title (e.g. "The Weeknd - Starboy" or "Queen - Bohemian Rhapsody")
  // and has no mood words, we don't need to transform it.
  if (!isDescriptive && clean.split(/\s+/).length > 2 && !clean.toLowerCase().includes("song")) {
    return { resolvedQuery: clean, isRecommendation: false };
  }

  try {
    const prompt = `The user wants to listen to music on WhatsApp matching this request: "${clean}".
Analyze their mood, genre, activity, or artist request and pick ONE specific, globally famous, critically loved, and iconic real hit song that matches this request.
Format your answer in EXACTLY one line as: Artist - Song Name
Example: Adele - Someone Like You
Example: Arijit Singh - Tum Hi Ho
Example: Travis Scott - FE!N
Example: Kordhell - Murder In My Mind
Strictly reply with ONE line: Artist - Song Name`;

    const res = await ai.models.generateContent({
      model: FAST_MODEL,
      contents: [{ role: "user", parts: [{ text: prompt }] }],
    });

    const recommended = res.text?.trim()?.replace(/^["']|["']$/g, "") || clean;

    return {
      resolvedQuery: `${recommended} official audio`,
      displayTitle: recommended,
      isRecommendation: true,
      originalMood: clean,
    };
  } catch (err) {
    console.warn("⚠️ [AI Music Resolver] Fallback to raw query:", err.message);
    return { resolvedQuery: clean, isRecommendation: false };
  }
}

/**
 * Fast YouTube Search
 */
export async function searchYoutubeMusic(query, limit = 5) {
  try {
    const searchResult = await yts(query);
    const videos = searchResult.videos.slice(0, limit);

    return videos.map((v) => ({
      title: v.title,
      url: v.url,
      videoId: v.videoId,
      timestamp: v.timestamp,
      seconds: v.seconds,
      author: v.author.name,
      thumbnail: v.thumbnail,
    }));
  } catch (error) {
    console.error("YouTube Search Error:", error);
    return [];
  }
}

/**
 * Ultra-fast Direct Stream Download into Memory Buffer
 */
export function downloadYoutubeAudioStream(videoUrl) {
  return new Promise((resolve, reject) => {
    const args = [
      videoUrl,
      "-f", "ba/b",
      "-x",
      "--audio-format", "mp3",
      "--audio-quality", "5",
      "-o", "-",
      "--no-playlist",
      "--quiet",
      "--no-warnings"
    ];

    const child = spawn(YTDLP_BINARY_PATH, args);
    const chunks = [];

    child.stdout.on("data", (chunk) => chunks.push(chunk));

    child.on("close", (code) => {
      if (code === 0 && chunks.length > 0) {
        resolve(Buffer.concat(chunks));
      } else {
        reject(new Error(`yt-dlp exited with code ${code}`));
      }
    });

    child.on("error", (err) => reject(err));
  });
}