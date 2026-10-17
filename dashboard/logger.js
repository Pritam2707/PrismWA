/**
 * Lightweight In-Memory Activity Logger with Server-Sent Events (SSE)
 * Designed for low-resource environments (Termux on Android, background daemons).
 * Uses a fixed circular buffer (max 150 entries) so memory never expands.
 */

const MAX_LOGS = 150;
const logBuffer = [];
const sseClients = new Set();

/**
 * Adds an event log to the circular buffer and broadcasts to active SSE dashboard clients
 * @param {string} type - 'MSG' | 'CMD' | 'AI' | 'SYSTEM' | 'ERROR' | 'SUCCESS'
 * @param {string} message - Descriptive text
 * @param {object} [meta] - Optional metadata (chat, sender, command, duration, etc.)
 */
export function logEvent(type, message, meta = null) {
  const entry = {
    id: Date.now() + "_" + Math.random().toString(36).substr(2, 4),
    timestamp: new Date().toISOString(),
    type: type.toUpperCase(),
    message,
    meta: meta || {},
  };

  logBuffer.push(entry);
  if (logBuffer.length > MAX_LOGS) {
    logBuffer.shift(); // Keep buffer strictly bounded to prevent memory growth
  }

  // Also print concisely to standard console
  const typeIcons = {
    MSG: "💬",
    CMD: "⚡",
    AI: "🧠",
    SYSTEM: "⚙️",
    ERROR: "❌",
    SUCCESS: "✅",
  };
  const icon = typeIcons[entry.type] || "📌";
  console.log(`${icon} [${entry.type}] ${message}`);

  // Broadcast to connected SSE dashboard web clients (if any)
  if (sseClients.size > 0) {
    const dataStr = `data: ${JSON.stringify(entry)}\n\n`;
    for (const client of sseClients) {
      try {
        client.write(dataStr);
      } catch (e) {
        sseClients.delete(client);
      }
    }
  }

  return entry;
}

/**
 * Returns recent logs snapshot
 */
export function getRecentLogs() {
  return [...logBuffer];
}

/**
 * Clears in-memory log buffer
 */
export function clearLogs() {
  logBuffer.length = 0;
}

/**
 * Registers an SSE response stream for live dashboard updates
 */
export function registerSseClient(res) {
  sseClients.add(res);
  res.on("close", () => {
    sseClients.delete(res);
  });
}
