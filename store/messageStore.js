import { existsSync, readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

// Define path to the data folder and persistence file
const DATA_DIR = "./data";
const DATA_FILE = join(DATA_DIR, "store.json");

// Ensure the data directory exists
if (!existsSync(DATA_DIR)) {
  mkdirSync(DATA_DIR, { recursive: true });
}

// ---------------------------------------------------------------------------
// Persistence Helpers
// ---------------------------------------------------------------------------

function loadPersistentData() {
  try {
    if (existsSync(DATA_FILE)) {
      const raw = readFileSync(DATA_FILE, "utf-8");
      return JSON.parse(raw);
    }
  } catch (err) {
    console.error("❌ Failed to load persistent data:", err.message);
  }
  return { antiDeleteChats: [], featureChats: [] };
}

export function savePersistentData() {
  try {
    const payload = {
      antiDeleteChats: Array.from(activeAntiDeleteChats),
      featureChats: Array.from(activeFeatureChats),
    };
    writeFileSync(DATA_FILE, JSON.stringify(payload, null, 2), "utf-8");
  } catch (err) {
    console.error("❌ Failed to persist data:", err.message);
  }
}

const CONTACTS_FILE = join(DATA_DIR, "contacts.json");

function loadContacts() {
  try {
    if (existsSync(CONTACTS_FILE)) {
      const raw = readFileSync(CONTACTS_FILE, "utf-8");
      return new Map(Object.entries(JSON.parse(raw)));
    }
  } catch (err) {
    console.error("❌ Failed to load contacts:", err.message);
  }
  return new Map();
}

export function saveContacts() {
  try {
    const payload = Object.fromEntries(contactStore);
    writeFileSync(CONTACTS_FILE, JSON.stringify(payload, null, 2), "utf-8");
  } catch (err) {
    console.error("❌ Failed to persist contacts:", err.message);
  }
}

export function setContact(jid, name) {
  if (!jid || !name) return;
  contactStore.set(jid, name);
  saveContacts();
}

export function getDisplayName(jid) {
  if (!jid) return "Unknown";
  if (contactStore.has(jid)) return contactStore.get(jid);
  const cleanPhone = jid.split("@")[0].split(":")[0];
  if (jid.endsWith("@g.us")) {
    return `Group (${cleanPhone.slice(-4)})`;
  }
  if (jid.endsWith("@lid")) {
    return `User (${cleanPhone.slice(-4)})`;
  }
  if (cleanPhone.length >= 10) {
    return `+${cleanPhone}`;
  }
  return cleanPhone || jid;
}

// ---------------------------------------------------------------------------
// Store Initialization
// ---------------------------------------------------------------------------

const initialData = loadPersistentData();

export const messageStore = new Map();
export const contactStore = loadContacts();

// Populate Sets from saved JSON file
export const activeAntiDeleteChats = new Set(initialData.antiDeleteChats);
export const activeFeatureChats = new Set(initialData.featureChats);

// ---------------------------------------------------------------------------
// Message Store Helpers
// ---------------------------------------------------------------------------

/**
 * Stores incoming message payloads with deep cloning & multi-key indexing
 */
export function storeMessage(msg) {
  if (!msg || !msg.key || !msg.key.id) return;

  const rawId = msg.key.id;
  const cleanId = rawId.replace(/[^a-zA-Z0-9]/g, ""); // Strip any non-alphanumeric artifacts

  let clonedMsg;
  try {
    clonedMsg = JSON.parse(JSON.stringify(msg));
  } catch (e) {
    clonedMsg = msg;
  }

  const payload = {
    msg: clonedMsg,
    timestamp: Date.now(),
  };

  // Index by raw ID and normalized clean ID
  messageStore.set(rawId, payload);
  if (cleanId !== rawId) {
    messageStore.set(cleanId, payload);
  }
}

/**
 * Deep Lookup Helper: Finds message by exact ID, clean ID, or substring match
 */
export function getStoredMessage(targetId) {
  if (!targetId) return null;

  // 1. Direct hit
  if (messageStore.has(targetId)) {
    return messageStore.get(targetId);
  }

  // 2. Clean ID hit
  const cleanTarget = targetId.replace(/[^a-zA-Z0-9]/g, "");
  if (messageStore.has(cleanTarget)) {
    return messageStore.get(cleanTarget);
  }

  // 3. Substring / Partial match scan across all cached keys
  for (const [cachedKey, data] of messageStore.entries()) {
    if (
      cachedKey.includes(targetId) ||
      targetId.includes(cachedKey) ||
      cachedKey.includes(cleanTarget)
    ) {
      return data;
    }
  }

  return null;
}

const CACHE_TTL_MS = 24 * 60 * 60 * 1000; // 24 hours

export function startCacheCleanupInterval(getIsSleeping) {
  setInterval(() => {
    if (getIsSleeping && getIsSleeping()) return;

    const now = Date.now();
    for (const [id, data] of messageStore.entries()) {
      if (now - data.timestamp > CACHE_TTL_MS) {
        messageStore.delete(id);
      }
    }
  }, 15 * 60 * 1000);
}