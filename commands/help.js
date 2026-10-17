/**
 * Streamlined Help & Command Dashboard for PrismBot
 * Supports category sub-menus: !help ai, !help media, !help tools, !help games, !help admin, !help all
 */
export async function handleHelpCommand(sock, jid, msg, rawText = "") {
  try {
    const args = (rawText || "").trim().toLowerCase().split(/\s+/);
    const category = args[1] || "";

    // 1. AI & Personas Category (!help ai)
    if (category === "ai" || category === "persona" || category === "voice") {
      const text =
`╭━━━ 🤖 *AI & PERSONAS* ━━━╮

*🎭 15+ Character Personas:*
• \`!zero2\` | \`!kurumi\` | \`!makima\` | \`!mai\` | \`!ganyu\`
• \`!johan\` | \`!light\` | \`!naruto\` | \`!gojo\` | \`!levi\`
• \`!dazai\` | \`!ayankoji\` | \`!itachi\` | \`!l\` | \`!sukuna\`
_💡 Usage: \`!zero2 Hello Darling!\`_

*🎙️ Voice Conversion & RVC:*
• \`!vc <model> [preset]\` — Convert replied voice note
• \`!vc <model> [preset] song\` — UVR vocal separation
• \`!vc <model> -play <link/query>\` — Social/YouTube cover
• \`!vc q\` | \`!vc status <num>\` | \`!vc get <num>\`
• \`!tts <voice> <text>\` — Text-to-speech (\`!voices\` for list)

*🎨 AI Image Generation:*
• \`!gen <prompt>\` — Realistic / digital art
• \`!anime <prompt>\` — High-detail anime art

╰──────────────────────────╯`;
      await sock.sendMessage(jid, { text }, { quoted: msg });
      return true;
    }

    // 2. Media & Downloader Category (!help media)
    if (category === "media" || category === "dl" || category === "sticker") {
      const text =
`╭━━━ 🎨 *MEDIA & DOWNLOADER* ━━━╮

*✨ Stickers & Utilities:*
• \`!s\` / \`!sticker\` — Make sticker (Reply/Attach image/video)
• \`!toimg\` — Convert sticker back to standard image
• \`!togif\` / \`!tovideo\` — Convert animated sticker to GIF/video
• \`!toaudio\` / \`!tomp3\` — Extract audio from video as MP3
• \`!meme <top> | <bottom>\` — Create classic meme sticker

*📥 Universal Media Downloader:*
• \`!play <song name / URL>\` / \`!song <query>\` — High-quality MP3
• \`!video <link>\` / \`!ig <link>\` / \`!reddit <link>\` / \`!tiktok <link>\` — Download video
• \`!yt <query>\` — Search YouTube top 5 results
• \`!img <query>\` — Search high-res Google images

╰──────────────────────────────╯`;
      await sock.sendMessage(jid, { text }, { quoted: msg });
      return true;
    }

    // 3. Tools & Utilities Category (!help tools)
    if (category === "tools" || category === "util" || category === "utility") {
      const text =
`╭━━━ 🛠️ *SUPERPOWER TOOLS* ━━━╮

*📄 AI OCR & Summarizer:*
• \`!ocr\` / \`!read\` — Extract text from handwriting, docs & photos
• \`!sum\` / \`!summarize\` — 3-bullet executive summary of replied text

*🌐 Translation & Language:*
• \`!tr <language> [text]\` — Accurate multi-language translation (\`!tr hi\`, \`!tr es\`, \`!tr ja\`)
• \`!define <word>\` — Dictionary definition, pronunciation & examples

*📊 Native Polls & Group Tools:*
• \`!poll "Question" "Opt 1" "Opt 2" ...\` — WhatsApp Interactive Voting Poll
• \`!tagall <msg>\` — Mention all group members (Admin only)

*🧮 Math, Conversions & Live Weather:*
• \`!calc <math/units>\` — AI scientific calculator & currency conversion
• \`!weather <city>\` — Live weather forecast card
• \`!quote\` | \`!fact\` — Daily motivation or verified mind-blowing facts

*🗑️ Recovery & Notes:*
• \`!vv\` — Restore latest View-Once disappearing media
• \`!notes\` | \`!addnote <path> <text>\`
• \`!remind <10m|1h> <message>\` | \`!reminders\`
• \`!qr <text/url>\` — Generate QR code

╰──────────────────────────────╯`;
      await sock.sendMessage(jid, { text }, { quoted: msg });
      return true;
    }

    // 4. Admin Category (!help admin)
    if (category === "admin" || category === "owner") {
      const text =
`╭━━━ ⚙️ *ADMIN & MODERATION* ━━━╮

*🤖 Away & Auto-Reply:*
• \`!autoreply on [global|target|keyword]\`
• \`!autoreply off\` — Disable & view visitor summary
• \`!autoreply addtrigger <word> | <reply>\`

*👋 Group Welcomer:*
• \`!welcome on/off\` | \`!welcome set <text>\`
• \`!welcome setimage\` (Reply to image/GIF)

*🤫 Moderation & System (Owner):*
• \`!mute @user\` / \`!unmute @user\` / \`!mutelist\`
• \`!tagall <announcement>\` — Admin tag all members
• \`!antidelete\` — Toggle anti-delete logging
• \`!features on/off\` — Toggle interactive features
• \`!storage\` / \`!storage clear media\`
• \`!wake\` / \`!sleep\` / \`!botstatus\`

╰──────────────────────────────╯`;
      await sock.sendMessage(jid, { text }, { quoted: msg });
      return true;
    }

    // 5. Full Monolithic View (!help all)
    if (category === "all" || category === "full") {
      const text =
`╭━━━ ✨ *PRISMBOT FULL COMMAND DIRECTORY* ✨ ━━━╮

*🤖 AI & VOICES*
• \`!zero2\` to \`!sukuna\` (15 personas)
• \`!vc <model> [preset] [-play <url>]\`
• \`!gen <prompt>\` | \`!anime <prompt>\`
• \`!tts <voice> <text>\` | \`!voices\`

*🛠️ SUPERPOWER UTILITIES*
• \`!ocr\` / \`!read\` | \`!sum\` / \`!summarize\`
• \`!tr <lang> [text]\` | \`!define <word>\`
• \`!poll "Q" "Opt 1" "Opt 2"\` | \`!tagall <msg>\`
• \`!calc <expr>\` | \`!weather <city>\` | \`!quote\` | \`!fact\`

*🎨 MEDIA & DOWNLOADS*
• \`!s\` / \`!sticker\` | \`!toimg\` | \`!togif\` | \`!toaudio\` | \`!meme\`
• \`!play <query/URL>\` | \`!song <query>\` | \`!video <link>\` | \`!yt\` | \`!img\`
• \`!vv\` | \`!deleted\` | \`!notes\` | \`!remind <time> <msg>\` | \`!qr\`

*⚙️ ADMIN & AWAY*
• \`!autoreply\` | \`!welcome\` | \`!mute\` | \`!storage\` | \`!features\`

🌐 *Web Dashboard:* http://localhost:3000
╰───────────────────────────────────────────────╯`;
      await sock.sendMessage(jid, { text }, { quoted: msg });
      return true;
    }

    // Default Compact Menu (!help / !menu)
    const helpMenu =
`╭━━━ ✨ *PRISMBOT COMMAND MENU* ✨ ━━━╮

👉 *Quick Category Sub-Menus:*
• \`!help tools\`   — OCR, Summarizer, Translator, Polls, Calc, Weather
• \`!help media\`   — Stickers, Songs, Videos, Memes & Downloader
• \`!help ai\`      — 15+ AI Personas, Voice Cloning & Image Gen
• \`!help admin\`   — Auto-Reply, Mute, Welcomer, TagAll & Controls
• \`!help all\`     — View entire command directory

🌐 *Web Dashboard:* \`http://localhost:3000\`
_💡 Tip: Type \`!help <category>\` (e.g. \`!help tools\`)_
╰──────────────────────────────────────╯`;

    await sock.sendMessage(
      jid,
      { text: helpMenu },
      { quoted: msg }
    );
    return true;
  } catch (err) {
    console.error("Error sending Help text message:", err);
  }
}