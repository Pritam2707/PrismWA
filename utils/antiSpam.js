// Stores user command timestamps and spam infraction counts
const cooldownMap = new Map(); // senderId -> lastTimestamp
const warningMap = new Map();  // senderId -> warningCount
const muteMap = new Map();     // senderId -> mutedUntilTimestamp

const COOLDOWN_MS = 2500;       // Minimum delay between commands (2.5 seconds)
const MAX_WARNINGS = 4;         // Warnings allowed before temporary mute
const MUTE_DURATION_MS = 60000; // Mute length (1 minute)

/**
 * Checks if a message sender is abusing/spamming commands.
 * @param {string} senderId - WhatsApp JID of the sender.
 * @returns {object} { isSpam: boolean, reason: string | null }
 */
export function checkSpam(senderId) {
  const now = Date.now();

  // 1. Check if user is currently muted
  if (muteMap.has(senderId)) {
    const mutedUntil = muteMap.get(senderId);
    if (now < mutedUntil) {
      return { isSpam: true, reason: "MUTED" };
    } else {
      // Mute period expired
      muteMap.delete(senderId);
      warningMap.delete(senderId);
    }
  }

  // 2. Check cooldown between consecutive commands
  const lastTime = cooldownMap.get(senderId) || 0;
  const timeDiff = now - lastTime;

  if (timeDiff < COOLDOWN_MS) {
    const currentWarnings = (warningMap.get(senderId) || 0) + 1;
    warningMap.set(senderId, currentWarnings);

    if (currentWarnings >= MAX_WARNINGS) {
      // Mute user temporarily
      muteMap.set(senderId, now + MUTE_DURATION_MS);
      return { isSpam: true, reason: "JUST_MUTED" };
    }

    return { isSpam: true, reason: "COOLDOWN", warningsLeft: MAX_WARNINGS - currentWarnings };
  }

  // Reset timestamps and warning counters on valid interaction
  cooldownMap.set(senderId, now);
  if (warningMap.get(senderId) > 0) {
    warningMap.set(senderId, Math.max(0, warningMap.get(senderId) - 1));
  }

  return { isSpam: false, reason: null };
}