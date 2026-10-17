package renderer

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestExtractFormulas(t *testing.T) {
	// 1. Single display block
	input1 := "The solution is:\n\\[ \\int_0^\\infty e^{-x} dx = 1 \\]\nDone."
	extracted1 := ExtractFormulas(input1)
	if !strings.Contains(extracted1, `\int_0^\infty`) {
		t.Fatalf("Failed to extract single formula: %s", extracted1)
	}

	// 2. Multiple display blocks
	input2 := "Step 1: \\[ x = 5 \\] Step 2: \\[ y = 10 \\]"
	extracted2 := ExtractFormulas(input2)
	if !strings.Contains(extracted2, `\begin{aligned}`) || !strings.Contains(extracted2, `x = 5`) || !strings.Contains(extracted2, `y = 10`) {
		t.Fatalf("Failed to combine multiple formulas into aligned block: %s", extracted2)
	}

	// 3. Inline formula fallback
	input3 := "The value is \\( \\alpha + \\beta = 180^\\circ \\)."
	extracted3 := ExtractFormulas(input3)
	if !strings.Contains(extracted3, `\alpha + \beta`) {
		t.Fatalf("Failed to extract inline formula: %s", extracted3)
	}
}

func TestRenderMathImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	latex := "\\[ \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a} \\]"
	pngBytes, err := RenderMathImage(ctx, latex)
	if err != nil {
		t.Fatalf("RenderMathImage failed: %v", err)
	}

	if len(pngBytes) == 0 {
		t.Fatalf("RenderMathImage returned 0 bytes")
	}

	// Verify PNG magic header: \x89PNG\r\n\x1a\n
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(pngBytes, pngHeader) {
		t.Fatalf("Output is not a valid PNG image")
	}
}

func TestFormatExplanationAsLatex(t *testing.T) {
	input := `### Step 1: Formula
Given the equation:
$$x^2 + 5x + 6 = 0$$
- $a = 1$
- $b = 5$
Therefore $x = -2$ or $x = -3$.`

	latex := FormatExplanationAsLatex(input)
	if !strings.Contains(latex, `\begin{aligned}`) {
		t.Fatalf("Expected \\begin{aligned} in latex output: %s", latex)
	}
	if !strings.Contains(latex, `\textbf{Step 1: Formula}`) {
		t.Fatalf("Expected bold heading in latex output: %s", latex)
	}
	if !strings.Contains(latex, `x^2 + 5x + 6 = 0`) {
		t.Fatalf("Expected equation in latex output: %s", latex)
	}
	if !strings.Contains(latex, `\bullet`) {
		t.Fatalf("Expected bullet point in latex output: %s", latex)
	}
}

func TestRealisticAIMathResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	aiResponse := `**Problem:** Solve for x in $2x^2 + 5x - 3 = 0$.

**Step-by-step Solution:**
1. Identify the coefficients:
$$a = 2, \quad b = 5, \quad c = -3$$

2. Use the quadratic formula:
$$x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}$$

3. Substitute the values into the formula:
$$\begin{aligned}
x &= \frac{-5 \pm \sqrt{5^2 - 4(2)(-3)}}{2(2)} \\
  &= \frac{-5 \pm \sqrt{25 + 24}}{4} \\
  &= \frac{-5 \pm \sqrt{49}}{4} \\
  &= \frac{-5 \pm 7}{4}
\end{aligned}$$

4. Calculate the two possible solutions:
- For the positive root:
$$x = \frac{-5 + 7}{4} = \frac{2}{4} = \frac{1}{2}$$
- For the negative root:
$$x = \frac{-5 - 7}{4} = \frac{-12}{4} = -3$$

**Final Answer:**
The solutions are $x = \frac{1}{2}$ and $x = -3$.`

	png, err := RenderMathImage(ctx, aiResponse)
	if err != nil {
		t.Fatalf("RenderMathImage failed: %v", err)
	}
	t.Logf("Rendered %d bytes", len(png))
}

func TestCodeCogsUnbalanced(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	bad := `\left( x + 1`
	png, err := fetchLatexPng(ctx, bad)
	t.Logf("Unbalanced left png: %d bytes, err: %v", len(png), err)
}



