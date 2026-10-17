import { downloadMediaMessage } from "baileys";
import fs from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";

const NOTES_FILE = path.join(process.cwd(), "notes.json");

// Active working state per chat
const userCurrentPaths = new Map();
const lastDashboardMsgKey = new Map();

/**
 * Load notes database
 */
async function loadNotes() {
  if (!existsSync(NOTES_FILE)) return {};
  try {
    const data = await fs.readFile(NOTES_FILE, "utf-8");
    return JSON.parse(data);
  } catch {
    return {};
  }
}

/**
 * Save notes database
 */
async function saveNotes(notes) {
  await fs.writeFile(NOTES_FILE, JSON.stringify(notes, null, 2));
}

/**
 * Helper: Check if command executor has permission (Owner/Host only)
 */
function isAuthorizedOwner(sock, msg) {
  if (msg?.key?.fromMe) return true;

  const botJid = sock.user?.id ? sock.user.id.split(":")[0] + "@s.whatsapp.net" : null;
  const senderJid = msg?.key?.participant || msg?.key?.remoteJid;

  return !!(botJid && senderJid && (senderJid === botJid || senderJid.startsWith(botJid.split("@")[0])));
}

/**
 * Normalizes input path based on active working directory (`currentPath`)
 */
function resolveFullPath(rawPath, currentPath) {
  let cleaned = rawPath.replace(/^\/+|\/+$/g, "").replace(/\/{2,}/g, "/");

  if (cleaned.startsWith("root/")) {
    return cleaned;
  }

  if (cleaned.startsWith("public/") || cleaned.startsWith("private/")) {
    return `root/${cleaned}`;
  }

  if (currentPath.startsWith("root/private")) {
    return `${currentPath}${cleaned}`.replace(/\/{2,}/g, "/");
  }

  if (currentPath === "root/") {
    return `root/public/${cleaned}`;
  }

  return `${currentPath}${cleaned}`.replace(/\/{2,}/g, "/");
}

/**
 * Helper to download attached/quoted media safely
 */
async function downloadMediaSafely(msg) {
  const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
  const quotedMsg = contextInfo?.quotedMessage;

  if (quotedMsg) {
    const stanzaId = contextInfo.stanzaId;
    const participant = contextInfo.participant || msg.key.participant || msg.key.remoteJid;

    const downloadTarget = {
      key: {
        remoteJid: msg.key.remoteJid,
        id: stanzaId,
        participant: participant,
        fromMe: false,
      },
      message: quotedMsg,
    };

    return await downloadMediaMessage(downloadTarget, "buffer", {});
  }

  return await downloadMediaMessage(msg, "buffer", {});
}

/**
 * Detects media and document types (including Spreadsheets, PPT, PDF, Word)
 */
function detectMediaType(msg) {
  const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
  const m = contextInfo?.quotedMessage || msg.message;

  if (!m) return { type: "TXT", mimetype: "text/plain", fileName: null };

  if (m.imageMessage) return { type: "IMA", mimetype: m.imageMessage.mimetype || "image/jpeg", fileName: null };
  if (m.videoMessage) return { type: "VID", mimetype: m.videoMessage.mimetype || "video/mp4", fileName: null };
  if (m.audioMessage) return { type: "AUD", mimetype: m.audioMessage.mimetype || "audio/mp4", fileName: null };
  if (m.stickerMessage) return { type: "STK", mimetype: m.stickerMessage.mimetype || "image/webp", fileName: null };

  if (m.documentMessage) {
    const fileName = m.documentMessage.fileName || "document";
    const ext = path.extname(fileName).toLowerCase();
    const mimetype = m.documentMessage.mimetype || "application/octet-stream";

    let type = "DOC";
    if ([".xls", ".xlsx", ".csv"].includes(ext) || mimetype.includes("excel") || mimetype.includes("spreadsheetml") || mimetype.includes("csv")) {
      type = "XLS";
    } else if ([".ppt", ".pptx"].includes(ext) || mimetype.includes("powerpoint") || mimetype.includes("presentationml")) {
      type = "PPT";
    } else if (ext === ".pdf" || mimetype.includes("pdf")) {
      type = "PDF";
    } else if ([".doc", ".docx"].includes(ext) || mimetype.includes("word") || mimetype.includes("wordprocessingml")) {
      type = "DOC";
    }

    return { type, mimetype, fileName };
  }

  return { type: "TXT", mimetype: "text/plain", fileName: null };
}

