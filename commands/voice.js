import { downloadMediaMessage } from "baileys";
import axios from "axios";
import FormData from "form-data";
import fs from "node:fs";
import path from "node:path";
import { tmpdir } from "node:os";
import { randomUUID } from "node:crypto";
import exec from "yt-dlp-exec";
import { searchYoutubeMusic } from "../services/ytmusic.js";

// Configure API endpoint
const API_BASE_URL = process.env.RVC_API_URL || "http://127.0.0.1:8000";

/**
 * PRESETS CONFIGURATION
 * Pitch is semitones (-12 = -1 octave, +12 = +1 octave)
 */
const PRESETS = {
  m2f: { pitch: 12, autotune: true, protect: 0.33, f0_method: "rmvpe" },
  f2m: { pitch: -12, autotune: false, protect: 0.33, f0_method: "rmvpe" },
  m2m: { pitch: 0, autotune: false, protect: 0.33, f0_method: "rmvpe" },
  f2f: { pitch: 0, autotune: true, protect: 0.33, f0_method: "rmvpe" },
};

// Queue state stored in WhatsApp process memory
const queue = [];
let isProcessing = false;

/**
 * Parses user input to extract custom override flags
 */
function parseCustomSettings(args, basePresetName) {
  const baseSettings = PRESETS[basePresetName]
    ? { ...PRESETS[basePresetName] }
    : { pitch: 0, autotune: false, protect: 0.33, f0_method: "rmvpe" };

  args.forEach((arg) => {
    const lower = arg.toLowerCase();

    if (lower.startsWith("pitch:")) {
      const val = parseInt(lower.split(":")[1], 10);
      if (!isNaN(val)) baseSettings.pitch = val;
    }

    if (lower.startsWith("f0:")) {
      const val = lower.split(":")[1];
      if (["rmvpe", "crepe", "harvest", "pm"].includes(val)) {
        baseSettings.f0_method = val;
      }
    }

    if (lower.startsWith("autotune:")) {
      const val = lower.split(":")[1];
      baseSettings.autotune = val === "true" || val === "1";
    }

    if (lower.startsWith("protect:")) {
      const val = parseFloat(lower.split(":")[1]);
      if (!isNaN(val) && val >= 0 && val <= 0.5) {
        baseSettings.protect = val;
      }
    }
  });

  return baseSettings;
}

/**
 * Extract Media URL or Query if `-play`, `-yt`, `-url`, or `play:` flags exist
 */
function extractPlayQuery(rawText) {
  // Matches: -play "query/url", -yt "query/url", -url "url", play:query
  const match = rawText.match(/(?:-play|--play|-yt|--yt|-url|play:|yt:)\s*(?:"([^"]+)"|'([^']+)'|(\S+(?:\s+\S+)*))/i);
  if (!match) return null;
  return match[1] || match[2] || match[3];
}

/**
 * Downloads audio from ANY site (YouTube, Instagram, Reddit, Direct Link) to a temp file
 */
async function fetchAudioFromPlaySource(queryOrUrl) {
  const tmpFile = path.join(tmpdir(), `${randomUUID()}.mp3`);
  const cookiesPath = path.join(process.cwd(), "cookies.txt");
  const isUrl = /^https?:\/\//i.test(queryOrUrl);

  let downloadUrl = queryOrUrl;

  if (!isUrl) {
    try {
      const results = await searchYoutubeMusic(queryOrUrl, 1);
      downloadUrl = results && results.length > 0 ? results[0].url : `ytsearch:${queryOrUrl}`;
    } catch {
      downloadUrl = `ytsearch:${queryOrUrl}`;
    }
  }

  const flags = {
    noPlaylist: true,
    noWarnings: true,
    jsRuntimes: "node",
    output: tmpFile,
    format: "ba/b",
    extractAudio: true,
    audioFormat: "mp3",
    audioQuality: "0",
  };

  if (fs.existsSync(cookiesPath)) {
    flags.cookies = cookiesPath;
  }

  await exec.exec(downloadUrl, flags);

  if (!fs.existsSync(tmpFile)) {
    throw new Error("Failed to download audio source file.");
  }

  return tmpFile;
}

/**
 * Process the queue strictly one by one
 */
