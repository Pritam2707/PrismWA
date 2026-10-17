import QRCode from "qrcode";

/**
 * Commands: !qr <text/url> or reply to a message with !qr
 */
export async function handleQrCommand(sock, jid, rawText, msg) {
  const text = rawText.trim();
  const lowerText = text.toLowerCase();

  if (!lowerText.startsWith("!qr")) return false;

  const args = text.split(/\s+/).slice(1).join(" ");
  const quotedText = msg.message?.extendedTextMessage?.contextInfo?.quotedMessage?.conversation ||
                     msg.message?.extendedTextMessage?.contextInfo?.quotedMessage?.extendedTextMessage?.text;

  const qrContent = args || quotedText;

  if (!qrContent) {
    await sock.sendMessage(
      jid,
      { text: "⚠️ *Usage:* `!qr <text or link>` or reply to a text message with `!qr`" },
      { quoted: msg }
    );
    return true;
  }

  try {
    // Generate QR Code Buffer
    const qrBuffer = await QRCode.toBuffer(qrContent, {
      type: "png",
      width: 400,
      margin: 2,
      color: {
        dark: "#0F172A", // Dark Slate
        light: "#FFFFFF",
      },
    });

    await sock.sendMessage(
      jid,
      {
        image: qrBuffer,
        caption: `📲 *QR Code Generated!*\n\n*Content:* \`${qrContent.length > 60 ? qrContent.substring(0, 60) + "..." : qrContent}\``,
      },
      { quoted: msg }
    );
  } catch (err) {
    console.error("QR Generation Error:", err);
    await sock.sendMessage(jid, { text: "❌ Failed to generate QR Code." }, { quoted: msg });
  }

  return true;
}