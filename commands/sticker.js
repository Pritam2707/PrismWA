import { downloadMediaMessage } from "baileys";
import ffmpeg from "fluent-ffmpeg";
import sharp from "sharp";
import fs from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import { sendTypingStatus, sendPausedStatus } from "../services/presence.js";

const STICKER_SIZE = 512; // WhatsApp expects 512x512 canvases for stickers

/**
 * Parses time argument like "3-7" or "00:01-00:05" into start time and duration in seconds
 */
function parseTimeRange(timeStr) {
  if (!timeStr || !timeStr.includes("-")) {
    return { startTime: "00:00:00", duration: 5 };
  }

  const [startRaw, endRaw] = timeStr.split("-").map((s) => s.trim());

  const parseToSeconds = (val) => {
    if (val.includes(":")) {
      const parts = val.split(":").map(Number);
      if (parts.length === 3) return parts[0] * 3600 + parts[1] * 60 + parts[2];
      if (parts.length === 2) return parts[0] * 60 + parts[1];
    }
    return parseFloat(val) || 0;
  };

  const startSec = parseToSeconds(startRaw);
  const endSec = parseToSeconds(endRaw);

  const duration = endSec > startSec ? endSec - startSec : 5;
  const safeDuration = Math.min(duration, 10);

  return { startTime: startSec, duration: safeDuration };
}

/**
 * Detects whether a WebP buffer is animated by checking for the ANIM chunk.
 * More reliable than trusting Baileys' stickerMessage.isAnimated flag, which
 * is set by the sending client and is frequently wrong/missing for stickers
 * that have been forwarded or re-sent through other apps.
 */
function isAnimatedWebpBuffer(buffer) {
  try {
    // RIFF header is 12 bytes, chunks follow as 4-byte FourCC + size
    // Just search for the "ANIM" FourCC anywhere in the first ~64 bytes of chunk headers
    return buffer.includes("ANIM", 0, "ascii");
  } catch (err) {
    console.error("[sticker] isAnimatedWebpBuffer check failed:", err);
    return false;
  }
}

/**
 * Safe media downloader wrapper for Baileys
 */
async function downloadMediaSafely(msg, isQuoted, mediaType) {
  try {
    let buffer;
    if (isQuoted) {
      const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
      const targetMsg = {
        key: {
          remoteJid: msg.key.remoteJid,
          id: contextInfo?.stanzaId,
          participant: contextInfo?.participant || msg.key.participant || msg.key.remoteJid,
          fromMe: false,
        },
        message: contextInfo?.quotedMessage,
      };
      buffer = await downloadMediaMessage(targetMsg, "buffer", {});
    } else {
      buffer = await downloadMediaMessage(msg, "buffer", {});
    }

    console.log(`[sticker] downloadMediaSafely: downloaded ${buffer?.length ?? 0} bytes (type=${mediaType}, quoted=${isQuoted})`);

    if (!buffer || buffer.length === 0) {
      throw new Error("Downloaded media buffer is empty.");
    }

    return buffer;
  } catch (err) {
    console.error(`[sticker] Media download failed (type=${mediaType}, quoted=${isQuoted}):`, err);
    throw new Error("Could not download media file from WhatsApp.");
  }
}

/**
 * Creates a static or animated WebP sticker from an image or video buffer
 */
