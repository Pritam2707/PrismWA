import fs from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";

const DATA_DIR = path.join(process.cwd(), "data");
const MEDIA_DIR = path.join(DATA_DIR, "media");
const STICKERS_DIR = path.join(process.cwd(), "stickers");

const STORE_FILES = {
  deleted_msgs: path.join(DATA_DIR, "deleted_messages.json"),
  deleted_vo: path.join(DATA_DIR, "deleted_viewonce.json"),
  dynamic_store: path.join(DATA_DIR, "dynamic_store.json"),
  welcome_settings: path.join(DATA_DIR, "welcome_settings.json"),
  chat_stats: path.join(DATA_DIR, "chat_stats.json"),
  notes: path.join(DATA_DIR, "notes.json"),
  muted_users: path.join(DATA_DIR, "muted_users.json"),
};

// ---------------------------------------------------------------------------
// HELPER FUNCTIONS (Must be defined above or present in file)
// ---------------------------------------------------------------------------

/**
 * Calculates total size of a directory recursively
 */
async function getDirSize(dirPath) {
  if (!existsSync(dirPath)) return 0;
  let totalSize = 0;

  try {
    const files = await fs.readdir(dirPath, { withFileTypes: true });
    for (const file of files) {
      const fullPath = path.join(dirPath, file.name);
      if (file.isDirectory()) {
        totalSize += await getDirSize(fullPath);
      } else {
        const stats = await fs.stat(fullPath);
        totalSize += stats.size;
      }
    }
  } catch (err) {
    console.error(`Failed to calculate directory size for ${dirPath}:`, err.message);
  }

  return totalSize;
}

/**
 * Gets human-readable file size string
 */
function formatBytes(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

/**
 * Safely measures single file size
 */
async function getFileSize(filePath) {
  if (!existsSync(filePath)) return 0;
  try {
    const stats = await fs.stat(filePath);
    return stats.size;
  } catch {
    return 0;
  }
}

/**
 * Deletes all files in a directory
 */
async function clearDirectory(dirPath) {
  if (!existsSync(dirPath)) return 0;
  let count = 0;
  const files = await fs.readdir(dirPath);
  for (const file of files) {
    await fs.unlink(path.join(dirPath, file)).catch(() => {});
    count++;
  }
  return count;
}

// ---------------------------------------------------------------------------
// MAIN COMMAND HANDLER
// ---------------------------------------------------------------------------

export async function handleStorageCommand(sock, jid, senderId, rawText, msg) {
  // 🔒 OWNER SECURITY: Only execute if sent from your own account
  if (!msg.key.fromMe) return false;

  const args = rawText.trim().split(/\s+/);
  const command = args[0]?.toLowerCase();

  if (!["!storage", "!cleanstorage"].includes(command)) {
    return false;
  }

  const subCommand = args[1]?.toLowerCase();

  // 1. Storage Status Summary (`!storage`)
  if (command === "!storage" && !subCommand) {
    const [
      mediaSize,
      stickersSize,
      deletedMsgsSize,
      deletedVoSize,
      dynamicStoreSize,
      statsSize,
      notesSize,
    ] = await Promise.all([
      getDirSize(MEDIA_DIR),
      getDirSize(STICKERS_DIR),
      getFileSize(STORE_FILES.deleted_msgs),
      getFileSize(STORE_FILES.deleted_vo),
      getFileSize(STORE_FILES.dynamic_store),
      getFileSize(STORE_FILES.chat_stats),
      getFileSize(STORE_FILES.notes),
    ]);

    const totalCalculated =
      mediaSize + stickersSize + deletedMsgsSize + deletedVoSize + dynamicStoreSize + statsSize + notesSize;

    let statusText = `╭━━━ 💾 *STORAGE MONITOR* ━━━╮\n\n`;
    statusText += `📁 *Saved Media Files:* \`${formatBytes(mediaSize)}\`\n`;
    statusText += `🎨 *Persona Stickers:* \`${formatBytes(stickersSize)}\`\n`;
    statusText += `💬 *Deleted Messages DB:* \`${formatBytes(deletedMsgsSize)}\`\n`;
    statusText += `👁️ *View Once DB:* \`${formatBytes(deletedVoSize)}\`\n`;
    statusText += `🛠️ *Dynamic Store DB:* \`${formatBytes(dynamicStoreSize)}\`\n`;
    statusText += `📊 *Chat Statistics DB:* \`${formatBytes(statsSize)}\`\n`;
    statusText += `📝 *User Notes DB:* \`${formatBytes(notesSize)}\`\n`;
    statusText += `─────────────────────────\n`;
    statusText += `📦 *Total Allocated:* \`${formatBytes(totalCalculated)}\`\n\n`;
    statusText += `👉 *Clear Commands:*\n`;
    statusText += `• \`!storage clear media\` (Deletes saved media)\n`;
    statusText += `• \`!storage clear deleted\` (Cleans deleted msg JSONs)\n`;
    statusText += `• \`!storage clear stickers\` (Cleans dynamic stickers)\n`;
    statusText += `• \`!storage clear all\` (Wipes all stored cache & media)`;

    await sock.sendMessage(jid, { text: statusText }, { quoted: msg });
    return true;
  }

  // 2. Storage Cleanup Subcommands (`!storage clear <target>`)
  if (subCommand === "clear" || subCommand === "clean") {
    const target = args[2]?.toLowerCase();

    if (!target) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ Usage: `!storage clear <media | deleted | stickers | all>`" },
        { quoted: msg }
      );
      return true;
    }

    if (target === "media") {
      const count = await clearDirectory(MEDIA_DIR);
      await sock.sendMessage(jid, { text: `🧹 *Successfully cleared ${count} local media file(s)!*` }, { quoted: msg });
      return true;
    }

    if (target === "deleted") {
      await clearDirectory(MEDIA_DIR);
      await fs.writeFile(STORE_FILES.deleted_msgs, "{}", "utf-8");
      await fs.writeFile(STORE_FILES.deleted_vo, "{}", "utf-8");
      await sock.sendMessage(jid, { text: "🧹 *Cleared all deleted message logs and associated media caches!*" }, { quoted: msg });
      return true;
    }

    if (target === "stickers") {
      const count = await clearDirectory(STICKERS_DIR);
      await sock.sendMessage(jid, { text: `🎨 *Successfully deleted ${count} custom sticker file(s)!*` }, { quoted: msg });
      return true;
    }

    if (target === "all") {
      const mediaCount = await clearDirectory(MEDIA_DIR);
      await fs.writeFile(STORE_FILES.deleted_msgs, "{}", "utf-8");
      await fs.writeFile(STORE_FILES.deleted_vo, "{}", "utf-8");
      await fs.writeFile(STORE_FILES.chat_stats, "{}", "utf-8");

      await sock.sendMessage(
        jid,
        {
          text: `💥 *Complete Storage Cleanup Executed!*\n\n• Cleared \`${mediaCount}\` media files\n• Reset deleted message databases\n• Reset chat stats log`,
        },
        { quoted: msg }
      );
      return true;
    }

    await sock.sendMessage(jid, { text: "⚠️ Unknown target. Options: `media`, `deleted`, `stickers`, `all`" }, { quoted: msg });
    return true;
  }

  return true;
}