/**
 * Filters notes based on accessibility rules
 */
function getAccessibleNotes(notes, currentJid) {
  const accessible = {};
  for (const [notePath, noteData] of Object.entries(notes)) {
    if (notePath.startsWith("root/public/")) {
      accessible[notePath] = noteData;
    } else if (notePath.startsWith("root/private/")) {
      if (!noteData.scopeJid || noteData.scopeJid === currentJid) {
        accessible[notePath] = noteData;
      }
    }
  }
  return accessible;
}

/**
 * Builds nested tree hierarchy and returns indexed items for navigation
 */
function buildTreeAndIndexMap(currentPath, notes, currentJid) {
  const accessible = getAccessibleNotes(notes, currentJid);
  const normalizedCurrent = currentPath.endsWith("/") ? currentPath : `${currentPath}/`;

  let outputLines = [];
  let itemIndexMap = []; // Stores { num, name, isFolder, fullPath, relativePath }

  let counter = 1;

  // Root Virtual Nodes
  if (normalizedCurrent === "root/") {
    outputLines.push(`├── 📂 [ 1 ] public/`);
    itemIndexMap.push({ num: 1, name: "public", isFolder: true, fullPath: "root/public/" });

    outputLines.push(`└── 📂 [ 2 ] private/`);
    itemIndexMap.push({ num: 2, name: "private", isFolder: true, fullPath: "root/private/" });

    return { treeText: outputLines.join("\n"), itemIndexMap };
  }

  // Filter notes belonging to current subtree
  const relevantPaths = Object.keys(accessible).filter((p) => p.startsWith(normalizedCurrent));

  if (relevantPaths.length === 0) {
    return { treeText: "( empty directory )", itemIndexMap: [] };
  }

  // Group paths into a recursive tree structure
  const treeObj = {};

  for (const fullNotePath of relevantPaths) {
    const rel = fullNotePath.slice(normalizedCurrent.length);
    const parts = rel.split("/");

    let currentLevel = treeObj;
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      const isLast = i === parts.length - 1;

      if (isLast) {
        currentLevel[part] = { _fileData: accessible[fullNotePath], _fullPath: fullNotePath };
      } else {
        if (!currentLevel[part]) currentLevel[part] = {};
        currentLevel = currentLevel[part];
      }
    }
  }

  // Recursive Tree Formatter
  function traverse(node, prefix = "", currentFullPath = normalizedCurrent) {
    const keys = Object.keys(node).sort((a, b) => {
      const aIsFile = !!node[a]._fileData;
      const bIsFile = !!node[b]._fileData;
      if (aIsFile === bIsFile) return a.localeCompare(b);
      return aIsFile ? 1 : -1;
    });

    keys.forEach((key, index) => {
      const isLast = index === keys.length - 1;
      const connector = isLast ? "└── " : "├── ";
      const childPrefix = prefix + (isLast ? "    " : "│   ");

      const childNode = node[key];
      const num = counter++;

      if (childNode._fileData) {
        // File Node
        const mediaType = childNode._fileData.mediaType || "TXT";
        outputLines.push(`${prefix}${connector}📄 [ ${num} ] ${key} (${mediaType})`);
        itemIndexMap.push({ num, name: key, isFolder: false, fullPath: childNode._fullPath });
      } else {
        // Subfolder Node
        const folderFullPath = `${currentFullPath}${key}/`;
        outputLines.push(`${prefix}${connector}📂 [ ${num} ] ${key}/`);
        itemIndexMap.push({ num, name: key, isFolder: true, fullPath: folderFullPath });
        traverse(childNode, childPrefix, folderFullPath);
      }
    });
  }

  traverse(treeObj, "", normalizedCurrent);

  return { treeText: outputLines.join("\n"), itemIndexMap };
}

