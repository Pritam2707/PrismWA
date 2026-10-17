import { readFile, unlink } from "node:fs/promises";
import { existsSync } from "node:fs";
import ffmpeg from "fluent-ffmpeg";
import path from "node:path";
import { tmpdir } from "node:os";

/**
 * Loads a local media file and converts it into a WhatsApp-compatible WebP sticker using fluent-ffmpeg.
 */
export default async function getLocalStickerBuffer(filePath) {
  try {
    if (!filePath || !existsSync(filePath)) {
      console.warn(`Local sticker file not found: ${filePath}`);
      return null;
    }

    const tempOutput = path.join(tmpdir(), `sticker_${Date.now()}_${Math.random().toString(36).substring(7)}.webp`);

    return new Promise((resolve) => {
      ffmpeg(filePath)
        .inputOptions(["-ss 00:00:00"])
        .addOutputOptions([
          "-vcodec libwebp",
          "-vf scale='min(512\\,iw):min(512\\,ih)':force_original_aspect_ratio=decrease,fps=15, pad=512:512:(512-iw)/2:(512-ih)/2:color=00000000",
          "-loop 0",
          "-pix_fmt rgba",
          "-an",
          "-vsync 0",
        ])
        .toFormat("webp")
        .on("end", async () => {
          try {
            const buffer = await readFile(tempOutput);
            await unlink(tempOutput).catch(() => {}); // Cleanup temp file
            resolve(buffer);
          } catch (readErr) {
            console.error("Failed to read processed sticker buffer:", readErr);
            resolve(null);
          }
        })
        .on("error", async (err) => {
          console.error("FFmpeg sticker conversion error:", err.message);
          await unlink(tempOutput).catch(() => {});
          resolve(null);
        })
        .save(tempOutput);
    });
  } catch (e) {
    console.error("Failed to load local sticker:", e);
    return null;
  }
}