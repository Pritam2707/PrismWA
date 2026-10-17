import { downloadMediaMessage } from "baileys";
import fs from "node:fs/promises";
import path from "node:path";

const WELCOME_DATA_PATH = path.join(process.cwd(), "data", "welcome_settings.json");
let welcomeStore = new Map();

// Initialize JSON store
async function loadWelcomeStore() {
  try {
    const raw = await fs.readFile(WELCOME_DATA_PATH, "utf-8");
    const parsed = JSON.parse(raw);
    welcomeStore = new Map(Object.entries(parsed));
  } catch (err) {
    welcomeStore = new Map();
  }
}

async function saveWelcomeStore() {
  try {
    const data = Object.fromEntries(welcomeStore);
    await fs.mkdir(path.dirname(WELCOME_DATA_PATH), { recursive: true });
    await fs.writeFile(WELCOME_DATA_PATH, JSON.stringify(data, null, 2));
  } catch (err) {
    console.error("Failed to save welcome settings:", err);
  }
}

// Initial load
loadWelcomeStore();

/**
 * Command Handler for Admin Welcome Configuration
 */
export async function handleWelcomeCommand(sock, jid, senderId, rawText, msg) {
  if (typeof rawText !== "string" || !rawText.trim()) return false;

  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();

  if (!["!welcome", "!setwelcome"].includes(command)) return false;

  const isGroup = jid.endsWith("@g.us");
  if (!isGroup) {
    await sock.sendMessage(jid, { text: "⚠️ Welcome settings can only be configured in group chats." }, { quoted: msg });
    return true;
  }

  // Admin Check
  const groupMetadata = await sock.groupMetadata(jid);
  const participants = groupMetadata.participants || [];
  const senderParticipant = participants.find((p) => p.id === senderId);
  const isAdmin = senderParticipant?.admin === "admin" || senderParticipant?.admin === "superadmin" || msg.key.fromMe;

  if (!isAdmin) {
    await sock.sendMessage(jid, { text: "🚫 Only group admins can configure welcome messages." }, { quoted: msg });
    return true;
  }

  const subCommand = args[1]?.toLowerCase();
  const settings = welcomeStore.get(jid) || { enabled: false, text: "Welcome @user to @group!", mediaBase64: null, mediaType: null };

  // Helper menu
  if (!subCommand) {
    const menu =
      "👋 *Group Welcomer Settings*\n\n" +
      "• `!welcome on / off` : Enable or disable welcome messages\n" +
      "• `!welcome set <text>` : Set custom greeting\n" +
      "  *(Placeholders: `@user` for member mention, `@group` for group name)*\n" +
      "• *(Attach/Reply Image or GIF)* `!welcome setimage` : Set custom welcome media\n" +
      "• `!welcome resetimage` : Remove welcome media\n" +
      "• `!welcome test` : Preview current welcome message";

    await sock.sendMessage(jid, { text: menu }, { quoted: msg });
    return true;
  }

  if (subCommand === "on") {
    settings.enabled = true;
    welcomeStore.set(jid, settings);
    await saveWelcomeStore();
    await sock.sendMessage(jid, { text: "✅ *Welcome messages enabled for this group!*" }, { quoted: msg });
    return true;
  }

  if (subCommand === "off") {
    settings.enabled = false;
    welcomeStore.set(jid, settings);
    await saveWelcomeStore();
    await sock.sendMessage(jid, { text: "🛑 *Welcome messages disabled for this group.*" }, { quoted: msg });
    return true;
  }

  if (subCommand === "set") {
    const customText = args.slice(2).join(" ").trim();
    if (!customText) {
      await sock.sendMessage(jid, { text: "⚠️ Please provide a message. Example:\n`!welcome set Hey @user, welcome to @group!`" }, { quoted: msg });
      return true;
    }
    settings.text = customText;
    welcomeStore.set(jid, settings);
    await saveWelcomeStore();
    await sock.sendMessage(jid, { text: "✅ *Custom welcome text updated successfully!*" }, { quoted: msg });
    return true;
  }

  if (subCommand === "setimage") {
    const contextInfo = msg.message?.extendedTextMessage?.contextInfo;
    const quotedMsg = contextInfo?.quotedMessage;

    // Images
    const imgMsg = msg.message?.imageMessage || quotedMsg?.imageMessage;
    // Videos & GIFs — WhatsApp sends animated GIFs as videoMessage with gifPlayback: true
    const vidMsg = msg.message?.videoMessage || quotedMsg?.videoMessage;

    const targetMsg = imgMsg || vidMsg;

    if (!targetMsg) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ Please attach or reply to an image or GIF with `!welcome setimage`." },
        { quoted: msg }
      );
      return true;
    }

    const isGif = !!vidMsg;
    // Basic size guard so huge videos don't get treated as "gifs"
    if (isGif && vidMsg?.seconds && vidMsg.seconds > 15) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ That video is too long — please use a short GIF (under ~15s)." },
        { quoted: msg }
      );
      return true;
    }

    try {
      const messageType = isGif ? "videoMessage" : "imageMessage";
      const buffer = await downloadMediaMessage({ message: { [messageType]: targetMsg } }, "buffer", {});

      settings.mediaBase64 = buffer.toString("base64");
      settings.mediaType = isGif ? "gif" : "image";

      // Clean up legacy field if present
      delete settings.imageBase64;

      welcomeStore.set(jid, settings);
      await saveWelcomeStore();
      await sock.sendMessage(
        jid,
        { text: `✅ *Custom welcome ${isGif ? "GIF" : "image"} set successfully!*` },
        { quoted: msg }
      );
    } catch (e) {
      console.error("Failed to save welcome media:", e);
      await sock.sendMessage(jid, { text: "❌ Failed to save media." }, { quoted: msg });
    }
    return true;
  }

  if (subCommand === "resetimage") {
    delete settings.mediaBase64;
    delete settings.mediaType;
    delete settings.imageBase64; // legacy cleanup
    welcomeStore.set(jid, settings);
    await saveWelcomeStore();
    await sock.sendMessage(jid, { text: "✅ *Custom welcome media removed.*" }, { quoted: msg });
    return true;
  }

  if (subCommand === "test") {
    await triggerGroupWelcome(sock, jid, [senderId]);
    return true;
  }

  return true;
}

