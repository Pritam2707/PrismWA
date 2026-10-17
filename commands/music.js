import fs, { existsSync, statSync } from "node:fs";
import { unlink } from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import exec from "yt-dlp-exec";
import { searchYoutubeMusic, resolveIntelligentMusicQuery } from "../services/ytmusic.js";

const MAX_UPLOAD_BYTES = 60 * 1024 * 1024; // 60MB safety cap for WhatsApp media
const DOWNLOAD_TIMEOUT_MS = 5 * 60 * 1000; // 5 min hard cap on a single download

/**
 * Creates a visual ASCII progress bar
 */
function makeProgressBar(percent, length = 10) {
  const filled = Math.min(length, Math.max(0, Math.round((percent / 100) * length)));
  const empty = length - filled;
  return "█".repeat(filled) + "░".repeat(empty);
}

/**
 * Downloads media using yt-dlp-exec with progress updates.
 * Cleans up any partial file on failure/timeout so temp dirs don't accumulate.
 */
async function downloadMediaStream(url, isAudioOnly = true, onProgress = null) {
  const tmpFile = path.join(tmpdir(), `${randomUUID()}.${isAudioOnly ? "mp3" : "mp4"}`);
  const cookiesPath = path.join(process.cwd(), "cookies.txt");
  const isYouTube = /(?:youtube\.com|youtu\.be)/i.test(url) || url.startsWith("ytsearch:");

  const buildFlags = (useAndroidClient = false, useCookies = true) => {
    const flags = {
      noPlaylist: true,
      noWarnings: true,
      output: tmpFile,
    };

    if (useCookies && existsSync(cookiesPath) && !useAndroidClient) {
      flags.cookies = cookiesPath;
    }

    if (isYouTube && useAndroidClient) {
      flags.extractorArgs = "youtube:player_client=android";
    }

    if (isAudioOnly) {
      flags.format = "ba/b";
      flags.extractAudio = true;
      flags.audioFormat = "mp3";
      flags.audioQuality = "0";
      flags.postprocessorArgs =
        "ExtractAudio:-ar 44100 -ac 2 -avoid_negative_ts make_zero -fflags +genpts -write_id3v1 1 -id3v2_version 3";
    } else {
      flags.format = "b/m4a/mp4";
      flags.postprocessorArgs = "ffmpeg:-movflags +faststart";
    }

    return flags;
  };

  const attemptDownload = async (flags) => {
    const subprocess = exec.exec(url, flags);
    let lastPercent = 0;

    if (subprocess.stdout) {
      subprocess.stdout.on("data", (chunk) => {
        const text = chunk.toString();
        const percentMatch = text.match(/\[download\]\s+([\d.]+)%/i);
        const speedMatch = text.match(/at\s+([\d.]+\s*\w+\/s)/i);
        const etaMatch = text.match(/ETA\s+([\d:]+)/i);

        if (percentMatch) {
          const percent = parseFloat(percentMatch[1]);
          const speed = speedMatch ? speedMatch[1] : "N/A";
          const eta = etaMatch ? etaMatch[1] : "N/A";

          if (onProgress && Math.floor(percent) > Math.floor(lastPercent)) {
            lastPercent = percent;
            onProgress({ percent, speed, eta });
          }
        }
      });
    }

    let timedOut = false;
    const timeoutTimer = setTimeout(() => {
      timedOut = true;
      try {
        subprocess.kill?.("SIGKILL");
      } catch (_) {}
    }, DOWNLOAD_TIMEOUT_MS);

    try {
      await subprocess;
    } catch (err) {
      await unlink(tmpFile).catch(() => {});
      throw timedOut ? new Error("Download timed out") : err;
    } finally {
      clearTimeout(timeoutTimer);
    }
  };

  try {
    // Attempt 1: Using Android client for YouTube (bypasses bot checks & PO tokens reliably)
    const initialFlags = isYouTube ? buildFlags(true, false) : buildFlags(false, true);
    await attemptDownload(initialFlags);
  } catch (err1) {
    console.warn("⚠️ [yt-dlp] First download pass failed, trying fallback mode...", err1.message);
    // Attempt 2: Fallback with cookies or standard extractor
    const fallbackFlags = isYouTube ? buildFlags(false, true) : buildFlags(false, false);
    await attemptDownload(fallbackFlags);
  }

  if (!existsSync(tmpFile)) {
    throw new Error("Download finished but no output file was produced");
  }

  return tmpFile;
}

