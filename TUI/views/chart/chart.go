// Draws connected line charts in the terminal using Unicode
// braille (each character cell is a 2x4 dot grid), giving ~8x the vertical
// resolution of block sparklines and a smooth "line chart" feel.
package chart

import (
	"strings"

	"github.com/brayden967/hl-tickers/TUI/views/format"
)

// Braille dot bit values
var dotBits = [2][4]byte{
	{0x01, 0x02, 0x04, 0x40}, // x=0, y=0..3 (top..bottom)
	{0x08, 0x10, 0x20, 0x80}, // x=1, y=0..3
}

func Line(values []float64, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if len(values) == 0 {
		return Blank(width, height)
	}
	return renderLine(values, width, height, minMaxYAt(values, height*4), true, 1, 0, 0, false)
}

func LineSmooth(values []float64, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if len(values) == 0 {
		return Blank(width, height)
	}
	return renderLine(values, width, height, minMaxYAt(values, height*4), true, 1, 0, 0, true)
}

func LineSmoothGapped(values []float64, width, height int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if len(values) == 0 {
		return Blank(width, height)
	}
	return renderLine(values, width, height, minMaxYAt(values, height*4), true, 1, 1, 1, true)
}

func Dots(values []float64, width, height, step int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if len(values) == 0 {
		return Blank(width, height)
	}
	if step < 1 {
		step = 1
	}
	return renderLine(values, width, height, minMaxYAt(values, height*4), false, step, 0, 0, false)
}

func LineDashed(values []float64, width, height, dashLen, gapLen int) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if len(values) == 0 {
		return Blank(width, height)
	}
	if dashLen < 1 {
		dashLen = 1
	}
	return renderLine(values, width, height, minMaxYAt(values, height*4), true, 1, dashLen, gapLen, false)
}

func minMaxYAt(values []float64, dotH int) func(float64) int {
	mn, mx := format.MinMax(values)
	span := mx - mn
	return func(v float64) int {
		if span == 0 {
			return dotH / 2
		}
		return clampDot(int(float64(dotH-1)*(1-(v-mn)/span)), dotH)
	}
}

func clampDot(y, dotH int) int {
	if y < 0 {
		return 0
	}
	if y >= dotH {
		return dotH - 1
	}
	return y
}

func renderLine(values []float64, width, height int, yAt func(float64) int, connect bool, step, dashLen, gapLen int, interp bool) []string {
	dotW := width * 2
	dotH := height * 4
	if step < 1 {
		step = 1
	}

	grid := make([][]byte, height)
	for i := range grid {
		grid[i] = make([]byte, width)
	}
	set := func(x, y int) {
		if x < 0 || x >= dotW || y < 0 || y >= dotH {
			return
		}
		grid[y/4][x/2] |= dotBits[x%2][y%4]
	}

	n := len(values)
	sample := func(x int) float64 {
		if dotW <= 1 || n == 1 {
			return values[0]
		}
		pos := float64(x) * float64(n-1) / float64(dotW-1)
		if !interp {
			return values[int(pos+0.5)]
		}
		i0 := int(pos)
		if i0 >= n-1 {
			return values[n-1]
		}
		frac := pos - float64(i0)
		return values[i0]*(1-frac) + values[i0+1]*frac
	}

	prevY := -1
	for x := 0; x < dotW; x += step {
		y := yAt(sample(x))
		set(x, y)
		if connect && prevY >= 0 {
			lo, hi := prevY, y
			if lo > hi {
				lo, hi = hi, lo
			}
			for yy := lo; yy <= hi; yy++ {
				set(x, yy)
			}
		}
		prevY = y
	}

	if gapLen > 0 {
		period := dashLen + gapLen
		for x := 0; x < dotW; x++ {
			if x%period >= dashLen {
				var mask byte
				for yy := 0; yy < 4; yy++ {
					mask |= dotBits[x%2][yy]
				}
				for r := 0; r < height; r++ {
					grid[r][x/2] &^= mask
				}
			}
		}
	}

	out := make([]string, height)
	for r := 0; r < height; r++ {
		var b strings.Builder
		b.Grow(width * 3)
		for c := 0; c < width; c++ {
			b.WriteRune(rune(0x2800 + int(grid[r][c])))
		}
		out[r] = b.String()
	}
	return out
}

// Direction returns +1 if the series ends higher than it starts, -1 if lower,
// 0 if flat/empty — used to colour the line.
func Direction(values []float64) int {
	if len(values) < 2 {
		return 0
	}
	switch {
	case values[len(values)-1] > values[0]:
		return 1
	case values[len(values)-1] < values[0]:
		return -1
	default:
		return 0
	}
}

// Blank returns height lines each of width spaces
func Blank(width, height int) []string {
	row := strings.Repeat(" ", width)
	out := make([]string, height)
	for i := range out {
		out[i] = row
	}
	return out
}