async function processQueue(sock) {
  if (isProcessing || queue.length === 0) return;

  isProcessing = true;
  const currentTask = queue[0];
  currentTask.status = "processing";

  try {
    const formData = new FormData();
    formData.append("file", fs.createReadStream(currentTask.tempFilePath));
    formData.append("voice_id", currentTask.voiceId);
    formData.append("pitch", currentTask.settings.pitch);
    formData.append("f0_method", currentTask.settings.f0_method);
    formData.append("autotune", currentTask.settings.autotune ? "true" : "false");
    formData.append("protect", currentTask.settings.protect);
    formData.append("clean_audio", "true");

    let endpoint = "/api/v1/convert-speech";
    if (currentTask.type === "song") {
      endpoint = "/api/v1/convert-song"; // Full song: separates stems via UVR MDX-Net
    } else if (currentTask.type === "acapella") {
      endpoint = "/api/v1/convert-speech"; // Vocals-only song or speech: skips stem separation
    }

    const response = await axios.post(`${API_BASE_URL}${endpoint}`, formData, {
      headers: formData.getHeaders(),
    });

    currentTask.apiJobId = response.data.job_id;
    await pollJobStatus(currentTask);
  } catch (err) {
    console.error("Task Error:", err.message);
    currentTask.status = "failed";
    currentTask.error = err.response?.data?.detail || err.message;
  } finally {
    if (fs.existsSync(currentTask.tempFilePath)) {
      fs.unlinkSync(currentTask.tempFilePath);
    }

    isProcessing = false;

    if (queue.length > 0 && queue[0].status !== "completed") {
      if (queue[0].status === "failed") {
        const failedTask = queue.shift();
        await sock.sendMessage(failedTask.jid, {
          text: `❌ Conversion failed for task: ${failedTask.voiceId}. Error: ${failedTask.error}`
        });
      }
      processQueue(sock);
    }
  }
}

/**
 * Polls API status until completion
 */
async function pollJobStatus(task) {
  return new Promise((resolve) => {
    const interval = setInterval(async () => {
      try {
        const res = await axios.get(`${API_BASE_URL}/api/v1/job-status/${task.apiJobId}`);
        const data = res.data;

        task.progress = data.progress || 0;
        task.step = data.step || "Processing";

        if (data.status === "completed") {
          clearInterval(interval);
          task.status = "completed";
          resolve();
        } else if (data.status === "failed") {
          clearInterval(interval);
          task.status = "failed";
          task.error = data.error || "Processing failed";
          resolve();
        }
      } catch (err) {
        clearInterval(interval);
        task.status = "failed";
        task.error = "Failed to poll job status";
        resolve();
      }
    }, 3000);
  });
}

