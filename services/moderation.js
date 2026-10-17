// services/moderation.js
const TIMEOUT_MS = 5000;

export async function checkImageSafety(imageUrl) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), TIMEOUT_MS);

  try {
    const res = await fetch("https://api.sightengine.com/1.0/check.json", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({
        url: imageUrl,
        models: "nudity-2.1",
        api_user: process.env.SIGHTENGINE_USER,
        api_secret: process.env.SIGHTENGINE_SECRET,
      }),
      signal: controller.signal,
    });

    const data = await res.json();
    const score = data?.nudity?.sexual_activity ?? data?.nudity?.raw ?? 0;

    return score < 0.5; // tune threshold to taste
  } finally {
    clearTimeout(timeout);
  }
}