import fs from "node:fs/promises";
import { existsSync } from "node:fs";
import path from "node:path";

const STORE_PATH = path.join(process.cwd(), "autoreply.json");

// Default State Structure supporting all 3 modes
const defaultData = {
  enabled: false,
  mode: "global", // "global" | "target" | "keyword"
  message: "🤖 *Auto-Reply:* I am currently away/busy. Leave your message or request here, and I'll get it when I'm back!",
  targets: [], // Array of string JIDs: ["1234567890@s.whatsapp.net"]
  triggers: {}, // Object map: { "pricing": "Our rates start at $50", "help": "..." }
  notes: [], // Array of { senderJid, senderName, text, timestamp }
};

let data = { ...defaultData };

/**
 * Load store from disk
 */
export async function loadAutoReplyStore() {
  try {
    if (existsSync(STORE_PATH)) {
      const raw = await fs.readFile(STORE_PATH, "utf-8");
      data = { ...defaultData, ...JSON.parse(raw) };
    } else {
      await saveAutoReplyStore();
    }
  } catch (err) {
    console.error("Failed to load Auto-Reply store:", err);
  }
}

/**
 * Save store to disk
 */
async function saveAutoReplyStore() {
  try {
    await fs.writeFile(STORE_PATH, JSON.stringify(data, null, 2));
  } catch (err) {
    console.error("Failed to save Auto-Reply store:", err);
  }
}

// State & Mode Controls
export function isAutoReplyEnabled() {
  return data.enabled;
}

export function getAutoReplyMode() {
  return data.mode || "global";
}

export async function setAutoReplyMode(mode) {
  if (["global", "target", "keyword"].includes(mode)) {
    data.mode = mode;
    await saveAutoReplyStore();
  }
}

export async function setAutoReplyState(enabled) {
  data.enabled = enabled;
  await saveAutoReplyStore();
}

export function getAutoReplyMessage() {
  return data.message;
}

export async function setAutoReplyCustomMessage(newMessage) {
  data.message = newMessage;
  await saveAutoReplyStore();
}

// Target Management (Specific Contact Mode)
export async function addAutoReplyTarget(jidOrNumber) {
  const cleanJid = jidOrNumber.includes("@")
    ? jidOrNumber
    : `${jidOrNumber.replace(/\D/g, "")}@s.whatsapp.net`;

  if (!data.targets.includes(cleanJid)) {
    data.targets.push(cleanJid);
    await saveAutoReplyStore();
  }
}

export async function removeAutoReplyTarget(jidOrNumber) {
  const cleanJid = jidOrNumber.includes("@")
    ? jidOrNumber
    : `${jidOrNumber.replace(/\D/g, "")}@s.whatsapp.net`;

  data.targets = data.targets.filter((t) => t !== cleanJid && !t.startsWith(cleanJid.split("@")[0]));
  await saveAutoReplyStore();
}

export function getAutoReplyTargets() {
  return data.targets || [];
}

// Trigger Management (Keyword Match Mode)
export async function addAutoReplyTrigger(keyword, replyMsg) {
  const cleanKey = keyword.toLowerCase().trim();
  data.triggers[cleanKey] = replyMsg.trim();
  await saveAutoReplyStore();
}

export async function removeAutoReplyTrigger(keyword) {
  const cleanKey = keyword.toLowerCase().trim();
  delete data.triggers[cleanKey];
  await saveAutoReplyStore();
}

export function getAutoReplyTriggers() {
  return Object.entries(data.triggers || {});
}

// Notes / Messages Recording
export async function addAutoReplyNote(senderJid, senderName, text) {
  data.notes.push({
    senderJid,
    senderName: senderName || senderJid.split("@")[0],
    text,
    timestamp: new Date().toLocaleString(),
  });
  await saveAutoReplyStore();
}

export async function flushAutoReplyNotes() {
  const notes = [...data.notes];
  data.notes = [];
  await saveAutoReplyStore();
  return notes;
}