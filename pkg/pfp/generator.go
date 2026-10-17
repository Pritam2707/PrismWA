package pfp

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"strings"
	"time"

	"whatsapp-bot/pkg/config"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Cached parsed font families
var (
	parsedBold    *opentype.Font
	parsedRegular *opentype.Font
)

func init() {
	var err error
	parsedBold, err = opentype.Parse(gobold.TTF)
	if err != nil {
		fmt.Printf("[PFP] Failed to parse GoBold font: %v\n", err)
	}
	parsedRegular, err = opentype.Parse(goregular.TTF)
	if err != nil {
		fmt.Printf("[PFP] Failed to parse GoRegular font: %v\n", err)
	}
}

func getFace(f *opentype.Font, size float64) (font.Face, error) {
	if f == nil {
		return nil, fmt.Errorf("font not initialized")
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// GenerateDeadlinePFP creates an aesthetic 640x640 countdown image for WhatsApp group PFP
func GenerateDeadlinePFP(title string, targetDateStr string) ([]byte, error) {
	const (
		width  = 640
		height = 640
	)

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Parse target date
	targetDate, err := time.Parse("2006-01-02", strings.TrimSpace(targetDateStr))
	if err != nil {
		return nil, fmt.Errorf("invalid target date format (expected YYYY-MM-DD): %w", err)
	}

	// Calculate days remaining (based on date at midnight in configured timezone, default IST)
	loc := config.GetLocation()
	now := time.Now().In(loc)
	todayMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	targetMidnight := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, loc)

	daysLeft := int(math.Ceil(targetMidnight.Sub(todayMidnight).Hours() / 24.0))

	// Determine status and colors
	var accentColor color.RGBA
	var numberStr string
	var unitStr string

	if daysLeft > 1 {
		accentColor = color.RGBA{R: 56, G: 189, B: 248, A: 255} // Radiant Sky Blue
		numberStr = fmt.Sprintf("%d", daysLeft)
		unitStr = "DAYS LEFT"
	} else if daysLeft == 1 {
		accentColor = color.RGBA{R: 251, G: 191, B: 36, A: 255} // Amber Glow
		numberStr = "1"
		unitStr = "DAY REMAINING"
	} else if daysLeft == 0 {
		accentColor = color.RGBA{R: 239, G: 68, B: 68, A: 255} // Vivid Crimson
		numberStr = "0"
		unitStr = "D-DAY IS TODAY"
	} else {
		accentColor = color.RGBA{R: 16, G: 185, B: 129, A: 255} // Emerald Green
		numberStr = "DONE"
		unitStr = "DEADLINE PASSED"
	}

	// 1. Draw rich background gradient (Deep Obsidian to Midnight Navy)
	for y := 0; y < height; y++ {
		factor := float64(y) / float64(height)
		// Top: #0B0F19 (11, 15, 25) -> Bottom: #111827 (17, 24, 39)
		r := uint8(11.0 + (17.0-11.0)*factor)
		g := uint8(15.0 + (24.0-15.0)*factor)
		b := uint8(25.0 + (39.0-25.0)*factor)

		for x := 0; x < width; x++ {
			// Subtle radial spotlight in the top-center
			dx := float64(x - width/2)
			dy := float64(y - 220)
			dist := math.Sqrt(dx*dx + dy*dy)
			spotlight := math.Max(0, 1.0-dist/380.0)

			sr := math.Min(255, float64(r)+spotlight*float64(accentColor.R)*0.15)
			sg := math.Min(255, float64(g)+spotlight*float64(accentColor.G)*0.15)
			sb := math.Min(255, float64(b)+spotlight*float64(accentColor.B)*0.15)

			img.SetRGBA(x, y, color.RGBA{R: uint8(sr), G: uint8(sg), B: uint8(sb), A: 255})
		}
	}

	// 2. Draw aesthetic Inner Glass Card (36px margin)
	cardX1, cardY1 := 36, 36
	cardX2, cardY2 := width-36, height-36
	cardRadius := 28

	cardBg := color.RGBA{R: 17, G: 24, B: 39, A: 210}
	cardBorder := color.RGBA{R: 51, G: 65, B: 85, A: 255}
	accentBorder := color.RGBA{R: accentColor.R, G: accentColor.G, B: accentColor.B, A: 120}

	drawRoundedCard(img, cardX1, cardY1, cardX2, cardY2, cardRadius, cardBg, cardBorder, accentBorder)

	// 3. Draw Top Category Pill Badge ("TARGET COUNTDOWN")
	badgeText := "TARGET COUNTDOWN"
	drawPillBadge(img, width/2, 82, badgeText, accentColor)

	// 4. Draw Group/Event Title
	if strings.TrimSpace(title) == "" {
		title = "Goal Deadline"
	}
	// Truncate title if excessively long
	if len(title) > 28 {
		title = title[:25] + "..."
	}

	titleFace, err := getFace(parsedBold, 26)
	if err == nil {
		defer titleFace.Close()
		titleColor := color.RGBA{R: 248, G: 250, B: 252, A: 255} // #F8FAFC
		drawCenteredString(img, titleFace, title, width/2, 160, titleColor)
	}

	// 5. Draw Massive Big Countdown Number
	numFaceSize := 120.0
	if len(numberStr) > 2 {
		numFaceSize = 90.0
	}
	numFace, err := getFace(parsedBold, numFaceSize)
	if err == nil {
		defer numFace.Close()
		// Glowing shadow
		shadowColor := color.RGBA{R: accentColor.R / 3, G: accentColor.G / 3, B: accentColor.B / 3, A: 255}
		drawCenteredString(img, numFace, numberStr, width/2+2, 312, shadowColor)
		// Main luminous number
		numColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		drawCenteredString(img, numFace, numberStr, width/2, 310, numColor)
	}

	// 6. Draw Unit / State Text ("DAYS LEFT")
	unitFace, err := getFace(parsedBold, 22)
	if err == nil {
		defer unitFace.Close()
		drawCenteredString(img, unitFace, unitStr, width/2, 385, accentColor)
	}

	// 7. Divider Accent Line with glowing center
	dividerY := 435
	divWidth := 340
	startX := (width - divWidth) / 2
	for i := 0; i < divWidth; i++ {
		px := startX + i
		// Normalized -1 to 1 from center
		rel := math.Abs(float64(i-divWidth/2)) / float64(divWidth/2)
		alpha := uint8(math.Max(0, (1.0-rel)*200))
		img.SetRGBA(px, dividerY, color.RGBA{R: accentColor.R, G: accentColor.G, B: accentColor.B, A: alpha})
		img.SetRGBA(px, dividerY+1, color.RGBA{R: accentColor.R, G: accentColor.G, B: accentColor.B, A: alpha / 2})
	}

	// 8. Target Date Badge
	targetFormatted := targetDate.Format("January 02, 2006")
	targetFace, err := getFace(parsedRegular, 19)
	if err == nil {
		defer targetFace.Close()
		subColor := color.RGBA{R: 203, G: 213, B: 225, A: 255} // Slate-300
		drawCenteredString(img, targetFace, "Due: "+targetFormatted, width/2, 485, subColor)
	}

	// 9. Motivational Tagline / Footer
	footerFace, err := getFace(parsedRegular, 15)
	if err == nil {
		defer footerFace.Close()
		footerColor := color.RGBA{R: 148, G: 163, B: 184, A: 255} // Slate-400
		motto := "Focus • Discipline • Every Day Counts"
		if daysLeft <= 1 {
			motto = "Final Stretch • Give Everything You've Got!"
		} else if daysLeft < 0 {
			motto = "Mission Accomplished • Set Next Objective"
		}
		drawCenteredString(img, footerFace, motto, width/2, 545, footerColor)
	}

	// Encode to JPEG (WhatsApp SetGroupPhoto strictly requires valid JPEG)
	var buf bytes.Buffer
	opts := &jpeg.Options{Quality: 95}
	if err := jpeg.Encode(&buf, img, opts); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}

	return buf.Bytes(), nil
}

