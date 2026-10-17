import {
  isAutoReplyEnabled,
  getAutoReplyMode,
  setAutoReplyMode,
  getAutoReplyMessage,
  setAutoReplyState,
  setAutoReplyCustomMessage,
  addAutoReplyTarget,
  removeAutoReplyTarget,
  getAutoReplyTargets,
  addAutoReplyTrigger,
  removeAutoReplyTrigger,
  getAutoReplyTriggers,
  addAutoReplyNote,
  flushAutoReplyNotes,
} from "../services/autoreplyStore.js";

/**
 * Command Handler for !autoreply
 */
export async function handleAutoReplyCommand(sock, jid, senderId, rawText, msg) {
  const args = rawText.trim().split(/\s+/);
  const command = args[0].toLowerCase();

  if (command !== "!autoreply" && command !== "!away") return false;

  const subCommand = args[1]?.toLowerCase();

  // 1. Toggle ON: !autoreply on [mode]
  if (subCommand === "on") {
    const requestedMode = args[2]?.toLowerCase();
    if (requestedMode && ["global", "target", "keyword"].includes(requestedMode)) {
      setAutoReplyMode(requestedMode);
    }

    await setAutoReplyState(true);

    await sock.sendMessage(
      jid,
      {
        text:
          `✅ *Auto-Reply Enabled!*\n\n` +
          `• *Mode:* \`${getAutoReplyMode().toUpperCase()}\`\n` +
          `• *Default Reply:* "${getAutoReplyMessage()}"\n\n` +
          `💡 Use \`!autoreply mode <global|target|keyword>\` to change behavior.`
      },
      { quoted: msg }
    );
    return true;
  }

  // 2. Toggle OFF: !autoreply off
  if (subCommand === "off") {
    await setAutoReplyState(false);
    const pendingNotes = await flushAutoReplyNotes();

    let text = `🔴 *Auto-Reply Disabled!*\n\n`;

    if (pendingNotes.length === 0) {
      text += `📬 No messages or notes were recorded while active.`;
    } else {
      text += `📬 *Messages Left While Away (${pendingNotes.length}):*\n\n`;
      pendingNotes.forEach((note, index) => {
        text += `*${index + 1}. From:* @${note.senderJid.split("@")[0]} (${note.senderName})\n`;
        text += `   *Time:* ${note.timestamp}\n`;
        text += `   *Note:* "${note.text}"\n\n`;
      });
    }

    await sock.sendMessage(jid, { text, mentions: pendingNotes.map((n) => n.senderJid) }, { quoted: msg });
    return true;
  }

  // 3. Switch Mode: !autoreply mode <global|target|keyword>
  if (subCommand === "mode") {
    const targetMode = args[2]?.toLowerCase();
    if (!["global", "target", "keyword"].includes(targetMode)) {
      await sock.sendMessage(
        jid,
        { text: `⚠️ Invalid mode. Choose between:\n• \`global\` (all DMs)\n• \`target\` (specific contacts)\n• \`keyword\` (trigger phrases)` },
        { quoted: msg }
      );
      return true;
    }

    setAutoReplyMode(targetMode);
    await sock.sendMessage(jid, { text: `🔄 Auto-reply mode switched to: *${targetMode.toUpperCase()}*` }, { quoted: msg });
    return true;
  }

  // 4. Manage Targets: !autoreply addtarget / !autoreply deltarget
  if (subCommand === "addtarget") {
    const target = args[2];
    if (!target) {
      await sock.sendMessage(jid, { text: "⚠️ Usage: `!autoreply addtarget <phone_number_or_jid>`" }, { quoted: msg });
      return true;
    }
    addAutoReplyTarget(target);
    await sock.sendMessage(jid, { text: `🎯 Added target recipient: *${target}*` }, { quoted: msg });
    return true;
  }

  if (subCommand === "deltarget") {
    const target = args[2];
    if (!target) {
      await sock.sendMessage(jid, { text: "⚠️ Usage: `!autoreply deltarget <phone_number_or_jid>`" }, { quoted: msg });
      return true;
    }
    removeAutoReplyTarget(target);
    await sock.sendMessage(jid, { text: `🗑️ Removed target: *${target}*` }, { quoted: msg });
    return true;
  }

  // 5. Manage Trigger Phrases: !autoreply addtrigger <phrase> | <custom_response>
  if (subCommand === "addtrigger") {
    const payload = args.slice(2).join(" ");
    const [phrase, ...responseParts] = payload.split("|");
    const response = responseParts.join("|").trim();

    if (!phrase || !response) {
      await sock.sendMessage(
        jid,
        { text: "⚠️ Format: `!autoreply addtrigger <phrase> | <response text>`\nExample: `!autoreply addtrigger pricing | Our rates start at $50.`" },
        { quoted: msg }
      );
      return true;
    }

    addAutoReplyTrigger(phrase.trim(), response);
    await sock.sendMessage(jid, { text: `⚡ Added keyword trigger:\n• *Trigger:* "${phrase.trim()}"\n• *Response:* "${response}"` }, { quoted: msg });
    return true;
  }

  if (subCommand === "deltrigger") {
    const phrase = args.slice(2).join(" ").trim();
    if (!phrase) {
      await sock.sendMessage(jid, { text: "⚠️ Usage: `!autoreply deltrigger <phrase>`" }, { quoted: msg });
      return true;
    }
    removeAutoReplyTrigger(phrase);
    await sock.sendMessage(jid, { text: `🗑️ Removed keyword trigger: "${phrase}"` }, { quoted: msg });
    return true;
  }

  // 6. Set Default Message: !autoreply set <message>
  if (subCommand === "set") {
    const newMsg = args.slice(2).join(" ").trim();
    if (!newMsg) {
      await sock.sendMessage(jid, { text: "⚠️ Usage: `!autoreply set <text>`" }, { quoted: msg });
      return true;
    }

    setAutoReplyCustomMessage(newMsg);
    await sock.sendMessage(jid, { text: `✅ Updated default reply message to:\n"${newMsg}"` }, { quoted: msg });
    return true;
  }

  // Help / Status Output
  const isEnabled = isAutoReplyEnabled();
  const mode = getAutoReplyMode();
  const targets = getAutoReplyTargets();
  const triggers = getAutoReplyTriggers();

  await sock.sendMessage(
    jid,
    {
      text:
        `🤖 *Auto-Reply Manager*\n\n` +
        `• *Status:* ${isEnabled ? "🟢 ACTIVE" : "🔴 INACTIVE"}\n` +
        `• *Active Mode:* \`${mode.toUpperCase()}\`\n` +
        `• *Default Text:* "${getAutoReplyMessage()}"\n` +
        `• *Target Contacts (${targets.length}):* ${targets.join(", ") || "None"}\n` +
        `• *Active Triggers (${triggers.length}):* ${triggers.map(([k]) => `"${k}"`).join(", ") || "None"}\n\n` +
        `*Commands:*\n` +
        `• \`!autoreply on [global|target|keyword]\`\n` +
        `• \`!autoreply off\`\n` +
        `• \`!autoreply mode <global|target|keyword>\`\n` +
        `• \`!autoreply set <default_message>\`\n` +
        `• \`!autoreply addtarget <number_or_jid>\`\n` +
        `• \`!autoreply deltarget <number_or_jid>\`\n` +
        `• \`!autoreply addtrigger <phrase> | <reply>\`\n` +
        `• \`!autoreply deltrigger <phrase>\``
    },
    { quoted: msg }
  );
  return true;
}

