package renderer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	parsedBoldFace    *opentype.Font
	parsedRegularFace *opentype.Font
)

func init() {
	parsedBoldFace, _ = opentype.Parse(gobold.TTF)
	parsedRegularFace, _ = opentype.Parse(goregular.TTF)
}

var (
	mathHintRegex = regexp.MustCompile(`\$|\\[a-zA-Z]+\{|\\[a-zA-Z]+\s|\^\{|_\{|\\\[|\\\(|\\begin\{|\\frac|\\sqrt|\\sum|\\int|\\prod|\\lim|\\alpha|\\beta|\\theta|\\infty`)

	reDisplayBlock  = regexp.MustCompile(`\\\[([\s\S]*?)\\\]`)
	reDisplayDollar = regexp.MustCompile(`\$\$([\s\S]*?)\$\$`)
	reBeginEnv      = regexp.MustCompile(`(?s)\\begin\{[a-zA-Z\*]+\}.*?\\end\{[a-zA-Z\*]+\}`)
	reInlineParen   = regexp.MustCompile(`\\\(([\s\S]*?)\\\)`)
	reInlineDollar  = regexp.MustCompile(`\$([^\$\n]+?)\$`)
	reNumberedStep  = regexp.MustCompile(`^(\d+[\.\)])\s*(.*)$`)
)

// ContainsMath returns true if the text contains LaTeX or math notation tokens.
func ContainsMath(text string) bool {
	return mathHintRegex.MatchString(text)
}

// ExtractFormulas extracts and composes LaTeX formulas from text into a cleanly renderable TeX expression.
func ExtractFormulas(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}

	// 1. Check if the whole string is already a standalone LaTeX environment
	if strings.HasPrefix(trimmed, `\begin{`) && strings.Contains(trimmed, `\end{`) {
		return trimmed
	}

	var formulas []string

	// 2. Extract block environments (\begin{aligned} ... \end{aligned})
	envMatches := reBeginEnv.FindAllString(trimmed, -1)
	for _, m := range envMatches {
		s := strings.TrimSpace(m)
		if s != "" {
			formulas = append(formulas, s)
		}
	}

	// 3. Extract display blocks \[ ... \]
	displayMatches := reDisplayBlock.FindAllStringSubmatch(trimmed, -1)
	for _, m := range displayMatches {
		if len(m) > 1 {
			s := strings.TrimSpace(m[1])
			if s != "" {
				formulas = append(formulas, s)
			}
		}
	}

	// 4. Extract display blocks $$ ... $$
	dollarMatches := reDisplayDollar.FindAllStringSubmatch(trimmed, -1)
	for _, m := range dollarMatches {
		if len(m) > 1 {
			s := strings.TrimSpace(m[1])
			if s != "" {
				formulas = append(formulas, s)
			}
		}
	}

	// 5. If no display formulas, extract inline formulas \( ... \) or $ ... $
	if len(formulas) == 0 {
		inlineParen := reInlineParen.FindAllStringSubmatch(trimmed, -1)
		for _, m := range inlineParen {
			if len(m) > 1 {
				s := strings.TrimSpace(m[1])
				if s != "" {
					formulas = append(formulas, s)
				}
			}
		}
		inlineDollar := reInlineDollar.FindAllStringSubmatch(trimmed, -1)
		for _, m := range inlineDollar {
			if len(m) > 1 {
				s := strings.TrimSpace(m[1])
				if s != "" {
					formulas = append(formulas, s)
				}
			}
		}
	}

	// 6. If still empty, but has math hints, use trimmed text
	if len(formulas) == 0 {
		return trimmed
	}

	// Deduplicate
	uniqueFormulas := make([]string, 0, len(formulas))
	seen := make(map[string]bool)
	for _, f := range formulas {
		if !seen[f] {
			seen[f] = true
			uniqueFormulas = append(uniqueFormulas, f)
		}
	}

	if len(uniqueFormulas) == 1 {
		return uniqueFormulas[0]
	}

	// Combine multiple formulas into aligned display block
	var sb strings.Builder
	sb.WriteString("\\begin{aligned}\n")
	for i, f := range uniqueFormulas {
		clean := strings.TrimSpace(f)
		clean = strings.TrimPrefix(clean, `\begin{aligned}`)
		clean = strings.TrimSuffix(clean, `\end{aligned}`)
		clean = strings.TrimSpace(clean)
		if !strings.HasPrefix(clean, "&") {
			sb.WriteString("& ")
		}
		sb.WriteString(clean)
		if i < len(uniqueFormulas)-1 {
			sb.WriteString(" \\\\[12pt]\n")
		} else {
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\\end{aligned}")
	return sb.String()
}

var reInline = regexp.MustCompile(`\$([^\$]+)\$|\\\(([^\)]+)\\\)`)

func escapeLatexText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\textbackslash `)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	s = strings.ReplaceAll(s, `&`, `\&`)
	s = strings.ReplaceAll(s, `#`, `\#`)
	return s
}

