package detail

import (
	"math"
	"strings"

	"github.com/brayden967/hl-tickers/TUI/views/chart"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderVolume(usdIn, buyVol, sellVol []float64, width, height int) ([]string, float64) {
	out := chart.Blank(width, height)
	if width <= 0 || height <= 0 || len(usdIn) == 0 {
		return out, 0
	}

	bg := style.ColorBg
	greenSt := lipgloss.NewStyle().Foreground(style.ColorUpNeon).Bold(true)
	redSt := lipgloss.NewStyle().Foreground(style.ColorDownNeon).Bold(true)
	emptySt := lipgloss.NewStyle().Foreground(style.Blend(style.ColorUpNeon, bg, 0.86))

	const (
		barW   = 3
		barGap = 2
	)
	barStep := barW + barGap
	numBars := (width + barStep - 1) / barStep
	n := len(usdIn)

	// Bucket the per-point USD volume and buy/sell volume
	usd := make([]float64, numBars)
	buy := make([]bool, numBars)
	for b := 0; b < numBars; b++ {
		c0, c1 := b*n/numBars, (b+1)*n/numBars
		if c1 > n {
			c1 = n
		}
		var bu, bs float64
		for i := c0; i < c1; i++ {
			usd[b] += usdIn[i]
			bu += buyVol[i]
			bs += sellVol[i]
		}
		buy[b] = bu >= bs
	}

	// Normalised largest bar. Volume is very spiky
	// (especially on short timeframes during volatility) and a linear scale crushes the quiet buckets to nothing
	maxUsd := 0.0
	for _, u := range usd {
		if u > maxUsd {
			maxUsd = u
		}
	}
	if maxUsd <= 0 {
		return out, 0
	}
	capacity := barW * height

	cells := make([][]byte, height)
	green := make([][]bool, height)
	for y := range cells {
		cells[y] = make([]byte, width)
		green[y] = make([]bool, width)
	}
	for b := 0; b < numBars; b++ {
		bx := b * barStep
		if bx+barW > width {
			break // skip a partial bar at the right edge
		}
		dots := int(math.Round(math.Sqrt(usd[b]/maxUsd) * float64(capacity)))
		if dots > capacity {
			dots = capacity
		}
		if dots == 0 && usd[b] > 0 {
			dots = 1
		}
		for r := 0; r < height; r++ {
			inRow := dots - r*barW // dots filled in this layer (left to right)
			if inRow <= 0 {
				break // no dots in this row or above leave blank
			}
			y := height - 1 - r // bottom-up
			for c := 0; c < barW; c++ {
				if c < inRow {
					cells[y][bx+c] = byte(2 + r)
					green[y][bx+c] = buy[b]
				} else {
					cells[y][bx+c] = 1 // ▫ completes the current (partial) row only
				}
			}
		}
	}

	for y := 0; y < height; y++ {
		var b strings.Builder
		for x := 0; x < width; x++ {
			switch v := cells[y][x]; {
			case v == 0:
				b.WriteByte(' ')
			case v == 1:
				b.WriteString(emptySt.Render("▫"))
			default:
				if green[y][x] {
					b.WriteString(greenSt.Render("•"))
				} else {
					b.WriteString(redSt.Render("•"))
				}
			}
		}
		out[y] = b.String()
	}
	return out, maxUsd
}

// Horizontal hr to split price from volume chart
func volDivider(width int) string {
	if width <= 0 {
		return ""
	}
	label := " volume "
	lw := lipgloss.Width(label)
	if width <= lw {
		return style.Dim.Render(strings.Repeat("·", width))
	}
	rem := width - lw
	left := rem / 2
	right := rem - left
	return style.Dim.Render(strings.Repeat("·", left)) +
		style.Label.Render(label) +
		style.Dim.Render(strings.Repeat("·", right))
}

func volAxisLabel(row, volH int, fullBar float64) string {
	const fieldW = 8
	if fullBar <= 0 || volH <= 0 || (volH-1-row)%2 != 0 {
		return ""
	}
	frac := float64(volH-row) / float64(volH)
	s := style.Dim.Render("≈$" + format.Compact(fullBar*frac*frac))
	if pad := fieldW - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}
