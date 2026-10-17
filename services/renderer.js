import { existsSync } from "node:fs";
import mathjax from "mathjax-node";
import { GlobalFonts, createCanvas, loadImage } from "@napi-rs/canvas";
import sharp from "sharp";

if (existsSync("/data/data/com.termux/files/usr/share/fonts/TTF/DejaVuSans.ttf")) {
  GlobalFonts.registerFromPath(
    "/data/data/com.termux/files/usr/share/fonts/TTF/DejaVuSans.ttf",
    "DejaVuSans"
  );
}

mathjax.config({
  MathJax: {
    SVG: { font: "TeX", scale: 120 },
  },
});
mathjax.start();

const MATH_HINT_RE = /\$|\\[a-zA-Z]+\{|\\[a-zA-Z]+\s|\^\{|_\{|\\\[|\\\(|\\begin\{/;

export function containsMath(text) {
  return MATH_HINT_RE.test(text);
}

export function normalizeLatex(latex) {
  if (!latex) return "";
  return latex
    .replace(/\\FRAC/g, "\\frac")
    .replace(/\\CDOT/g, "\\cdot")
    .replace(/\\LEFT/g, "\\left")
    .replace(/\\RIGHT/g, "\\right")
    .replace(/\\IMPLIES/g, "\\implies")
    .replace(/\\TEXT/g, "\\text")
    .replace(/\\SQRT/g, "\\sqrt")
    .replace(/\\TIMES/g, "\\times")
    .replace(/\\INFTY/g, "\\infty");
}

export function latexToSvg(latex, displayMode = false) {
  return new Promise((resolve, reject) => {
    const fixedLatex = normalizeLatex(latex);
    mathjax.typeset(
      { math: fixedLatex, format: displayMode ? "TeX" : "inline-TeX", svg: true },
      (data) => {
        if (data.errors && data.errors.length > 0) return reject(data.errors);
        if (!data.svg) return reject(new Error("No SVG output from MathJax"));
        resolve(data.svg);
      }
    );
  });
}

export function sanitizeMathNotation(str) {
  if (!str) return str;
  return str
    .replace(/\$\$?/g, "")
    .replace(/\\\(|\\\)|\\\[|\\\]/g, "")
    .replace(/\\d?frac\{([^{}]+)\}\{([^{}]+)\}/gi, "($1)/($2)")
    .replace(/\\sqrt\{([^{}]+)\}/gi, "√($1)")
    .replace(/\\sqrt/gi, "√")
    .replace(/\\times/gi, "×")
    .replace(/\\cdot/gi, "·")
    .replace(/\\div/gi, "÷")
    .replace(/\\pm/gi, "±")
    .replace(/\\infty/gi, "∞")
    .replace(/\\le(q)?/gi, "≤")
    .replace(/\\ge(q)?/gi, "≥")
    .replace(/\\neq/gi, "≠")
    .replace(/\\approx/gi, "≈")
    .replace(/\\(rightarrow|to)/gi, "→")
    .replace(/\\int/gi, "∫")
    .replace(/\\sum/gi, "∑")
    .replace(/\\prod/gi, "∏")
    .replace(/\\partial/gi, "∂")
    .replace(/\\alpha/gi, "α")
    .replace(/\\beta/gi, "β")
    .replace(/\\gamma/gi, "γ")
    .replace(/\\[Dd]elta/g, (m) => (m[1] === "D" ? "Δ" : "δ"))
    .replace(/\\theta/gi, "θ")
    .replace(/\\lambda/gi, "λ")
    .replace(/\\mu/gi, "μ")
    .replace(/\\pi/gi, "π")
    .replace(/\\sigma/gi, "σ")
    .replace(/\\omega/gi, "ω")
    .replace(/\^\{?2\}?\b/g, "²")
    .replace(/\^\{?3\}?\b/g, "³")
    .replace(/\^\{([^{}]+)\}/g, "^($1)")
    .replace(/_\{([^{}]+)\}/g, "_($1)")
    .replace(/\\(left|right)/gi, "")
    .replace(/\\[,;!]/g, " ")
    .replace(/\\([a-zA-Z]+)/g, "$1")
    .replace(/[ \t]{2,}/g, " ")
    .trim();
}

// ---------------------------------------------------------------------------
// Rendered-math cache — avoids re-running MathJax + sharp + canvas decode
// for formulas that repeat across messages (very common: same identity,
// same standard formula, etc). Simple FIFO cap to avoid unbounded growth.
// ---------------------------------------------------------------------------
const MATH_RENDER_CACHE_MAX = 300;
const mathRenderCache = new Map(); // cacheKey -> { img, width, height }

function getCachedMathRender(key) {
  return mathRenderCache.get(key);
}

function setCachedMathRender(key, value) {
  if (mathRenderCache.size >= MATH_RENDER_CACHE_MAX) {
    const oldestKey = mathRenderCache.keys().next().value;
    mathRenderCache.delete(oldestKey);
  }
  mathRenderCache.set(key, value);
}

// ---------------------------------------------------------------------------
// Bounded-concurrency map helper. mathjax-node (MathJax v2 under the hood)
// isn't guaranteed reentrant-safe under fully unbounded parallel typeset()
// calls, so we cap concurrency instead of firing everything at once with
// Promise.all — still a big speedup over fully sequential rendering.
// ---------------------------------------------------------------------------
const MATH_RENDER_CONCURRENCY = 4;

async function mapWithConcurrency(items, limit, fn) {
  const results = new Array(items.length);
  let nextIndex = 0;

  async function worker() {
    while (nextIndex < items.length) {
      const current = nextIndex++;
      results[current] = await fn(items[current], current);
    }
  }

  const workers = Array.from({ length: Math.min(limit, items.length) }, worker);
  await Promise.all(workers);
  return results;
}

export function extractCleanLatex(token) {
  let s = (token || "").trim();
  if (s.startsWith("$$") && s.endsWith("$$") && s.length >= 4) {
    return s.slice(2, -2).trim();
  }
  if (s.startsWith("\\[") && s.endsWith("\\]") && s.length >= 4) {
    return s.slice(2, -2).trim();
  }
  if (s.startsWith("\\(") && s.endsWith("\\)") && s.length >= 4) {
    return s.slice(2, -2).trim();
  }
  if (s.startsWith("$") && s.endsWith("$") && s.length >= 2) {
    return s.slice(1, -1).trim();
  }
  return s;
}

const TOKEN_REGEX = /(\$\$[\s\S]*?\$\$|\\\[[\s\S]*?\\\]|\\begin\{[a-z\*]+\}[\s\S]*?\\end\{[a-z\*]+\}|\$[^\$\n]+?\$|\\\([\s\S]*?\\\)|\*[^\*\n]+?\*)/gi;

function isDisplayMathToken(token) {
  const s = (token || "").trim();
  return (s.startsWith("$$") && s.endsWith("$$")) ||
         (s.startsWith("\\[") && s.endsWith("\\]")) ||
         (s.startsWith("\\begin{") && s.includes("\\end{"));
}

function isInlineMathToken(token) {
  const s = (token || "").trim();
  return ((s.startsWith("$") && s.endsWith("$")) ||
          (s.startsWith("\\(") && s.endsWith("\\)"))) && !isDisplayMathToken(s);
}

function mathCacheKey(cleanLatex, displayMode) {
  return `${displayMode ? "D" : "I"}::${cleanLatex}`;
}

export async function renderAnswerMedia(text) {
  const padding = 32;
  const maxWidth = 700;
  const fontSize = 20;
  const lineHeight = 34;
  const fontStyle = "DejaVuSans, Arial, sans-serif";

  const measureCanvas = createCanvas(maxWidth + padding * 2, 100);
  const measureCtx = measureCanvas.getContext("2d");

  const lines = text.split("\n");

  // -------------------------------------------------------------------
  // PHASE 1: Tokenize every line up front (pure, synchronous, cheap) and
  // collect the *unique* math expressions that need rendering.
  // -------------------------------------------------------------------
  const parsedLines = lines.map((line) => {
    if (!line.trim()) return { blank: true, tokens: [] };
    return { blank: false, tokens: line.split(TOKEN_REGEX).filter(Boolean) };
  });

  const mathJobs = new Map(); // cacheKey -> { latex, displayMode }
  for (const { blank, tokens } of parsedLines) {
    if (blank) continue;
    for (const token of tokens) {
      const displayMode = isDisplayMathToken(token);
      const inline = isInlineMathToken(token);
      if (!displayMode && !inline) continue;

      const cleanLatex = extractCleanLatex(token);
      const key = mathCacheKey(cleanLatex, displayMode);
      if (!mathJobs.has(key)) {
        mathJobs.set(key, { latex: cleanLatex, displayMode });
      }
    }
  }

  // -------------------------------------------------------------------
  // PHASE 2: Render all unique math expressions (cache-first, then
  // bounded-concurrency rendering for cache misses).
  // -------------------------------------------------------------------
  const renderedMath = new Map(); // cacheKey -> { img, width, height } | { error, fallbackText }
  const jobEntries = Array.from(mathJobs.entries());

  await mapWithConcurrency(jobEntries, MATH_RENDER_CONCURRENCY, async ([key, { latex, displayMode }]) => {
    const cached = getCachedMathRender(key);
    if (cached) {
      renderedMath.set(key, cached);
      return;
    }

    try {
      const svgString = await latexToSvg(latex, displayMode);
      const pngBuffer = await sharp(Buffer.from(svgString)).png().toBuffer();
      const img = await loadImage(pngBuffer);
      const result = { img, width: img.width, height: img.height };
      renderedMath.set(key, result);
      setCachedMathRender(key, result);
    } catch (e) {
      renderedMath.set(key, { error: true, fallbackText: sanitizeMathNotation(latex) });
      // not cached — a transient MathJax failure shouldn't get permanently
      // baked in as "this formula always fails"
    }
  });

  // -------------------------------------------------------------------
  // PHASE 3: Layout pass — fully synchronous now, just reading from the
  // renderedMath map instead of awaiting per token.
  // -------------------------------------------------------------------
  const renderQueue = [];
  let currentY = padding + fontSize;

  for (const { blank, tokens } of parsedLines) {
    if (blank) {
      currentY += lineHeight / 2;
      continue;
    }

    let currentX = padding;

    for (const token of tokens) {
      const displayMode = isDisplayMathToken(token);
      const inline = isInlineMathToken(token);
      const isBold = token.startsWith("*") && token.endsWith("*") && token.length > 2;

      if (displayMode || inline) {
        const cleanLatex = extractCleanLatex(token);
        const key = mathCacheKey(cleanLatex, displayMode);
        const rendered = renderedMath.get(key);

        if (rendered && !rendered.error) {
          const { img, width, height } = rendered;
          const targetHeight = displayMode
            ? Math.min(height, 80)
            : Math.max(height, fontSize * 1.1);
          const targetWidth = (width / height) * targetHeight;

          if (displayMode) {
            currentX = padding;
            renderQueue.push({
              type: "image", img, x: (maxWidth + padding * 2 - targetWidth) / 2,
              y: currentY, w: targetWidth, h: targetHeight,
            });
            currentY += targetHeight + 16;
            currentX = padding;
          } else {
            if (currentX + targetWidth > maxWidth + padding) {
              currentX = padding;
              currentY += lineHeight;
            }
            renderQueue.push({
              type: "image", img, x: currentX, y: currentY - fontSize * 0.75,
              w: targetWidth, h: targetHeight,
            });
            currentX += targetWidth + 4;
          }
        } else {
          const readableFallback = rendered ? rendered.fallbackText : sanitizeMathNotation(cleanLatex);
          measureCtx.font = `${fontSize}px ${fontStyle}`;
          const wordWidth = measureCtx.measureText(readableFallback).width;
          if (currentX + wordWidth > maxWidth + padding) {
            currentX = padding;
            currentY += lineHeight;
          }
          renderQueue.push({ type: "text", text: readableFallback, x: currentX, y: currentY, isBold: false });
          currentX += wordWidth;
        }
      } else {
        let content = token;
        let boldFlag = false;
        if (isBold) {
          content = token.slice(1, -1);
          boldFlag = true;
        }

        measureCtx.font = boldFlag ? `bold ${fontSize}px ${fontStyle}` : `${fontSize}px ${fontStyle}`;
        const words = content.split(" ");
        for (let i = 0; i < words.length; i++) {
          const word = words[i] + (i === words.length - 1 ? "" : " ");
          const wordWidth = measureCtx.measureText(word).width;
          if (currentX + wordWidth > maxWidth + padding) {
            currentX = padding;
            currentY += lineHeight;
          }
          renderQueue.push({ type: "text", text: word, x: currentX, y: currentY, isBold: boldFlag });
          currentX += wordWidth;
        }
      }
    }
    currentY += lineHeight;
  }

  const canvasHeight = Math.max(currentY + padding, 120);
  const canvas = createCanvas(maxWidth + padding * 2, canvasHeight);
  const ctx = canvas.getContext("2d");

  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  for (const item of renderQueue) {
    if (item.type === "image") {
      ctx.drawImage(item.img, item.x, item.y, item.w, item.h);
    } else if (item.type === "text") {
      ctx.fillStyle = "#111827";
      ctx.font = item.isBold ? `bold ${fontSize}px ${fontStyle}` : `${fontSize}px ${fontStyle}`;
      ctx.fillText(item.text, item.x, item.y);
    }
  }

  return { buffer: canvas.toBuffer("image/png"), mimetype: "image/png", kind: "image", ext: "png" };
}