/**
 * Middleware Listener: Evaluates incoming messages against the active mode
 */
export async function processAutoReplyListener(sock, msg) {
  if (!isAutoReplyEnabled()) return;

  const isFromMe = msg.key.fromMe;
  const jid = msg.key.remoteJid;
  const isGroup = jid.endsWith("@g.us");

  if (isFromMe || isGroup || jid.includes("status@broadcast")) return;

  const incomingText =
    msg.message?.conversation ||
    msg.message?.extendedTextMessage?.text ||
    "";

  if (incomingText.startsWith("!")) return;

  const senderJid = msg.key.participant || jid;
  const senderName = msg.pushName || "Unknown Contact";
  const mode = getAutoReplyMode();

  let shouldReply = false;
  let replyText = getAutoReplyMessage();

  // Mode 1: Global Mode (Replies to all private messages)
  if (mode === "global") {
    shouldReply = true;
  }

  // Mode 2: Targeted Person Mode (Replies only to configured JIDs)
  else if (mode === "target") {
    const targets = getAutoReplyTargets();
    const senderNumber = senderJid.split("@")[0];
    const isTargeted = targets.some((t) => t === senderJid || t.startsWith(senderNumber));

    if (isTargeted) {
      shouldReply = true;
    }
  }

  // Mode 3: Keyword / Trigger Mode (Checks if message includes any trigger phrase)
  else if (mode === "keyword") {
    const triggers = getAutoReplyTriggers();
    const lowerText = incomingText.toLowerCase();

    for (const [phrase, customReply] of triggers) {
      if (lowerText.includes(phrase)) {
        shouldReply = true;
        replyText = customReply;
        break;
      }
    }
  }

  // Execute reply and log notes if conditions were met
  if (shouldReply) {
    await sock.sendMessage(jid, { text: replyText }, { quoted: msg });

    if (incomingText.trim()) {
      await addAutoReplyNote(senderJid, senderName, incomingText.trim());
    }
  }
}