func formatTextLineWithMath(line string) string {
	indices := reInline.FindAllStringSubmatchIndex(line, -1)
	if len(indices) == 0 {
		clean := escapeLatexText(line)
		return fmt.Sprintf(`\text{%s}`, clean)
	}

	var sb strings.Builder
	lastIdx := 0
	for _, idx := range indices {
		start, end := idx[0], idx[1]
		if start > lastIdx {
			plain := line[lastIdx:start]
			if strings.TrimSpace(plain) != "" {
				sb.WriteString(fmt.Sprintf(`\text{%s}`, escapeLatexText(plain)))
			} else if len(plain) > 0 {
				sb.WriteString(`\ `)
			}
		}

		mathContent := ""
		if idx[2] >= 0 && idx[3] >= 0 {
			mathContent = line[idx[2]:idx[3]]
		} else if idx[4] >= 0 && idx[5] >= 0 {
			mathContent = line[idx[4]:idx[5]]
		}
		sb.WriteString(" " + strings.TrimSpace(mathContent) + " ")
		lastIdx = end
	}

	if lastIdx < len(line) {
		plain := line[lastIdx:]
		if strings.TrimSpace(plain) != "" {
			sb.WriteString(fmt.Sprintf(`\text{%s}`, escapeLatexText(plain)))
		} else if len(plain) > 0 {
			sb.WriteString(`\ `)
		}
	}

	return sb.String()
}

func wrapText(text string, maxLen int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	curr := words[0]

	for _, w := range words[1:] {
		if len(curr)+1+len(w) <= maxLen {
			curr += " " + w
		} else {
			lines = append(lines, curr)
			curr = w
		}
	}
	lines = append(lines, curr)
	return lines
}

// FormatExplanationAsLatex transforms an entire math explanation (text + equations)
// into a left-aligned, high-readability LaTeX document suitable for image rendering.
func FormatExplanationAsLatex(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, `\begin{`) && strings.Contains(trimmed, `\end{`) {
		return trimmed
	}

	lines := strings.Split(text, "\n")
	var sb strings.Builder
	sb.WriteString("\\begin{aligned}\n")

	inDisplayBlock := false
	var displayBuf strings.Builder
	itemCount := 0

	for _, rawLine := range lines {
		tLine := strings.TrimSpace(rawLine)

		if inDisplayBlock {
			if strings.Contains(tLine, "$$") || strings.Contains(tLine, `\]`) {
				clean := strings.ReplaceAll(tLine, "$$", "")
				clean = strings.ReplaceAll(clean, `\]`, "")
				displayBuf.WriteString(" " + strings.TrimSpace(clean))
				inDisplayBlock = false

				formula := strings.TrimSpace(displayBuf.String())
				if formula != "" {
					if !strings.HasPrefix(formula, "&") {
						sb.WriteString("& ")
					}
					sb.WriteString(formula)
					sb.WriteString(" \\\\[8pt]\n")
					itemCount++
				}
				displayBuf.Reset()
			} else {
				displayBuf.WriteString(" " + tLine)
			}
			continue
		}

		if tLine == "" {
			continue
		}

		// Single line display blocks
		if strings.HasPrefix(tLine, "$$") && strings.HasSuffix(tLine, "$$") && len(tLine) > 4 {
			formula := strings.TrimSpace(tLine[2 : len(tLine)-2])
			if !strings.HasPrefix(formula, "&") {
				sb.WriteString("& ")
			}
			sb.WriteString(formula)
			sb.WriteString(" \\\\[8pt]\n")
			itemCount++
			continue
		}
		if strings.HasPrefix(tLine, `\[`) && strings.HasSuffix(tLine, `\]`) && len(tLine) > 4 {
			formula := strings.TrimSpace(tLine[2 : len(tLine)-2])
			if !strings.HasPrefix(formula, "&") {
				sb.WriteString("& ")
			}
			sb.WriteString(formula)
			sb.WriteString(" \\\\[8pt]\n")
			itemCount++
			continue
		}
		if strings.HasPrefix(tLine, "$$") || strings.HasPrefix(tLine, `\[`) {
			inDisplayBlock = true
			displayBuf.WriteString(strings.TrimPrefix(strings.TrimPrefix(tLine, "$$"), `\[`))
			continue
		}

		// Numbered step: "1. Identify coefficients" -> "\textbf{Step 1: Identify coefficients}"
		if m := reNumberedStep.FindStringSubmatch(tLine); m != nil && len(m) > 2 {
			stepNum := strings.TrimRight(m[1], ".)")
			stepTitle := strings.TrimSpace(m[2])
			if !strings.HasPrefix(strings.ToLower(stepTitle), "step") {
				sb.WriteString(fmt.Sprintf("& \\textbf{Step %s: %s} \\\\[6pt]\n", escapeLatexText(stepNum), escapeLatexText(stepTitle)))
			} else {
				sb.WriteString(fmt.Sprintf("& \\textbf{%s} \\\\[6pt]\n", escapeLatexText(tLine)))
			}
			itemCount++
			continue
		}

		// Markdown Headings
		if strings.HasPrefix(tLine, "#") {
			heading := strings.TrimLeft(tLine, "# ")
			sb.WriteString(fmt.Sprintf("& \\textbf{%s} \\\\[6pt]\n", escapeLatexText(heading)))
			itemCount++
			continue
		}
		if strings.HasPrefix(tLine, "**") && strings.HasSuffix(tLine, "**") && len(tLine) > 4 {
			heading := tLine[2 : len(tLine)-2]
			sb.WriteString(fmt.Sprintf("& \\textbf{%s} \\\\[6pt]\n", escapeLatexText(heading)))
			itemCount++
			continue
		}

		// Bullet point
		if strings.HasPrefix(tLine, "- ") || strings.HasPrefix(tLine, "* ") {
			bulletContent := strings.TrimSpace(tLine[2:])
			formatted := formatTextLineWithMath(bulletContent)
			sb.WriteString(fmt.Sprintf("& \\bullet \\ %s \\\\[5pt]\n", formatted))
			itemCount++
			continue
		}

		// Regular text line: wrap at 65 characters to prevent wide overflow
		wrapped := wrapText(tLine, 65)
		for _, wLine := range wrapped {
			formatted := formatTextLineWithMath(wLine)
			sb.WriteString(fmt.Sprintf("& %s \\\\[4pt]\n", formatted))
			itemCount++
		}
	}

	if itemCount == 0 {
		return ""
	}

	sb.WriteString("\\end{aligned}")
	return sb.String()
}