async function convertToSticker(buffer, isVideo = false, timeRange = null) {
  const ext = isVideo ? ".mp4" : ".jpg";
  const tempInput = path.join(tmpdir(), `input_${Date.now()}${ext}`);
  const tempOutput = path.join(tmpdir(), `output_${Date.now()}.webp`);
  await fs.writeFile(tempInput, buffer);

  const vf = isVideo
    ? `scale='min(${STICKER_SIZE},iw)':'min(${STICKER_SIZE},ih)':force_original_aspect_ratio=decrease,fps=15,pad=${STICKER_SIZE}:${STICKER_SIZE}:(${STICKER_SIZE}-iw)/2:(${STICKER_SIZE}-ih)/2:color=00000000`
    : `scale='min(${STICKER_SIZE},iw)':'min(${STICKER_SIZE},ih)':force_original_aspect_ratio=decrease,pad=${STICKER_SIZE}:${STICKER_SIZE}:(${STICKER_SIZE}-iw)/2:(${STICKER_SIZE}-ih)/2:color=00000000`;

  return new Promise((resolve, reject) => {
    let f = ffmpeg(tempInput);

    if (isVideo) {
      const { startTime, duration } = parseTimeRange(timeRange);
      f.inputOptions([`-ss ${startTime}`, `-t ${duration}`]);
    }

    let stderrLog = "";

    f.outputOptions([
      "-vcodec", "libwebp",
      "-vf", vf,
      "-loop", "0",
      "-pix_fmt", "rgba",
      "-an",
      "-vsync", "0"
    ])
      .toFormat("webp")
      .on("stderr", (line) => {
        stderrLog += line + "\n";
      })
      .on("end", async () => {
        try {
          const outputBuffer = await fs.readFile(tempOutput);
          await Promise.all([fs.unlink(tempInput).catch(() => {}), fs.unlink(tempOutput).catch(() => {})]);
          resolve(outputBuffer);
        } catch (e) {
          console.error("[sticker] convertToSticker: failed reading output file:", e);
          reject(e);
        }
      })
      .on("error", async (err) => {
        console.error("[sticker] convertToSticker ffmpeg error:", err.message);
        console.error("[sticker] convertToSticker ffmpeg stderr:\n", stderrLog);
        await Promise.all([fs.unlink(tempInput).catch(() => {}), fs.unlink(tempOutput).catch(() => {})]);
        reject(err);
      })
      .save(tempOutput);
  });
}

/**
 * Converts static sticker buffer directly to PNG using Sharp
 */
async function convertStickerToImage(buffer) {
  if (!buffer || buffer.length === 0) {
    throw new Error("Sticker buffer is empty — media download likely failed silently.");
  }
  console.log(`[sticker] convertStickerToImage: input buffer size = ${buffer.length} bytes`);

  try {
    return await sharp(buffer).png().toBuffer();
  } catch (err) {
    console.error("[sticker] convertStickerToImage (sharp) error:", err);
    throw err;
  }
}

/**
 * Converts WebP sticker buffer (animated) to MP4 video.
 *
 * IMPORTANT: FFmpeg's native "webp" decoder (even with --enable-libwebp,
 * which only adds the libwebp ENCODER) cannot parse the ANIM/ANMF chunks
 * used by animated WebP files — it only understands single-frame WebP.
 * Feeding an animated webp straight into ffmpeg fails with errors like
 * "skipping unsupported chunk: ANIM" / "image data not found", which is
 * exactly what was happening here.
 *
 * Workaround: use sharp (libvips), which CAN decode animated webp frames,
 * to first re-encode the animated webp as an animated GIF. FFmpeg's GIF
 * decoder is fully supported, so we then hand that GIF to ffmpeg to make
 * the final MP4.
 */
async function convertStickerToGif(buffer) {
  if (!buffer || buffer.length === 0) {
    throw new Error("Sticker buffer is empty — media download likely failed silently.");
  }
  console.log(`[sticker] convertStickerToGif: input buffer size = ${buffer.length} bytes`);

  // Step 1: animated webp -> animated gif, via sharp/libvips (ffmpeg can't do this part)
  let gifBuffer;
  try {
    gifBuffer = await sharp(buffer, { animated: true })
      .gif()
      .toBuffer();
    console.log(`[sticker] convertStickerToGif: sharp produced intermediate gif, ${gifBuffer.length} bytes`);
  } catch (err) {
    console.error("[sticker] convertStickerToGif: sharp failed to decode animated webp:", err);
    throw new Error(
      "Could not read this animated sticker (unsupported WebP variant). " +
      "Make sure your 'sharp' package version supports GIF output (sharp >= 0.32)."
    );
  }

  // Step 2: animated gif -> mp4, via ffmpeg (fully supported path)
  const tempInput = path.join(tmpdir(), `animated_${Date.now()}.gif`);
  const tempOutput = path.join(tmpdir(), `gif_${Date.now()}.mp4`);
  await fs.writeFile(tempInput, gifBuffer);

  return new Promise((resolve, reject) => {
    let stderrLog = "";

    ffmpeg(tempInput)
      .outputOptions([
        "-movflags faststart",
        "-pix_fmt yuv420p",
        "-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2,fps=15",
        "-preset ultrafast"
      ])
      .toFormat("mp4")
      .on("stderr", (line) => {
        stderrLog += line + "\n";
      })
      .on("end", async () => {
        try {
          const outputBuffer = await fs.readFile(tempOutput);
          await Promise.all([
            fs.unlink(tempInput).catch(() => {}),
            fs.unlink(tempOutput).catch(() => {})
          ]);
          resolve(outputBuffer);
        } catch (readErr) {
          console.error("[sticker] convertStickerToGif: failed reading output file:", readErr);
          reject(readErr);
        }
      })
      .on("error", async (err) => {
        console.error("[sticker] convertStickerToGif ffmpeg error:", err.message);
        console.error("[sticker] convertStickerToGif ffmpeg stderr:\n", stderrLog);
        await Promise.all([
          fs.unlink(tempInput).catch(() => {}),
          fs.unlink(tempOutput).catch(() => {})
        ]);
        reject(err);
      })
      .save(tempOutput);
  });
}

