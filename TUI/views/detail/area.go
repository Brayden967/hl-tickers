package detail

import (
	"math"
	"strings"

	"github.com/brayden967/hl-tickers/TUI/views/chart"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderArea(closes []float64, width, height, dir int, hi, lo float64) []string {
	out := chart.Blank(width, height)
	if width <= 0 || height <= 0 || len(closes) == 0 {
		return out
	}

	// Neon colors for the price line
	neon := style.ColorUpNeon
	switch {
	case dir < 0:
		neon = style.ColorDownNeon
	case dir == 0:
		neon = style.ColorDim
	}
	bg := style.ColorBg
	lineSt := lipgloss.NewStyle().Foreground(neon).Bold(true)
	sqSt := lipgloss.NewStyle().Foreground(style.Blend(neon, bg, 0.60))
	gridSt := lipgloss.NewStyle().Foreground(style.Blend(neon, bg, 0.90))

	rowAt := make([]int, width)
	for x := 0; x < width; x++ {
		v := sampleSeries(closes, x, width)
		r := (height - 1) / 2
		if hi > lo {
			r = int(math.Round((hi - v) / (hi - lo) * float64(height-1)))
		}
		if r < 0 {
			r = 0
		} else if r >= height {
			r = height - 1
		}
		rowAt[x] = r
	}
	lineTop := make([]int, width)
	lineBot := make([]int, width)
	for x := 0; x < width; x++ {
		a, bn := rowAt[x], rowAt[x]
		if x+1 < width {
			if rowAt[x+1] < a {
				a = rowAt[x+1]
			}
			if rowAt[x+1] > bn {
				bn = rowAt[x+1]
			}
		}
		lineTop[x], lineBot[x] = a, bn
	}

	const (
		lineStep = 1
		bandRows = 2
		gridCol  = 6
		gridRow  = 2
	)
	cells := make([][]byte, height)
	for y := range cells {
		cells[y] = make([]byte, width)
	}
	step := 0
	for x := 0; x < width; x++ {
		for y := lineTop[x]; y <= lineBot[x]; y++ {
			if step%lineStep == 0 {
				cells[y][x] = 1
			}
			step++
		}
	}
	for x := 0; x < width; x += 2 {
		for k := 1; k <= bandRows; k++ {
			if ya := lineTop[x] - k; ya >= 0 && cells[ya][x] == 0 {
				cells[ya][x] = 2
			}
			if yb := lineBot[x] + k; yb < height && cells[yb][x] == 0 {
				cells[yb][x] = 2
			}
		}
	}

	for y := 0; y < height; y++ {
		var b strings.Builder
		for x := 0; x < width; x++ {
			switch cells[y][x] {
			case 1:
				b.WriteString(lineSt.Render("•"))
			case 2:
				b.WriteString(sqSt.Render("▫"))
			default:
				if y%gridRow == 0 && x%gridCol == 0 {
					b.WriteString(gridSt.Render("·"))
				} else {
					b.WriteByte(' ')
				}
			}
		}
		out[y] = b.String()
	}
	return out
}

func sampleSeries(v []float64, x, width int) float64 {
	if len(v) == 1 || width <= 1 {
		return v[len(v)-1]
	}
	pos := float64(x) / float64(width-1) * float64(len(v)-1)
	i := int(pos)
	if i >= len(v)-1 {
		return v[len(v)-1]
	}
	return v[i] + (v[i+1]-v[i])*(pos-float64(i))
}