export async function handleVoiceCommand(sock, jid, senderId, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();

  const PREFIXES = ["!vc", "!voice", "!cover"];
  if (!PREFIXES.includes(command)) return false;

  const subCommand = args[1]?.toLowerCase();

  // 1. Queue Status List (!vc q / !vc queue)
  if (subCommand === "q" || subCommand === "queue") {
    if (queue.length === 0) {
      await sock.sendMessage(jid, { text: "📂 Voice conversion queue is empty." }, { quoted: msg });
      return true;
    }

    let statusText = "🎙️ *Voice Conversion Queue*\n\n";
    queue.forEach((task, index) => {
      const idx = index + 1;
      const icon = task.status === "completed" ? "✅" : task.status === "processing" ? "⚙️" : "⏳";
      statusText += `*#${idx}* ${icon} *Voice:* \`${task.voiceId}\` | *Mode:* \`${task.type.toUpperCase()}\`\n`;
      statusText += `└ *Pitch:* ${task.settings.pitch} | *Status:* ${task.status.toUpperCase()} (${task.progress || 0}%)\n\n`;
    });

    statusText += "👉 Use `!vc status <num>` for details.\n👉 Use `!vc get <num>` to download completed audio.";
    await sock.sendMessage(jid, { text: statusText }, { quoted: msg });
    return true;
  }

  // 2. Specific Job Status by Index (!vc status 1)
  if (subCommand === "status") {
    const index = parseInt(args[2], 10) - 1;
    if (isNaN(index) || !queue[index]) {
      await sock.sendMessage(jid, { text: "⚠️ Invalid queue index. Usage: `!vc status 1`" }, { quoted: msg });
      return true;
    }

    const item = queue[index];
    const text = `📊 *Task Status [#${index + 1}]*\n\n` +
      `• *Voice ID:* ${item.voiceId}\n` +
      `• *Audio Type:* ${item.type.toUpperCase()}\n` +
      `• *Pitch Shift:* ${item.settings.pitch} semitones\n` +
      `• *F0 Method:* ${item.settings.f0_method}\n` +
      `• *Autotune:* ${item.settings.autotune ? "ON" : "OFF"}\n` +
      `• *Protect:* ${item.settings.protect}\n` +
      `• *Status:* ${item.status.toUpperCase()}\n` +
      `• *Progress:* ${item.progress || 0}%\n` +
      `• *Current Step:* ${item.step || "Queued"}`;

    await sock.sendMessage(jid, { text }, { quoted: msg });
    return true;
  }

  // 3. Download Completed Result (!vc get 1)
  if (subCommand === "get") {
    const index = parseInt(args[2], 10) - 1;
    if (isNaN(index) || !queue[index]) {
      await sock.sendMessage(jid, { text: "⚠️ Invalid queue index. Usage: `!vc get 1`" }, { quoted: msg });
      return true;
    }

    const item = queue[index];
    if (item.status !== "completed") {
      await sock.sendMessage(
        jid,
        { text: `⚠️ Task #${index + 1} is not completed yet. Current status: *${item.status.toUpperCase()}*` },
        { quoted: msg }
      );
      return true;
    }

    try {
      await sock.sendMessage(jid, { text: "⬇️ Downloading converted audio..." }, { quoted: msg });

      const res = await axios.get(`${API_BASE_URL}/api/v1/download/${item.apiJobId}`, {
        responseType: "arraybuffer",
      });

      const audioBuffer = Buffer.from(res.data);

      await sock.sendMessage(
        jid,
        {
          audio: audioBuffer,
          mimetype: "audio/mp4",
          ptt: item.type === "speech" || item.type === "acapella",
        },
        { quoted: msg }
      );

      queue.splice(index, 1);
      processQueue(sock);
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Failed to download file: ${err.message}` }, { quoted: msg });
    }
    return true;
  }

  // 4. Add Conversion Task (!vc <voice_id> [preset/overrides] [-play <source>])
  const playQuery = extractPlayQuery(rawText);
  const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
  const quotedMsg = contextInfo?.quotedMessage;
  const audioMsg = msg.message?.audioMessage || quotedMsg?.audioMessage;

  if (!audioMsg && !playQuery) {
    await sock.sendMessage(
      jid,
      {
        text:
          "⚠️ *Voice Conversion Usage:*\n" +
          "Reply to an audio message OR pass download source flag `-play`:\n" +
          "`!vc <voice_id> [preset] [-play <link or query>] [flags]`\n\n" +
          "*Supported Sources:* YouTube, Instagram Reels, TikTok, Reddit, Soundcloud, etc.\n\n" +
          "*Presets:* `m2f`, `f2m`, `m2m`, `f2f`\n" +
          "*Audio Flags:*\n" +
          "• `song` / `inst` -> Full song (runs UVR stem separation)\n" +
          "• `noinst` / `acapella` -> Vocal-only audio\n\n" +
          "*Examples:*\n" +
          "• YouTube: `!vc zero2 m2f -play Blinding Lights song`\n" +
          "• Instagram: `!vc zero2 m2f -play https://instagram.com/reel/... song`\n" +
          "• Audio Note Reply: `!vc zero2 m2f`"
      },
      { quoted: msg }
    );
    return true;
  }

  const voiceId = args[1] || "zero2";
  const presetArg = (args[2] || "m2m").toLowerCase();

  // Detect Audio Structure Type
  let audioType = "speech";
  const lowerArgs = args.map((a) => a.toLowerCase());

  if (lowerArgs.includes("song") || lowerArgs.includes("inst")) {
    audioType = "song";
  } else if (lowerArgs.includes("noinst") || lowerArgs.includes("acapella")) {
    audioType = "acapella";
  }

  const settings = parseCustomSettings(args.slice(2), presetArg);
  let tempFilePath = null;

  // Handle Multi-Platform Play Source Download
  if (playQuery) {
    const statusMsg = await sock.sendMessage(
      jid,
      { text: `⚡ *Fetching audio from media downloader for:* "${playQuery}"...` },
      { quoted: msg }
    );

    try {
      tempFilePath = await fetchAudioFromPlaySource(playQuery);
      await sock.sendMessage(jid, { text: "✅ Audio source downloaded! Adding to RVC queue...", edit: statusMsg.key });
    } catch (err) {
      await sock.sendMessage(jid, { text: `❌ Failed to fetch media source: ${err.message}`, edit: statusMsg.key });
      return true;
    }
  } else {
    // Download raw WhatsApp audio message
    const buffer = await downloadMediaMessage({ message: { audioMessage: audioMsg } }, "buffer", {});
    tempFilePath = path.join(tmpdir(), `input_${Date.now()}.wav`);
    fs.writeFileSync(tempFilePath, buffer);
  }

  const task = {
    id: Date.now().toString(),
    jid,
    senderId,
    type: audioType,
    voiceId,
    presetName: PRESETS[presetArg] ? presetArg : "custom",
    settings,
    tempFilePath,
    status: "queued",
    progress: 0,
    step: "Queued",
    apiJobId: null,
  };

  queue.push(task);
  const queueIndex = queue.length;

  await sock.sendMessage(
    jid,
    {
      text:
        `✅ *Added to Voice Queue! [#${queueIndex}]*\n\n` +
        `• *Voice Model:* \`${voiceId}\`\n` +
        `• *Source:* \`${playQuery ? "External Media (Downloader)" : "WhatsApp Voice Note"}\`\n` +
        `• *Audio Type:* \`${audioType.toUpperCase()}\`\n` +
        `• *Pitch:* \`${settings.pitch} semitones\`\n` +
        `• *Autotune:* \`${settings.autotune ? "ON" : "OFF"}\`\n\n` +
        `Check progress: \`!vc status ${queueIndex}\`\n` +
        `Get result: \`!vc get ${queueIndex}\``
    },
    { quoted: msg }
  );

  processQueue(sock);
  return true;
}