func fetchLatexPng(ctx context.Context, latex string) ([]byte, error) {
	clean := strings.TrimSpace(latex)
	if !strings.HasPrefix(clean, `\dpi`) {
		clean = fmt.Sprintf(`\dpi{240}\bg{white}\large %s`, clean)
	}

	escaped := strings.ReplaceAll(url.QueryEscape(clean), "+", "%20")
	endpoint := "https://latex.codecogs.com/png.image?" + escaped

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) PrismBot/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("latex rendering request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("latex rendering returned status %d: %s", resp.StatusCode, string(body))
	}

	pngBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(pngBytes) < 50 {
		return nil, fmt.Errorf("failed to read rendered png image or empty payload")
	}

	return pngBytes, nil
}

// RenderMathImage renders LaTeX expressions into an aesthetic, framed mathematical card PNG image.
func RenderMathImage(ctx context.Context, text string) ([]byte, error) {
	// 1. Try full explanation card (text + formulas)
	cardLatex := FormatExplanationAsLatex(text)
	if cardLatex != "" {
		if pngBytes, err := fetchLatexPng(ctx, cardLatex); err == nil && len(pngBytes) > 100 {
			if beautified, bErr := beautifyMathCard(pngBytes); bErr == nil {
				return beautified, nil
			}
			return pngBytes, nil
		}
	}

	// 2. Fallback: extract formulas only with clean left alignment
	formula := ExtractFormulas(text)
	if formula == "" {
		return nil, fmt.Errorf("no formula could be extracted")
	}

	rawBytes, err := fetchLatexPng(ctx, formula)
	if err != nil {
		return nil, err
	}
	if beautified, bErr := beautifyMathCard(rawBytes); bErr == nil {
		return beautified, nil
	}
	return rawBytes, nil
}