/**
 * Extracts the audio track from a video buffer as an MP3 file.
 */
async function extractAudioFromVideo(buffer) {
  if (!buffer || buffer.length === 0) {
    throw new Error("Video buffer is empty — media download likely failed silently.");
  }
  console.log(`[sticker] extractAudioFromVideo: input buffer size = ${buffer.length} bytes`);

  const tempInput = path.join(tmpdir(), `toaudio_in_${Date.now()}.mp4`);
  const tempOutput = path.join(tmpdir(), `toaudio_out_${Date.now()}.mp3`);
  await fs.writeFile(tempInput, buffer);

  return new Promise((resolve, reject) => {
    let stderrLog = "";

    ffmpeg(tempInput)
      .outputOptions([
        "-vn",           // drop video stream entirely
        "-acodec", "libmp3lame",
        "-b:a", "192k",
        "-ar", "44100",
        "-ac", "2"
      ])
      .toFormat("mp3")
      .on("stderr", (line) => {
        stderrLog += line + "\n";
      })
      .on("end", async () => {
        try {
          const outputBuffer = await fs.readFile(tempOutput);
          await Promise.all([
            fs.unlink(tempInput).catch(() => {}),
            fs.unlink(tempOutput).catch(() => {})
          ]);
          resolve(outputBuffer);
        } catch (readErr) {
          console.error("[sticker] extractAudioFromVideo: failed reading output file:", readErr);
          reject(readErr);
        }
      })
      .on("error", async (err) => {
        console.error("[sticker] extractAudioFromVideo ffmpeg error:", err.message);
        console.error("[sticker] extractAudioFromVideo ffmpeg stderr:\n", stderrLog);
        await Promise.all([
          fs.unlink(tempInput).catch(() => {}),
          fs.unlink(tempOutput).catch(() => {})
        ]);
        // A video with no audio track at all lands here too — give a clearer message.
        if (/does not contain any stream|Output file does not contain any stream/i.test(stderrLog)) {
          reject(new Error("This video has no audio track to extract."));
        } else {
          reject(err);
        }
      })
      .save(tempOutput);
  });
}

