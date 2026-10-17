export async function sendTypingStatus(sock, jid) {
  try {
    await sock.sendPresenceUpdate("composing", jid);
  } catch (e) {}
}

export async function sendRecordingStatus(sock, jid) {
  try {
    await sock.sendPresenceUpdate("recording", jid);
  } catch (e) {}
}

export async function sendPausedStatus(sock, jid) {
  try {
    await sock.sendPresenceUpdate("paused", jid);
  } catch (e) {}
}

/**
 * Lightweight WhatsApp Reaction Helper
 * Sends emoji reaction on a specific message without polluting chat with text.
 */
export async function sendReaction(sock, jid, messageKey, emoji) {
  if (!sock || !jid || !messageKey || !emoji) return;
  try {
    await sock.sendMessage(jid, {
      react: {
        text: emoji,
        key: messageKey,
      },
    });
  } catch (e) {}
}

/**
 * Clears reaction from a message
 */
export async function clearReaction(sock, jid, messageKey) {
  if (!sock || !jid || !messageKey) return;
  try {
    await sock.sendMessage(jid, {
      react: {
        text: "",
        key: messageKey,
      },
    });
  } catch (e) {}
}