func getFont(f *opentype.Font, size float64) (font.Face, error) {
	if f == nil {
		return nil, fmt.Errorf("font not initialized")
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// beautifyMathCard embeds the rendered LaTeX equation into an aesthetic study card
func beautifyMathCard(rawBytes []byte) ([]byte, error) {
	rawImg, err := png.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return rawBytes, nil // Fallback to raw bytes if decoding fails
	}

	rawW := rawImg.Bounds().Dx()
	rawH := rawImg.Bounds().Dy()

	const (
		padX    = 40
		padY    = 24
		headerH = 50
		footerH = 36
	)

	cardW := rawW + padX*2
	if cardW < 560 {
		cardW = 560
	}
	cardH := headerH + rawH + padY*2 + footerH

	canvas := image.NewRGBA(image.Rect(0, 0, cardW, cardH))

	// 1. Fill card background with soft light background (#F8FAFC)
	bgCol := color.RGBA{R: 248, G: 250, B: 252, A: 255}
	for y := 0; y < cardH; y++ {
		for x := 0; x < cardW; x++ {
			canvas.SetRGBA(x, y, bgCol)
		}
	}

	// 2. Inner White Paper Card Container for the equation
	paperX1 := 16
	paperY1 := headerH
	paperX2 := cardW - 16
	paperY2 := cardH - footerH
	paperBg := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	paperBorder := color.RGBA{R: 226, G: 232, B: 240, A: 255}

	for y := paperY1; y <= paperY2; y++ {
		for x := paperX1; x <= paperX2; x++ {
			if x == paperX1 || x == paperX2 || y == paperY1 || y == paperY2 {
				canvas.SetRGBA(x, y, paperBorder)
			} else {
				canvas.SetRGBA(x, y, paperBg)
			}
		}
	}

	// 3. Draw outer border with subtle rounded corners
	borderCol := color.RGBA{R: 203, G: 213, B: 225, A: 255} // Slate-300
	drawCardBorder(canvas, 0, 0, cardW-1, cardH-1, 10, borderCol)

	// 4. Header Pill Badge ("MATHEMATICAL SOLUTION")
	drawHeaderPill(canvas, cardW/2, 24, "📐 MATHEMATICAL SOLUTION")

	// 5. Draw equation centered inside the white paper container
	formulaX := (cardW - rawW) / 2
	formulaY := paperY1 + (paperY2-paperY1-rawH)/2
	if formulaY < paperY1+padY {
		formulaY = paperY1 + padY
	}
	formulaRect := image.Rect(formulaX, formulaY, formulaX+rawW, formulaY+rawH)
	draw.Draw(canvas, formulaRect, rawImg, image.Point{}, draw.Over)

	// 6. Footer text
	if footFace, err := getFont(parsedRegularFace, 11); err == nil {
		defer footFace.Close()
		drawCenteredText(canvas, footFace, "PrismBot AI • Verified Step-by-Step Solution", cardW/2, cardH-14, color.RGBA{R: 148, G: 163, B: 184, A: 255})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return rawBytes, nil
	}
	return buf.Bytes(), nil
}

func drawCardBorder(img *image.RGBA, x1, y1, x2, y2, r int, border color.RGBA) {
	for x := x1 + r; x <= x2-r; x++ {
		img.SetRGBA(x, y1, border)
		img.SetRGBA(x, y2, border)
	}
	for y := y1 + r; y <= y2-r; y++ {
		img.SetRGBA(x1, y, border)
		img.SetRGBA(x2, y, border)
	}
}

func drawHeaderPill(img *image.RGBA, cx, cy int, label string) {
	pillFace, err := getFont(parsedBoldFace, 11)
	if err != nil {
		return
	}
	defer pillFace.Close()

	d := &font.Drawer{Face: pillFace}
	textW := d.MeasureString(label).Round()

	pillW := textW + 24
	pillH := 24
	x1 := cx - pillW/2
	y1 := cy - pillH/2
	x2 := cx + pillW/2
	y2 := cy + pillH/2

	bg := color.RGBA{R: 238, G: 242, B: 255, A: 255}    // Indigo-50
	border := color.RGBA{R: 199, G: 210, B: 254, A: 255} // Indigo-200

	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			if x == x1 || x == x2 || y == y1 || y == y2 {
				img.SetRGBA(x, y, border)
			} else {
				img.SetRGBA(x, y, bg)
			}
		}
	}

	textColor := color.RGBA{R: 79, G: 70, B: 229, A: 255} // Indigo-600
	drawCenteredText(img, pillFace, label, cx, cy+4, textColor)
}

func drawCenteredText(dst draw.Image, face font.Face, text string, cx, yBaseline int, col color.RGBA) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: face,
	}
	textW := d.MeasureString(text).Round()
	d.Dot = fixed.P(cx-textW/2, yBaseline)
	d.DrawString(text)
}