export async function handleStickerCommand(sock, jid, rawText, msg) {
  const safeText = rawText || "";
  const args = safeText.trim().split(/\s+/);
  const command = (args[0] || "").toLowerCase();

  const STICKER_CMDS = ["!s", "!sticker"];
  const TOIMG_CMDS = ["!toimg"];
  const TOGIF_CMDS = ["!togif", "!tovideo"];
  const TOAUDIO_CMDS = ["!toaudio", "!tomp3"];

  if (![...STICKER_CMDS, ...TOIMG_CMDS, ...TOGIF_CMDS, ...TOAUDIO_CMDS].includes(command)) {
    return false;
  }

  await sendTypingStatus(sock, jid);

  try {
    const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
    const quotedMsg = contextInfo?.quotedMessage;
    const isQuoted = !!quotedMsg;

    // 1. Sticker Creation (!s / !sticker [start-end])
    if (STICKER_CMDS.includes(command)) {
      const imgMsg = msg.message?.imageMessage || quotedMsg?.imageMessage;
      const vidMsg = msg.message?.videoMessage || quotedMsg?.videoMessage;

      if (!imgMsg && !vidMsg) {
        await sock.sendMessage(jid, { text: "⚠️ Reply to or send an image/video with `!sticker` or `!s`.\n\n*Video clip range example:* `!s 3-7` (seconds 3 to 7)" }, { quoted: msg });
        return true;
      }

      const timeRangeArg = args[1] || null;
      const buffer = await downloadMediaSafely(msg, isQuoted, imgMsg ? "imageMessage" : "videoMessage");
      const stickerBuffer = await convertToSticker(buffer, !!vidMsg, timeRangeArg);

      await sock.sendMessage(jid, { sticker: stickerBuffer }, { quoted: msg });
      return true;
    }

    // 2. Sticker to Image / GIF (!toimg)
    if (TOIMG_CMDS.includes(command)) {
      const stkMsg = msg.message?.stickerMessage || quotedMsg?.stickerMessage;

      if (!stkMsg) {
        await sock.sendMessage(jid, { text: "⚠️ Reply to a sticker with `!toimg`." }, { quoted: msg });
        return true;
      }

      const buffer = await downloadMediaSafely(msg, isQuoted, "stickerMessage");

      // Sniff the buffer instead of trusting stkMsg.isAnimated, which is
      // set by the sending client and can be wrong for forwarded stickers.
      const isAnimated = isAnimatedWebpBuffer(buffer);

      if (isAnimated) {
        const gifBuffer = await convertStickerToGif(buffer);
        await sock.sendMessage(
          jid,
          { video: gifBuffer, caption: "✅ Animated Sticker converted to GIF", gifPlayback: true },
          { quoted: msg }
        );
      } else {
        const imageBuffer = await convertStickerToImage(buffer);
        await sock.sendMessage(
          jid,
          { image: imageBuffer, caption: "✅ Sticker converted to Image" },
          { quoted: msg }
        );
      }
      return true;
    }

    // 3. Explicit Sticker to GIF (!togif / !tovideo)
    if (TOGIF_CMDS.includes(command)) {
      const stkMsg = msg.message?.stickerMessage || quotedMsg?.stickerMessage;

      if (!stkMsg) {
        await sock.sendMessage(jid, { text: "⚠️ Reply to any sticker with `!togif` or `!tovideo`." }, { quoted: msg });
        return true;
      }

      const buffer = await downloadMediaSafely(msg, isQuoted, "stickerMessage");
      const gifBuffer = await convertStickerToGif(buffer);

      await sock.sendMessage(
        jid,
        { video: gifBuffer, caption: "✅ Sticker converted to Video/GIF", gifPlayback: true },
        { quoted: msg }
      );
      return true;
    }

    // 4. Extract Audio from Video (!toaudio / !tomp3)
    if (TOAUDIO_CMDS.includes(command)) {
      const vidMsg = msg.message?.videoMessage || quotedMsg?.videoMessage;
      const ptvMsg = msg.message?.ptvMessage || quotedMsg?.ptvMessage; // "video note" circle videos
      const audioMsg = msg.message?.audioMessage || quotedMsg?.audioMessage;

      if (!vidMsg && !ptvMsg && !audioMsg) {
        await sock.sendMessage(jid, { text: "⚠️ Reply to or send a video with `!toaudio` (or `!tomp3`) to extract its audio." }, { quoted: msg });
        return true;
      }

      const buffer = await downloadMediaSafely(
        msg,
        isQuoted,
        vidMsg ? "videoMessage" : ptvMsg ? "ptvMessage" : "audioMessage"
      );

      // Already an audio message — nothing to extract, just re-send as mp3-labelled audio
      if (audioMsg && !vidMsg && !ptvMsg) {
        await sock.sendMessage(
          jid,
          { audio: buffer, mimetype: "audio/mp4", ptt: false },
          { quoted: msg }
        );
        return true;
      }

      const audioBuffer = await extractAudioFromVideo(buffer);

      await sock.sendMessage(
        jid,
        { audio: audioBuffer, mimetype: "audio/mpeg", ptt: false, fileName: "audio.mp3" },
        { quoted: msg }
      );
      return true;
    }
  } catch (err) {
    console.error("[sticker] Sticker Command Error:", err);
    await sock.sendMessage(jid, { text: `❌ Failed to process media: ${err.message}` }, { quoted: msg });
  } finally {
    await sendPausedStatus(sock, jid);
  }

  return true;
}