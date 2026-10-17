import {
  wakeUpBot,
  enterSleepMode,
  getIsSleeping,
  getLastActivityTime,
} from "../utils/sleepManager.js";
import {
  activeAntiDeleteChats,
  activeFeatureChats,
  savePersistentData,
} from "../store/messageStore.js";

export async function handleSystemCommands(sock, jid, rawText, msg) {
  const text = rawText.toLowerCase().trim();

  // 1. Wake Command
  if (text === "!wake") {
    const wasAsleep = wakeUpBot();
    const reply = wasAsleep
      ? "⏰ *PrismBot is now awake!* Ready to take commands."
      : "⚡ *PrismBot is already active and awake.*";
    await sock.sendMessage(jid, { text: reply }, { quoted: msg });
    return true;
  }

  // 2. Sleep Command
  if (text === "!sleep") {
    enterSleepMode();
    await sock.sendMessage(
      jid,
      { text: "💤 *PrismBot is now in Sleep Mode.* Send `!wake` to wake me up!" },
      { quoted: msg }
    );
    return true;
  }

  // 3. Status / Uptime Command
  if (text === "!botstatus" || text === "!uptime" || text === "!status") {
    const statusText = getIsSleeping() ? "💤 Sleeping" : "⚡ Active & Awake";
    const antiDeleteStatus = activeAntiDeleteChats.has(jid) ? "✅ Enabled" : "❌ Disabled";
    const featuresStatus = activeFeatureChats.has(jid) ? "✅ Enabled" : "❌ Disabled";
    const lastActiveSecs = Math.floor((Date.now() - getLastActivityTime()) / 1000);

    await sock.sendMessage(
      jid,
      {
        text: `🤖 *PrismBot Status:*\n\n• *Bot:* ${statusText}\n• *Anti-Delete:* ${antiDeleteStatus}\n• *Bot Features:* ${featuresStatus}\n• *Last Activity:* ${lastActiveSecs}s ago`,
      },
      { quoted: msg }
    );
    return true;
  }

  // 4. Toggle Anti-Delete (OWNER ONLY)
  if (text === "!antidelete" || text === "!autodelete") {
    if (!msg.key.fromMe) {
      await sock.sendMessage(
        jid,
        { text: "❌ Only the bot owner can toggle anti-delete settings." },
        { quoted: msg }
      );
      return true;
    }

    if (activeAntiDeleteChats.has(jid)) {
      activeAntiDeleteChats.delete(jid);
      await sock.sendMessage(
        jid,
        { text: "❌ Anti-delete protection disabled for this chat." },
        { quoted: msg }
      );
    } else {
      activeAntiDeleteChats.add(jid);
      await sock.sendMessage(
        jid,
        { text: "✅ Anti-delete protection enabled for this chat." },
        { quoted: msg }
      );
    }

    // 💾 Persist state changes to ./data/store.json
    savePersistentData();
    return true;
  }

  // 5. Toggle Interactive Features (OWNER ONLY)
  if (text.startsWith("!features")) {
    if (!msg.key.fromMe) {
      await sock.sendMessage(
        jid,
        { text: "❌ Only the bot owner can activate/deactivate bot features." },
        { quoted: msg }
      );
      return true;
    }

    if (text === "!features on" || text === "!features enable") {
      activeFeatureChats.add(jid);
      await sock.sendMessage(
        jid,
        { text: "✅ Interactive features (AI, Music, TTS) are now ENABLED in this chat." },
        { quoted: msg }
      );
      savePersistentData(); // 💾 Persist state
    } else if (text === "!features off" || text === "!features disable") {
      activeFeatureChats.delete(jid);
      await sock.sendMessage(
        jid,
        { text: "❌ Interactive features are now DISABLED in this chat." },
        { quoted: msg }
      );
      savePersistentData(); // 💾 Persist state
    } else {
      const isEnabled = activeFeatureChats.has(jid);
      await sock.sendMessage(
        jid,
        {
          text: `⚙️ *Features for this chat:* ${isEnabled ? "✅ Enabled" : "❌ Disabled"}\nUse \`!features on\` or \`!features off\` to change.`,
        },
        { quoted: msg }
      );
    }
    return true;
  }

  return false;
}