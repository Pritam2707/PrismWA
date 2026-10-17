import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

const DATA_DIR = path.join(process.cwd(), "data");
const MUTE_FILE = path.join(DATA_DIR, "muted_users.json");

if (!existsSync(DATA_DIR)) {
  mkdirSync(DATA_DIR, { recursive: true });
}

function loadMutedUsers() {
  try {
    if (existsSync(MUTE_FILE)) {
      return JSON.parse(readFileSync(MUTE_FILE, "utf-8"));
    }
  } catch (e) {
    console.error("Failed to read muted users file:", e);
  }
  return {};
}

function saveMutedUsers(data) {
  try {
    writeFileSync(MUTE_FILE, JSON.stringify(data, null, 2), "utf-8");
  } catch (e) {
    console.error("Failed to save muted users file:", e);
  }
}


async function isAdmin(sock, jid, senderId) {
  try {
    const groupMetadata = await sock.groupMetadata(jid);
    const participants = groupMetadata.participants || [];
    const participant = participants.find((p) => p.id === senderId);
    return participant?.admin === "admin" || participant?.admin === "superadmin";
  } catch (e) {
    return false;
  }
}

function getBotJids(sock) {
  const botUser = sock.user;
  if (!botUser) return [];
  
  const rawId = botUser.id || "";
  const normalizedPhoneJid = rawId.split(":")[0] + "@s.whatsapp.net";
  
  return [rawId, normalizedPhoneJid, botUser.lid].filter(Boolean);
}


export async function checkAndHandleMutedUser(sock, jid, senderId, msg) {
  const mutedDB = loadMutedUsers();
  const chatMutedList = mutedDB[jid] || [];

  if (chatMutedList.includes(senderId)) {
    try {
      // Auto-delete the muted user's message
      await sock.sendMessage(jid, { delete: msg.key });
      return true; // Message was intercepted and deleted
    } catch (err) {
      console.error("Failed to delete muted message (Bot might not be admin):", err);
    }
  }
  return false;
}


export async function handleMuteCommand(sock, jid, senderId, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();
  const isGroup = jid.endsWith("@g.us");

  if (!["!mute", "!unmute", "!mutelist"].includes(command)) {
    return false;
  }

  if (!isGroup) {
    await sock.sendMessage(jid, { text: "❌ Mute commands can only be used in group chats." }, { quoted: msg });
    return true;
  }

  // Ensure command sender is an admin
  const senderIsAdmin = await isAdmin(sock, jid, senderId);
  if (!senderIsAdmin) {
    await sock.sendMessage(jid, { text: "❌ Only group admins can use mute commands." }, { quoted: msg });
    return true;
  }

  const mutedDB = loadMutedUsers();
  if (!mutedDB[jid]) {
    mutedDB[jid] = [];
  }


  const quotedSender = msg.message?.extendedTextMessage?.contextInfo?.participant;
  const mentionedSender = msg.message?.extendedTextMessage?.contextInfo?.mentionedJid?.[0];
  const targetUser = mentionedSender || quotedSender;

  // 1. Mute User (!mute @user or reply to user)
  if (command === "!mute") {
    if (!targetUser) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ *Usage:* Reply to a user's message or tag them with `!mute @user`" },
        { quoted: msg }
      );
      return true;
    }

    // 🛑 Anti-Self Mute Guard
    const botJids = getBotJids(sock);
    const isBotTarget = botJids.some(
      (bJid) => bJid === targetUser || bJid.split("@")[0] === targetUser.split("@")[0]
    );

    if (isBotTarget) {
      await sock.sendMessage(jid, { text: "🤖 *I cannot mute myself!*" }, { quoted: msg });
      return true;
    }

    // Check if target is an admin
    const targetIsAdmin = await isAdmin(sock, jid, targetUser);
    if (targetIsAdmin) {
      await sock.sendMessage(jid, { text: "❌ You cannot mute a group admin!" }, { quoted: msg });
      return true;
    }

    if (mutedDB[jid].includes(targetUser)) {
      await sock.sendMessage(jid, { text: `⚠️ @${targetUser.split("@")[0]} is already muted.`, mentions: [targetUser] }, { quoted: msg });
      return true;
    }

    mutedDB[jid].push(targetUser);
    saveMutedUsers(mutedDB);

    await sock.sendMessage(
      jid,
      {
        text: `🤫 *User Muted!*\n\n@${targetUser.split("@")[0]} has been muted by an admin. All their future messages will be automatically deleted.`,
        mentions: [targetUser],
      },
      { quoted: msg }
    );
    return true;
  }

  // 2. Unmute User (!unmute @user or reply to user)
  if (command === "!unmute") {
    if (!targetUser) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ *Usage:* Reply to a user's message or tag them with `!unmute @user`" },
        { quoted: msg }
      );
      return true;
    }

    if (!mutedDB[jid].includes(targetUser)) {
      await sock.sendMessage(jid, { text: `⚠️ @${targetUser.split("@")[0]} is not muted.`, mentions: [targetUser] }, { quoted: msg });
      return true;
    }

    mutedDB[jid] = mutedDB[jid].filter((id) => id !== targetUser);
    saveMutedUsers(mutedDB);

    await sock.sendMessage(
      jid,
      {
        text: `🔊 *User Unmuted!*\n\n@${targetUser.split("@")[0]} can now send messages again.`,
        mentions: [targetUser],
      },
      { quoted: msg }
    );
    return true;
  }

  // 3. View Muted List (!mutelist)
  if (command === "!mutelist") {
    const list = mutedDB[jid] || [];

    if (list.length === 0) {
      await sock.sendMessage(jid, { text: "🔊 No users are muted in this group." }, { quoted: msg });
      return true;
    }

    let text = `🤫 *Muted Users in this Group (${list.length}):*\n\n`;
    list.forEach((user, idx) => {
      text += `*${idx + 1}.* @${user.split("@")[0]}\n`;
    });

    await sock.sendMessage(jid, { text, mentions: list }, { quoted: msg });
    return true;
  }

  return false;
}