/**
 * Renders nested tree dashboard
 */
function renderNestedDashboard(currentPath, notes, currentJid) {
  const { treeText, itemIndexMap } = buildTreeAndIndexMap(currentPath, notes, currentJid);

  let output = `📁 *${currentPath}*\n==================================\n`;
  output += treeText + "\n";
  output += `==================================\n`;
  output += `👉 *Open:* \`!cd <num|name>\` | *Back:* \`!cd ..\`\n`;
  output += `👉 *Get file:* \`!getnote <num|path>\`\n`;
  output += `❌ *Delete (Owner Only):* \`!delnote <num|path>\``;

  return { viewText: output, itemIndexMap };
}

/**
 * Sends brand new dashboard frame
 */
async function sendNewDashboard(sock, jid, textContent, msg) {
  const sent = await sock.sendMessage(jid, { text: textContent }, { quoted: msg });
  if (sent?.key) {
    lastDashboardMsgKey.set(jid, sent.key);
  }
}

/**
 * Edits active dashboard frame
 */
async function updateDashboardGUI(sock, jid, textContent, msg) {
  const previousKey = lastDashboardMsgKey.get(jid);

  if (previousKey) {
    try {
      await sock.sendMessage(jid, {
        text: textContent,
        edit: previousKey,
      });
      return;
    } catch {
      // Fallback to sending a new message if editing fails
    }
  }

  await sendNewDashboard(sock, jid, textContent, msg);
}

/**
 * Main Notes Command Handler
 */
