import { GoogleGenAI, Type } from "@google/genai";

const ai = new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY });

const ALL_VOICE_KEYS = [
  "zero2_jp", "kurumi_en", "kurumi_jp", "goku_en", "elon_musk", 
  "naruto_jp", "naruto_en", "paimon_en", "kratos", "snoop_dog", 
  "eminem", "joshua_graham", "rus_en", "simon_riley", "johan_en", 
  "light_en", "l_en", "lara_croft", "marin_en", "rem_en", "modi", 
  "trump", "miku", "sharukh_khan", "amitabh_bachchan", "columnbina", 
  "sandrone", "furina", "venti", "zhongli", "mavuika", "ankit", "sreyan"
];

/**
 * Universal Intent Classifier for PrismBot
 */
export async function parseUniversalCommand(
  userMessage,
  currentPath,
  isAutoReplyActive
) {
  const systemInstruction = `
You are the natural language brain for PrismBot on WhatsApp.
Your task is to translate plain user text into a structured command JSON.

Environment Context:
• Working Directory: "${currentPath}"
• Auto-Reply Status: ${isAutoReplyActive ? "ACTIVE" : "INACTIVE"}

SUPPORTED ACTIONS & PARAMETERS:

1. PERSONAS (AI Characters):
   - "persona": Triggered whenever the user asks to speak to, roleplay with, act like, or imitate a character persona.
     "character": Extract character name key in lowercase: "zero2" | "kurumi" | "makima" | "mai" | "ganyu" | "johan" | "light" | "naruto" | "gojo" | "levi" | "dazai" | "ayankoji" | "itachi" | "l" | "sukuna".
     "prompt": The message to send to the character. If user only says "speak like gojo" without a question, set prompt to a default greeting like "Hello!".

2. STICKERS & MEDIA CREATION:
   - "sticker": Make sticker from attached/quoted media.
   - "toimg": Convert sticker back to standard image.
   - "tovideo": Convert animated sticker to video or GIF.
   - "gen_image": Generate AI image. "prompt": image description.

3. DOWNLOADER & MEDIA:
   - "download_audio": Download song/MP3. "query": title or link, "url": URL if provided.
   - "download_video": Download video/MP4. "query": title or link, "url": URL if provided.
   - "search_yt": Search YouTube. "query": search keywords.
   - "search_img": Search pictures/photos. "query": search keywords.
   - "download_social": Download post/video from Instagram, TikTok, Reddit, etc. "url": media link.

4. NOTES & REMINDERS:
   - "notes": View/reset file system dashboard.
   - "cd": Change folder. "target": folder name, number index, or "..".
   - "addnote": Save note or file. "path": relative target path, "content": text content.
   - "getnote": Retrieve note file. "target": number index or relative path.
   - "remind": Set reminder. "time": duration (e.g. "10m", "2h"), "text": reminder text.

5. RECOVERY & VIEW-ONCE:
   - "vv_get": View or list View-Once media. "target": index number or "list".
   - "deleted_get": View or list deleted messages. "target": index number or "list".

6. CHAT STATS & RANKINGS:
   - "stats": Show group message statistics and leaderboard.
   - "mystats": Show user's personal activity card.

7. AUTO-REPLY / AWAY MODE:
   - "autoreply_on": Turn on auto-reply away mode. "text": custom away message.
   - "autoreply_off": Turn off auto-reply away mode and get summary digest.
   - "autoreply_status": Check current auto-reply status.

8. UTILITIES & ADMIN TOOLS:
   - "welcome": Welcome settings manager. "target" or "text": settings command or custom greeting text.
   - "storage": Storage and disk space manager. "target": "clear", "media", "stickers", or "all".
   - "mute": Mute a user. "target": user mention or phone number.
   - "unmute": Unmute a user. "target": user mention or phone number.
   - "qrcode": Generate QR code. "text" or "query": data string to encode.
   - "tts": Generate Text-To-Speech audio.
     "text": The EXACT speech content to read out loud.
             RULES FOR GENERATING "text":
             1. IF user provides explicit text to read (e.g., 'say hello world'), put "hello world" here.
             2. NEVER output literal filler phrases like "something", "a speech", "anything", or "tell me something".
             3. IF the user makes a vague or open request (e.g., 'tell me something', 'generate something', 'tell me a story', 'say something funny'), USE YOUR INTELLIGENCE to compose a creative, interesting speech script right here!
             4. IF the user asks a question or requests advice (e.g., 'explain how to solve irodov', 'motivate me for jee', 'convince me to crack GATE'), WRITE THE ACTUAL ANSWER OR MOTIVATIONAL SCRIPT directly here!
             5. DO NOT include markdown formatting like asterisks (*), underscores (_), or hash symbols (#).

     "voice_key": Match any requested voice, accent, or character to the CLOSEST matching key from the voice_key ENUM list.
     VOICE MAPPING EXAMPLES:
     - "in sreyan voice" -> "sreyan"
     - "in zero2_jp voice" -> "zero2_jp"
     - "in elon musk voice" -> "elon_musk"
     - "in modi voice" -> "modi"
     - "in trump voice" -> "trump"
     - "in goku voice" -> "goku_en"
     - If no specific voice is mentioned in the request, leave voice_key null.

   - "help": Open help menu or dashboard.
   - "system": Check system status, uptime, wake, or sleep. "target": "botstatus", "uptime", "wake", "sleep", "antidelete".

9. CHAT & FALLBACK:
   - "chat": User is directly conversing with or asking a general question to the bot. Provide answer in "replyText".
   - "none": Background human chatter between group members not intended for the bot.

Respond STRICTLY with valid JSON adhering to the provided schema.
`;

  try {
    const response = await ai.models.generateContent({
      model: "gemini-3.5-flash-lite",
      contents: userMessage,
      config: {
        systemInstruction,
        responseMimeType: "application/json",
        responseSchema: {
          type: Type.OBJECT,
          properties: {
            action: {
              type: Type.STRING,
              enum: [
                "persona",
                "sticker",
                "toimg",
                "tovideo",
                "gen_image",
                "download_audio",
                "download_video",
                "search_yt",
                "search_img",
                "download_social",
                "notes",
                "cd",
                "addnote",
                "getnote",
                "remind",
                "vv_get",
                "deleted_get",
                "stats",
                "mystats",
                "autoreply_on",
                "autoreply_off",
                "autoreply_status",
                "welcome",
                "storage",
                "mute",
                "unmute",
                "qrcode",
                "tts",
                "help",
                "system",
                "chat",
                "none",
              ],
            },
            character: { type: Type.STRING },
            prompt: { type: Type.STRING },
            query: { type: Type.STRING },
            url: { type: Type.STRING },
            target: { type: Type.STRING },
            path: { type: Type.STRING },
            content: { type: Type.STRING },
            time: { type: Type.STRING },
            text: { type: Type.STRING },
            voice_key: { 
              type: Type.STRING, 
              enum: ALL_VOICE_KEYS,
              description: "Matched voice key from allowed VOICE_IDS"
            },
            replyText: { type: Type.STRING },
          },
          required: ["action"],
        },
      },
    });

    return JSON.parse(response.text);
  } catch (err) {
    console.error("Gemini Router Error:", err);
    return { action: "none" };
  }
}