var (
	reDollar    = regexp.MustCompile(`\$\$?`)
	reBracket   = regexp.MustCompile(`\\\(|\\\)|\\\[|\\\]`)
	reFrac      = regexp.MustCompile(`(?i)\\d?frac\{([^{}]+)\}\{([^{}]+)\}`)
	reSqrtGroup = regexp.MustCompile(`(?i)\\sqrt\{([^{}]+)\}`)
	reSqrt      = regexp.MustCompile(`(?i)\\sqrt`)
	reTimes     = regexp.MustCompile(`(?i)\\times`)
	reCdot      = regexp.MustCompile(`(?i)\\cdot`)
	reDiv       = regexp.MustCompile(`(?i)\\div`)
	rePm        = regexp.MustCompile(`(?i)\\pm`)
	reInfty     = regexp.MustCompile(`(?i)\\infty`)
	reLe        = regexp.MustCompile(`(?i)\\le(q)?`)
	reGe        = regexp.MustCompile(`(?i)\\ge(q)?`)
	reNeq       = regexp.MustCompile(`(?i)\\neq`)
	reApprox    = regexp.MustCompile(`(?i)\\approx`)
	reArrow     = regexp.MustCompile(`(?i)\\(rightarrow|to)`)
	reInt       = regexp.MustCompile(`(?i)\\int`)
	reSum       = regexp.MustCompile(`(?i)\\sum`)
	reProd      = regexp.MustCompile(`(?i)\\prod`)
	rePartial   = regexp.MustCompile(`(?i)\\partial`)
	reAlpha     = regexp.MustCompile(`(?i)\\alpha`)
	reBeta      = regexp.MustCompile(`(?i)\\beta`)
	reGamma     = regexp.MustCompile(`(?i)\\gamma`)
	reTheta     = regexp.MustCompile(`(?i)\\theta`)
	reLambda    = regexp.MustCompile(`(?i)\\lambda`)
	reMu        = regexp.MustCompile(`(?i)\\mu`)
	rePi        = regexp.MustCompile(`(?i)\\pi`)
	reSigma     = regexp.MustCompile(`(?i)\\sigma`)
	reOmega     = regexp.MustCompile(`(?i)\\omega`)
	reSup2      = regexp.MustCompile(`\^\{?2\}?\b`)
	reSup3      = regexp.MustCompile(`\^\{?3\}?\b`)
	reSupGroup  = regexp.MustCompile(`\^\{([^{}]+)\}`)
	reSubGroup  = regexp.MustCompile(`_\{([^{}]+)\}`)
	reLeftRight = regexp.MustCompile(`(?i)\\(left|right)`)
	reSpaces    = regexp.MustCompile(`[ \t]{2,}`)
)

// SanitizeMathNotation converts raw LaTeX syntax into readable Unicode math characters as a fallback.
func SanitizeMathNotation(str string) string {
	if str == "" {
		return str
	}
	s := str
	s = reDollar.ReplaceAllString(s, "")
	s = reBracket.ReplaceAllString(s, "")
	s = reFrac.ReplaceAllString(s, "($1)/($2)")
	s = reSqrtGroup.ReplaceAllString(s, "√($1)")
	s = reSqrt.ReplaceAllString(s, "√")
	s = reTimes.ReplaceAllString(s, "×")
	s = reCdot.ReplaceAllString(s, "·")
	s = reDiv.ReplaceAllString(s, "÷")
	s = rePm.ReplaceAllString(s, "±")
	s = reInfty.ReplaceAllString(s, "∞")
	s = reLe.ReplaceAllString(s, "≤")
	s = reGe.ReplaceAllString(s, "≥")
	s = reNeq.ReplaceAllString(s, "≠")
	s = reApprox.ReplaceAllString(s, "≈")
	s = reArrow.ReplaceAllString(s, "→")
	s = reInt.ReplaceAllString(s, "∫")
	s = reSum.ReplaceAllString(s, "∑")
	s = reProd.ReplaceAllString(s, "∏")
	s = rePartial.ReplaceAllString(s, "∂")
	s = reAlpha.ReplaceAllString(s, "α")
	s = reBeta.ReplaceAllString(s, "β")
	s = reGamma.ReplaceAllString(s, "γ")
	s = reTheta.ReplaceAllString(s, "θ")
	s = reLambda.ReplaceAllString(s, "λ")
	s = reMu.ReplaceAllString(s, "μ")
	s = rePi.ReplaceAllString(s, "π")
	s = reSigma.ReplaceAllString(s, "σ")
	s = reOmega.ReplaceAllString(s, "ω")
	s = reSup2.ReplaceAllString(s, "²")
	s = reSup3.ReplaceAllString(s, "³")
	s = reSupGroup.ReplaceAllString(s, "^($1)")
	s = reSubGroup.ReplaceAllString(s, "_($1)")
	s = reLeftRight.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