export async function handleNotesCommand(sock, jid, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();

  const notes = await loadNotes();
  let currentPath = userCurrentPaths.get(jid) || "root/";

  // 1. !notes / !ls — Generate brand new nested dashboard
  if (command === "!notes" || command === "!ls") {
    currentPath = "root/";
    userCurrentPaths.set(jid, currentPath);

    const { viewText } = renderNestedDashboard(currentPath, notes, jid);
    await sendNewDashboard(sock, jid, viewText, msg);
    return true;
  }

  // 2. !cd <num | name | ..> — Change Directory
  if (command === "!cd") {
    const target = args[1];

    if (!target) {
      await updateDashboardGUI(sock, jid, "⚠️ Usage: `!cd <number | folder_name | ..>`", msg);
      return true;
    }

    if (target === "..") {
      if (currentPath === "root/") {
        const { viewText } = renderNestedDashboard("root/", notes, jid);
        await updateDashboardGUI(sock, jid, viewText + "\n\n⚠️ *Already at root/*", msg);
        return true;
      }

      const parts = currentPath.replace(/\/$/, "").split("/");
      parts.pop();
      currentPath = parts.join("/") + "/";
      if (!currentPath.startsWith("root/")) currentPath = "root/";

      userCurrentPaths.set(jid, currentPath);
      const { viewText } = renderNestedDashboard(currentPath, notes, jid);
      await updateDashboardGUI(sock, jid, viewText, msg);
      return true;
    }

    const { itemIndexMap } = buildTreeAndIndexMap(currentPath, notes, jid);
    const numIndex = parseInt(target, 10);

    let selectedChild = null;

    if (!isNaN(numIndex)) {
      selectedChild = itemIndexMap.find((item) => item.num === numIndex);
    } else {
      selectedChild = itemIndexMap.find((item) => item.name.toLowerCase() === target.toLowerCase());
    }

    if (!selectedChild) {
      const { viewText } = renderNestedDashboard(currentPath, notes, jid);
      await updateDashboardGUI(sock, jid, viewText + `\n\n❌ *Item "${target}" not found!*`, msg);
      return true;
    }

    if (!selectedChild.isFolder) {
      const { viewText } = renderNestedDashboard(currentPath, notes, jid);
      await updateDashboardGUI(sock, jid, viewText + `\n\n⚠️ *"${selectedChild.name}" is a file. Use !getnote ${selectedChild.name}*`, msg);
      return true;
    }

    currentPath = selectedChild.fullPath;
    userCurrentPaths.set(jid, currentPath);

    const { viewText } = renderNestedDashboard(currentPath, notes, jid);
    await updateDashboardGUI(sock, jid, viewText, msg);
    return true;
  }

  // 3. !addnote <path> [content]
  if (command === "!addnote") {
    const notePathRaw = args[1];
    let contentText = args.slice(2).join(" ").trim();

    if (!notePathRaw) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ Usage:\n• `!addnote <path> <text>`\n• `!addnote private/<path>` (Private note to this chat)" },
        { quoted: msg }
      );
      return true;
    }

    const fullPath = resolveFullPath(notePathRaw, currentPath);
    const isPrivate = fullPath.startsWith("root/private/");
    const mediaInfo = detectMediaType(msg);

    const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
    const quotedText = contextInfo?.quotedMessage?.conversation || contextInfo?.quotedMessage?.extendedTextMessage?.text;

    if (!contentText && quotedText) {
      contentText = quotedText;
    }

    let base64Media = null;

    if (mediaInfo.type !== "TXT") {
      try {
        const buffer = await downloadMediaSafely(msg);
        base64Media = buffer.toString("base64");
      } catch (err) {
        console.error("Failed to download note media:", err);
      }
    }

    notes[fullPath] = {
      content: contentText || "",
      mediaType: mediaInfo.type,
      mimetype: mediaInfo.mimetype,
      fileName: mediaInfo.fileName || path.basename(fullPath),
      mediaData: base64Media,
      isPrivate: isPrivate,
      scopeJid: jid,
      timestamp: new Date().toISOString(),
    };

    await saveNotes(notes);

    const { viewText } = renderNestedDashboard(currentPath, notes, jid);
    await updateDashboardGUI(sock, jid, viewText + `\n\n✅ *Note saved:* \`${fullPath}\``, msg);
    return true;
  }

  // 4. !getnote <num | name | fullPath>
  if (command === "!getnote" || command === "!getnotes") {
    const target = args[1];

    if (!target) {
      await sock.sendMessage(jid, { text: "⚠️ Usage: `!getnote <num | name | path>`" }, { quoted: msg });
      return true;
    }

    let targetPath = null;
    const numIndex = parseInt(target, 10);
    const accessibleNotes = getAccessibleNotes(notes, jid);

    if (!isNaN(numIndex)) {
      const { itemIndexMap } = buildTreeAndIndexMap(currentPath, notes, jid);
      const selected = itemIndexMap.find((item) => item.num === numIndex);
      if (selected && !selected.isFolder) {
        targetPath = selected.fullPath;
      }
    }

    if (!targetPath) {
      const possibleRelative = `${currentPath}${target}`;
      if (accessibleNotes[possibleRelative]) {
        targetPath = possibleRelative;
      }
    }

    if (!targetPath) {
      const resolved = resolveFullPath(target, currentPath);
      if (accessibleNotes[resolved]) {
        targetPath = resolved;
      }
    }

    if (!targetPath || !accessibleNotes[targetPath]) {
      await sock.sendMessage(jid, { text: `❌ Note \`${target}\` not found or permission denied.` }, { quoted: msg });
      return true;
    }

    const note = accessibleNotes[targetPath];
    const caption = `📄 *Note:* \`${targetPath}\`\n🏷️ *Type:* \`${note.mediaType}\`\n📅 *Saved:* ${new Date(note.timestamp).toLocaleString()}\n\n${note.content}`.trim();

    if (note.mediaType === "TXT" || !note.mediaData) {
      await sock.sendMessage(jid, { text: caption }, { quoted: msg });
    } else {
      const buffer = Buffer.from(note.mediaData, "base64");
      const fallbackName = note.fileName || path.basename(targetPath);

      switch (note.mediaType) {
        case "IMA":
          await sock.sendMessage(jid, { image: buffer, caption }, { quoted: msg });
          break;
        case "VID":
          await sock.sendMessage(jid, { video: buffer, caption }, { quoted: msg });
          break;
        case "AUD":
          await sock.sendMessage(jid, { audio: buffer, mimetype: note.mimetype || "audio/mp4", ptt: false }, { quoted: msg });
          break;
        case "STK":
          await sock.sendMessage(jid, { sticker: buffer }, { quoted: msg });
          break;

        case "XLS":
        case "PPT":
        case "PDF":
        case "DOC":
          await sock.sendMessage(
            jid,
            {
              document: buffer,
              mimetype: note.mimetype || "application/octet-stream",
              fileName: fallbackName,
              caption,
            },
            { quoted: msg }
          );
          break;

        default:
          await sock.sendMessage(jid, { document: buffer, mimetype: "application/octet-stream", fileName: fallbackName, caption }, { quoted: msg });
      }
    }

    return true;
  }

  // 5. !delnote / !rmnote <num | name | path> (Strictly Owner Only)
  if (command === "!delnote" || command === "!rmnote" || command === "!deletenote") {
    // ⛔ Permission check
    if (!isAuthorizedOwner(sock, msg)) {
      await sock.sendMessage(
        jid,
        { text: "⛔ *Access Denied:* Only the bot owner/host is permitted to delete notes or directories." },
        { quoted: msg }
      );
      return true;
    }

    const target = args[1];
    if (!target) {
      await sock.sendMessage(jid, { text: "⚠️ Usage: `!delnote <num | name | path>`" }, { quoted: msg });
      return true;
    }

    let targetPath = null;
    let isFolder = false;
    const numIndex = parseInt(target, 10);

    if (!isNaN(numIndex)) {
      const { itemIndexMap } = buildTreeAndIndexMap(currentPath, notes, jid);
      const selected = itemIndexMap.find((item) => item.num === numIndex);
      if (selected) {
        targetPath = selected.fullPath;
        isFolder = selected.isFolder;
      }
    }

    if (!targetPath) {
      const possibleRelative = `${currentPath}${target}`;
      if (notes[possibleRelative]) {
        targetPath = possibleRelative;
      } else {
        const asFolder = possibleRelative.endsWith("/") ? possibleRelative : `${possibleRelative}/`;
        if (Object.keys(notes).some((p) => p.startsWith(asFolder))) {
          targetPath = asFolder;
          isFolder = true;
        }
      }
    }

    if (!targetPath) {
      const resolved = resolveFullPath(target, currentPath);
      if (notes[resolved]) {
        targetPath = resolved;
      } else {
        const asFolder = resolved.endsWith("/") ? resolved : `${resolved}/`;
        if (Object.keys(notes).some((p) => p.startsWith(asFolder))) {
          targetPath = asFolder;
          isFolder = true;
        }
      }
    }

    if (!targetPath) {
      await sock.sendMessage(jid, { text: `❌ Note or directory \`${target}\` not found.` }, { quoted: msg });
      return true;
    }

    // Protect root directories from accidental wipes
    if (targetPath === "root/" || targetPath === "root/public/" || targetPath === "root/private/") {
      await sock.sendMessage(jid, { text: "⚠️ *Cannot delete root directories!*" }, { quoted: msg });
      return true;
    }

    if (isFolder) {
      const folderPrefix = targetPath.endsWith("/") ? targetPath : `${targetPath}/`;
      let count = 0;
      for (const key of Object.keys(notes)) {
        if (key.startsWith(folderPrefix)) {
          delete notes[key];
          count++;
        }
      }
      await saveNotes(notes);
      const { viewText } = renderNestedDashboard(currentPath, notes, jid);
      await updateDashboardGUI(sock, jid, viewText + `\n\n🗑️ *Deleted folder \`${targetPath}\` (${count} notes removed)*`, msg);
    } else {
      delete notes[targetPath];
      await saveNotes(notes);
      const { viewText } = renderNestedDashboard(currentPath, notes, jid);
      await updateDashboardGUI(sock, jid, viewText + `\n\n🗑️ *Deleted note:* \`${targetPath}\``, msg);
    }

    return true;
  }

  return false;
}