/**
 * Event Handler for Group Member Join Events
 */
export async function triggerGroupWelcome(sock, groupJid, newMembers) {
  const settings = welcomeStore.get(groupJid);
  if (!settings || !settings.enabled) return;

  try {
    const metadata = await sock.groupMetadata(groupJid);
    const groupName = metadata.subject || "the group";

    for (const member of newMembers) {
      // Baileys can hand back plain JID strings or participant objects
      // ({ id, phoneNumber, lid, ... }) depending on version/event source.
      const memberId = typeof member === "string" ? member : member?.id;

      if (!memberId || typeof memberId !== "string") {
        console.warn("Skipping welcome message: unresolvable member id", member);
        continue;
      }

      let caption = settings.text || "Welcome @user to @group!";
      caption = caption.replace(/@user/g, `@${memberId.split("@")[0]}`).replace(/@group/g, groupName);

      // Support legacy `imageBase64` field alongside new `mediaBase64`/`mediaType`
      const mediaBase64 = settings.mediaBase64 || settings.imageBase64;
      const mediaType = settings.mediaType || (settings.imageBase64 ? "image" : null);

      if (mediaBase64) {
        const mediaBuffer = Buffer.from(mediaBase64, "base64");

        if (mediaType === "gif") {
          await sock.sendMessage(groupJid, {
            video: mediaBuffer,
            gifPlayback: true,
            caption,
            mentions: [memberId],
          });
        } else {
          await sock.sendMessage(groupJid, {
            image: mediaBuffer,
            caption,
            mentions: [memberId],
          });
        }
      } else {
        await sock.sendMessage(groupJid, {
          text: caption,
          mentions: [memberId],
        });
      }
    }
  } catch (err) {
    console.error("Failed to send welcome message:", err);
  }
}