// drawRoundedCard draws a rounded container with double border
func drawRoundedCard(img *image.RGBA, x1, y1, x2, y2, r int, fill, border, accentBorder color.RGBA) {
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			// Check corners
			inCorner := false
			var dx, dy int
			if x < x1+r && y < y1+r {
				dx, dy = (x1 + r - x), (y1 + r - y)
				inCorner = true
			} else if x > x2-r && y < y1+r {
				dx, dy = (x - (x2 - r)), (y1 + r - y)
				inCorner = true
			} else if x < x1+r && y > y2-r {
				dx, dy = (x1 + r - x), (y - (y2 - r))
				inCorner = true
			} else if x > x2-r && y > y2-r {
				dx, dy = (x - (x2 - r)), (y - (y2 - r))
				inCorner = true
			}

			if inCorner {
				dist := math.Sqrt(float64(dx*dx + dy*dy))
				if dist > float64(r) {
					continue // Outside rounded boundary
				}
				if dist >= float64(r-2) {
					blendPixel(img, x, y, accentBorder)
					continue
				}
			} else {
				// Outer borders
				if x == x1 || x == x2 || y == y1 || y == y2 {
					blendPixel(img, x, y, accentBorder)
					continue
				}
				if x == x1+1 || x == x2-1 || y == y1+1 || y == y2-1 {
					blendPixel(img, x, y, border)
					continue
				}
			}

			blendPixel(img, x, y, fill)
		}
	}
}

