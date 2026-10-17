/**
 * Modes: 'global' | 'target' | 'keyword'
 */
const autoReplyState = {
  enabled: false,
  mode: "global",
  defaultMessage: "I am currently away. Please leave a note and I will get back to you soon.",
  targets: new Set(), // Set of JIDs or phone numbers (e.g. "1234567890@s.whatsapp.net")
  triggers: new Map(), // Map<keyword, customReplyMessage>
  notes: [],
};

export function isAutoReplyEnabled() {
  return autoReplyState.enabled;
}

export function getAutoReplyMode() {
  return autoReplyState.mode;
}

export function setAutoReplyMode(mode) {
  if (["global", "target", "keyword"].includes(mode)) {
    autoReplyState.mode = mode;
  }
}

export function setAutoReplyState(enabled) {
  autoReplyState.enabled = enabled;
}

export function getAutoReplyMessage() {
  return autoReplyState.defaultMessage;
}

export function setAutoReplyCustomMessage(msg) {
  autoReplyState.defaultMessage = msg;
}

// Target Management
export function addAutoReplyTarget(jidOrNumber) {
  const cleanJid = jidOrNumber.includes("@") ? jidOrNumber : `${jidOrNumber.replace(/\D/g, "")}@s.whatsapp.net`;
  autoReplyState.targets.add(cleanJid);
}

export function removeAutoReplyTarget(jidOrNumber) {
  const cleanJid = jidOrNumber.includes("@") ? jidOrNumber : `${jidOrNumber.replace(/\D/g, "")}@s.whatsapp.net`;
  autoReplyState.targets.delete(cleanJid);
}

export function getAutoReplyTargets() {
  return Array.from(autoReplyState.targets);
}

// Trigger Management
export function addAutoReplyTrigger(keyword, replyMsg) {
  autoReplyState.triggers.set(keyword.toLowerCase().trim(), replyMsg);
}

export function removeAutoReplyTrigger(keyword) {
  autoReplyState.triggers.delete(keyword.toLowerCase().trim());
}

export function getAutoReplyTriggers() {
  return Array.from(autoReplyState.triggers.entries());
}

// Notes
export async function addAutoReplyNote(senderJid, senderName, text) {
  autoReplyState.notes.push({
    senderJid,
    senderName,
    text,
    timestamp: new Date().toLocaleString(),
  });
}

export async function flushAutoReplyNotes() {
  const notes = [...autoReplyState.notes];
  autoReplyState.notes = [];
  return notes;
}