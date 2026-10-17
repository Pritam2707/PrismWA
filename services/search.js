import { SERPER_API_KEY } from "../config/constants.js";

const SERPER_BASE_URL = "https://google.serper.dev";
const DEFAULT_TIMEOUT_MS = 6000;

// Shared HTTP headers object (reused across requests to minimize GC)
const HEADERS = Object.freeze({
  "X-API-KEY": SERPER_API_KEY,
  "Content-Type": "application/json",
});

/**
 * Shared, robust fetch helper with built-in AbortController timeout.
 */
async function postSerper(endpoint, bodyData) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT_MS);

  try {
    const res = await fetch(`${SERPER_BASE_URL}${endpoint}`, {
      method: "POST",
      headers: HEADERS,
      body: JSON.stringify(bodyData),
      signal: controller.signal,
    });

    if (!res.ok) {
      console.warn(`⚠️ [Serper API] HTTP ${res.status} on ${endpoint}`);
      return null;
    }

    return await res.json();
  } catch (err) {
    if (err.name === "AbortError") {
      console.error(`⏱️ [Serper API] Request timed out on ${endpoint}`);
    } else {
      console.error(`❌ [Serper API Error]: ${err.message}`);
    }
    return null;
  } finally {
    clearTimeout(timer);
  }
}

/**
 * High-performance text search handler with direct format pipeline.
 */
export async function getFreeSearchResults(query) {
  if (!query?.trim()) return null;

  console.log(`📡 [Serper Text Search] Querying: "${query}"...`);

  const data = await postSerper("/search", { q: query, num: 4 });
  if (!data) return null;

  const sections = [];

  // 1. Direct Answer Box
  if (data.answerBox) {
    const abText = data.answerBox.answer || data.answerBox.snippet || data.answerBox.title;
    if (abText) sections.push(`[Direct Answer]: ${abText}`);
  }

  // 2. Knowledge Graph
  if (data.knowledgeGraph) {
    const kg = data.knowledgeGraph;
    const kgText = kg.description || kg.title;
    if (kgText) sections.push(`[Knowledge Graph]: ${kg.title || query} — ${kgText}`);
  }

  // 3. Organic Web Results
  if (Array.isArray(data.organic) && data.organic.length > 0) {
    for (let i = 0; i < data.organic.length; i++) {
      const r = data.organic[i];
      sections.push(
        `[Source ${i + 1}]: ${r.title}\nSnippet: ${r.snippet || "N/A"}\nURL: ${r.link}`
      );
    }
  }

  if (sections.length === 0) {
    console.warn(`⚠️ [Serper Text Search] 0 usable sections for: "${query}"`);
    return null;
  }

  console.log(`✅ [Serper Text Search] Retrieved ${sections.length} section(s).`);
  return sections.join("\n\n");
}

/**
 * Image search with fallback scanner.
 */
export async function getFreeImageSearch(query) {
  if (!query?.trim()) return null;

  console.log(`🖼️ [Serper Image Search] Querying: "${query}"...`);

  const data = await postSerper("/images", { q: query, num: 5 });
  if (!data || !Array.isArray(data.images) || data.images.length === 0) {
    console.warn(`⚠️ [Serper Image Search] 0 image results for: "${query}"`);
    return null;
  }

  // Scan for the first result containing a valid image URL
  for (const img of data.images) {
    if (img.imageUrl) {
      console.log(`✅ [Serper Image Search] Found image URL: ${img.imageUrl}`);
      return img.imageUrl;
    }
  }

  console.warn(`⚠️ [Serper Image Search] No valid imageUrl found in payload for: "${query}"`);
  return null;
}