// drawPillBadge draws a small rounded pill badge with centered label
func drawPillBadge(img *image.RGBA, cx, cy int, label string, accent color.RGBA) {
	pillFace, err := getFace(parsedBold, 13)
	if err != nil {
		return
	}
	defer pillFace.Close()

	d := &font.Drawer{Face: pillFace}
	textAdv := d.MeasureString(label)
	textW := textAdv.Round()

	pillW := textW + 28
	pillH := 26
	x1 := cx - pillW/2
	y1 := cy - pillH/2
	x2 := cx + pillW/2
	y2 := cy + pillH/2
	radius := pillH / 2

	bg := color.RGBA{R: accent.R / 4, G: accent.G / 4, B: accent.B / 4, A: 230}
	border := color.RGBA{R: accent.R, G: accent.G, B: accent.B, A: 200}

	drawRoundedCard(img, x1, y1, x2, y2, radius, bg, border, border)

	// Draw badge text
	textColor := color.RGBA{R: accent.R, G: accent.G, B: accent.B, A: 255}
	drawCenteredString(img, pillFace, label, cx, cy+5, textColor)
}

// drawCenteredString measures string and centers it horizontally at (cx, yBaseline)
func drawCenteredString(dst draw.Image, face font.Face, text string, cx, yBaseline int, col color.RGBA) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(col),
		Face: face,
	}

	adv := d.MeasureString(text)
	textW := adv.Round()
	startX := cx - textW/2

	d.Dot = fixed.P(startX, yBaseline)
	d.DrawString(text)
}

// blendPixel blends a color with alpha over the existing pixel
func blendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return
	}
	if src.A == 255 {
		img.SetRGBA(x, y, src)
		return
	}
	bg := img.RGBAAt(x, y)
	alpha := float64(src.A) / 255.0
	invAlpha := 1.0 - alpha

	r := uint8(float64(src.R)*alpha + float64(bg.R)*invAlpha)
	g := uint8(float64(src.G)*alpha + float64(bg.G)*invAlpha)
	b := uint8(float64(src.B)*alpha + float64(bg.B)*invAlpha)

	img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}