/**
 * Unified Downloader & YouTube Search Handler
 */
export async function handlePlayCommand(sock, jid, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();
  const input = args.slice(1).join(" ").trim();

  if (!input) {
    await sock.sendMessage(
      jid,
      {
        text: `🎵 *Media Downloader & Search Help:*\n\n` +
              `• \`!play <song name or link>\` : Download MP3 Audio\n` +
              `• \`!song <query>\` : Search and send audio\n` +
              `• \`!video <link>\` : Download Video (YouTube, Instagram, Reddit, TikTok, X)\n` +
              `• \`!ig <link>\` : Download Instagram Reel/Post\n` +
              `• \`!yt <query>\` : Search YouTube and display top 5 results\n` +
              `• \`!reddit <link>\` : Download Reddit Video`,
      },
      { quoted: msg }
    );
    return;
  }

  const isUrl = /^https?:\/\//i.test(input);
  const isVideoRequested = command === "!video" || command === "!ig" || command === "!reddit" || command === "!mp4" || command === "!reel" || command === "!tiktok";
  let downloadUrl = input;

  // 1. YouTube Search List Feature (!yt <query>)
  if (command === "!yt" && !isUrl) {
    try {
      const results = await searchYoutubeMusic(input, 5);
      if (!results || results.length === 0) {
        await sock.sendMessage(jid, { text: "❌ No results found on YouTube." }, { quoted: msg });
        return;
      }

      let listText = `🎵 *YouTube Search Results for:* "${input}"\n\n`;
      results.forEach((item, index) => {
        listText += `${index + 1}. *${item.title}*\n🔗 ${item.url}\n\n`;
      });
      listText += `💡 *Reply with \`!play <link>\` to download audio.*`;

      await sock.sendMessage(jid, { text: listText }, { quoted: msg });
      return;
    } catch (e) {
      console.error("YouTube Search Error:", e);
      await sock.sendMessage(jid, { text: "❌ Failed to search YouTube." }, { quoted: msg });
      return;
    }
  }

  // FIX: `!yt <url>` fell through to the generic download path and silently
  // downloaded audio with no explanation — `!yt` is documented purely as a
  // search command, so a URL here should just point the user at `!play`/`!video`.
  if (command === "!yt" && isUrl) {
    await sock.sendMessage(
      jid,
      { text: "ℹ️ `!yt` is for searching by name. Use `!play <link>` for audio or `!video <link>` for video." },
      { quoted: msg }
    );
    return;
  }

  let resolvedDisplayTitle = null;
  let isAiRecommended = false;

  // 2. Intelligent Search & Intent Resolution for non-URL queries
  if (!isUrl) {
    try {
      const queryInfo = await resolveIntelligentMusicQuery(input);
      isAiRecommended = queryInfo.isRecommendation;
      resolvedDisplayTitle = queryInfo.displayTitle || null;

      const searchQuery = queryInfo.resolvedQuery || input;
      const results = await searchYoutubeMusic(searchQuery, 1);

      if (results && results.length > 0) {
        downloadUrl = results[0].url;
        if (!resolvedDisplayTitle) resolvedDisplayTitle = results[0].title;
      } else {
        downloadUrl = `ytsearch:${searchQuery}`;
      }
    } catch (e) {
      downloadUrl = `ytsearch:${input}`;
    }
  }

  // FIX: `!ig` / `!reddit` require a real link
  if (!isUrl && (command === "!ig" || command === "!reddit")) {
    await sock.sendMessage(
      jid,
      { text: `⚠️ \`${command}\` needs a direct link, e.g. \`${command} https://...\`` },
      { quoted: msg }
    );
    return;
  }

  const initialStatusText = isAiRecommended && resolvedDisplayTitle
    ? `🎧 *Found matching song:* "${resolvedDisplayTitle}"\n✨ *Requested Mood:* "${input}"\n⚡ *Starting download...*`
    : "⚡ *Starting download... Please wait!*";

  // Send status message to update
  const statusMsg = await sock.sendMessage(
    jid,
    { text: initialStatusText },
    { quoted: msg }
  );

  let lastEditTime = 0;

  // Throttle updates to prevent WhatsApp rate limits (1.5s interval)
  const handleProgress = async ({ percent, speed, eta }) => {
    const now = Date.now();
    if (now - lastEditTime < 1500 && percent < 100) return;
    lastEditTime = now;

    const bar = makeProgressBar(percent);
    const titleHeader = resolvedDisplayTitle ? `🎶 *${resolvedDisplayTitle}*\n` : "";
    const progressText =
      `${titleHeader}⬇️ *Downloading ${isVideoRequested ? "Video" : "Audio"}...*\n\n` +
      `\`[${bar}]\` *${percent.toFixed(1)}%*\n` +
      `🚀 *Speed:* ${speed} | ⏱️ *ETA:* ${eta}`;

    try {
      await sock.sendMessage(jid, { text: progressText, edit: statusMsg.key });
    } catch (err) {}
  };

  let filePath = null;
  let uploadInterval = null;

  try {
    filePath = await downloadMediaStream(downloadUrl, !isVideoRequested, handleProgress);
    const fileStats = statSync(filePath);
    const fileSizeMB = (fileStats.size / (1024 * 1024)).toFixed(1);

    if (fileStats.size > MAX_UPLOAD_BYTES) {
      await sock.sendMessage(
        jid,
        {
          text: `❌ *File too large to send* (${fileSizeMB} MB). Try a shorter clip or lower quality.`,
          edit: statusMsg.key,
        }
      );
      return;
    }

    // Simulate progress updates during WhatsApp upload
    let uploadProgress = 0;
    uploadInterval = setInterval(async () => {
      uploadProgress = Math.min(95, uploadProgress + 25);
      const bar = makeProgressBar(uploadProgress);
      const uploadText =
        `📤 *Uploading to WhatsApp... (${fileSizeMB} MB)*\n\n` +
        `\`[${bar}]\` *${uploadProgress}%*`;

      try {
        await sock.sendMessage(jid, { text: uploadText, edit: statusMsg.key });
      } catch (err) {}
    }, 1200);

    const captionText = isAiRecommended
      ? `🎧 *Now Playing:* ${resolvedDisplayTitle}\n✨ *Matched Mood:* "${input}"`
      : `🎥 *Downloaded Content*\n🔗 *Source:* ${isUrl ? input : downloadUrl}`;

    const mediaPayload = isVideoRequested
      ? {
          video: { url: filePath },
          caption: captionText,
          mimetype: "video/mp4",
        }
      : {
          audio: { url: filePath },
          mimetype: "audio/mpeg",
          ptt: false,
        };

    await sock.sendMessage(jid, mediaPayload, { quoted: msg });

    // Clean up intermediate progress card from chat to prevent clutter
    if (statusMsg?.key) {
      await sock.sendMessage(jid, { delete: statusMsg.key }).catch(() => {});
    }
  } catch (error) {
    console.error("Download Error:", error);
    if (statusMsg?.key) {
      await sock.sendMessage(
        jid,
        { text: "❌ *Download Failed!* Ensure the link is public or try again later.", edit: statusMsg.key }
      );
    }
  } finally {

    if (uploadInterval) clearInterval(uploadInterval);

    // Clean up temporary file regardless of outcome
    if (filePath) await unlink(filePath).catch(() => {});
  }
}