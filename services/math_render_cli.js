import fs from "node:fs";
import { renderAnswerMedia } from "./renderer.js";

async function main() {
  const args = process.argv.slice(2);
  if (args.length < 2) {
    console.error("Usage: node math_render_cli.js <inputFile> <outputFile>");
    process.exit(1);
  }

  const inputFile = args[0];
  const outputFile = args[1];

  try {
    const text = fs.readFileSync(inputFile, "utf8");
    const res = await renderAnswerMedia(text);
    fs.writeFileSync(outputFile, res.buffer);
    process.exit(0);
  } catch (err) {
    console.error("Render error:", err);
    process.exit(1);
  }
}

main();
