import { INACTIVITY_SLEEP_MS } from "../config/constants.js";

let isSleeping = false;
let inactivityTimer = null;
let lastActivityTime = Date.now();

/**
 * Resets the automatic sleep timer.
 * Call this whenever valid bot commands or active interactions occur.
 */
export function resetInactivityTimer() {
  lastActivityTime = Date.now();

  if (inactivityTimer) clearTimeout(inactivityTimer);

  inactivityTimer = setTimeout(() => {
    enterSleepMode();
  }, INACTIVITY_SLEEP_MS);
}

/**
 * Manually or automatically puts the bot into sleep mode
 */
export function enterSleepMode() {
  isSleeping = true;
  if (inactivityTimer) clearTimeout(inactivityTimer);
  console.log("💤 [PrisamBot] Entered Sleep Mode. Inactive operations paused.");
}

/**
 * Manually or automatically wakes up the bot
 */
export function wakeUpBot() {
  const wasSleeping = isSleeping;
  isSleeping = false;
  resetInactivityTimer();
  console.log("⏰ [PrismBot] Woke up! Activity timer reset.");
  return wasSleeping;
}

/**
 * Check if the bot is currently sleeping
 */
export function getIsSleeping() {
  return isSleeping;
}

/**
 * Get timestamp of last active message
 */
export function getLastActivityTime() {
  return lastActivityTime;
}