// Package format holds number/price formatting and sparkline helpers.
package format

import (
	"math"
	"strconv"
	"strings"
)

var sparkRunes = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Returns the smallest and largest values in v (both 0 if v is empty).
func MinMax(v []float64) (min, max float64) {
	if len(v) == 0 {
		return 0, 0
	}
	min, max = v[0], v[0]
	for _, x := range v {
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
	}
	return min, max
}

// Formats a price at HL perp precision (max decimals = 6 - szDecimals).
func Price(p float64, szDecimals int) string {
	if p == 0 {
		return "—"
	}
	dec := 6 - szDecimals
	if dec < 0 {
		dec = 0
	}
	if dec > 6 {
		dec = 6
	}
	abs := math.Abs(p)
	switch {
	case abs >= 10000 && dec > 1:
		dec = 1
	case abs >= 1000 && dec > 2:
		dec = 2
	}
	s := strconv.FormatFloat(p, 'f', dec, 64)
	return withThousands(s)
}

// Human formats a number with decimals scaled to value
func Human(v float64) string {
	a := math.Abs(v)
	if a == 0 {
		return "0"
	}
	var dec int
	switch {
	case a >= 1000:
		dec = 0
	case a >= 100:
		dec = 1
	case a >= 1:
		dec = 2
	case a >= 0.01:
		dec = 4
	default:
		dec = 3 - int(math.Floor(math.Log10(a)))
		if dec > 12 {
			dec = 12
		}
	}
	s := strconv.FormatFloat(v, 'f', dec, 64)
	if dec > 0 && strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return withThousands(s)
}

func Signed(v float64, dec int) string {
	s := strconv.FormatFloat(v, 'f', dec, 64)
	if v > 0 {
		return "+" + s
	}
	return s
}

func Percent(v float64) string {
	return Signed(v, 2) + "%"
}

// Funding formats an hourly funding rate fraction as a percentage
func Funding(rate float64) string {
	return Signed(rate*100, 4) + "%"
}

func Money(v float64) string {
	return "$" + withThousands(strconv.FormatFloat(v, 'f', 2, 64))
}

func Size(v float64) string {
	a := math.Abs(v)
	switch {
	case a == 0:
		return "0"
	case a >= 100000:
		return Compact(v)
	case a >= 100:
		return trimTo(v, 2)
	case a >= 1:
		return trimTo(v, 3)
	case a >= 0.01:
		return trimTo(v, 4)
	default:
		return trimTo(v, 6)
	}
}

func trimTo(v float64, dec int) string {
	s := strconv.FormatFloat(v, 'f', dec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

func Compact(v float64) string {
	abs := math.Abs(v)
	sign := ""
	if v < 0 {
		sign = "-"
	}
	switch {
	case abs >= 1e12:
		return sign + trim(abs/1e12) + "T"
	case abs >= 1e9:
		return sign + trim(abs/1e9) + "B"
	case abs >= 1e6:
		return sign + trim(abs/1e6) + "M"
	case abs >= 1e3:
		return sign + trim(abs/1e3) + "K"
	default:
		return sign + trim(abs)
	}
}

func trim(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func withThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i:]
	}
	n := len(intPart)
	if n <= 3 {
		return sign(neg) + intPart + frac
	}
	var b strings.Builder
	lead := n % 3
	if lead > 0 {
		b.WriteString(intPart[:lead])
		if n > lead {
			b.WriteByte(',')
		}
	}
	for i := lead; i < n; i += 3 {
		b.WriteString(intPart[i : i+3])
		if i+3 < n {
			b.WriteByte(',')
		}
	}
	return sign(neg) + b.String() + frac
}

func sign(neg bool) string {
	if neg {
		return "-"
	}
	return ""
}

func Sparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return strings.Repeat(" ", max(width, 0))
	}

	var pts []float64
	switch {
	case len(values) > width:
		pts = downsample(values, width)
	case len(values) < width:
		pts = upsample(values, width)
	default:
		pts = values
	}

	mn, mx := MinMax(pts)
	span := mx - mn

	var b strings.Builder
	for _, v := range pts {
		idx := 0
		if span > 0 {
			idx = int(math.Round((v - mn) / span * float64(len(sparkRunes)-1)))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkRunes) {
			idx = len(sparkRunes) - 1
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}

func upsample(values []float64, width int) []float64 {
	out := make([]float64, width)
	n := len(values)
	for i := 0; i < width; i++ {
		idx := i * n / width
		if idx >= n {
			idx = n - 1
		}
		out[i] = values[idx]
	}
	return out
}

func downsample(values []float64, width int) []float64 {
	if len(values) <= width {
		return values
	}
	out := make([]float64, width)
	bucket := float64(len(values)) / float64(width)
	for i := 0; i < width; i++ {
		start := int(float64(i) * bucket)
		end := int(float64(i+1) * bucket)
		if end > len(values) {
			end = len(values)
		}
		if end <= start {
			end = start + 1
		}
		var sum float64
		for j := start; j < end; j++ {
			sum += values[j]
		}
		out[i] = sum / float64(end-start)
	}
	return out
}

// Abbreviates a 0x address as 0x1234…abcd
func ShortAddr(a string) string {
	if len(a) <= 12 {
		return a
	}
	return a[:6] + "…" + a[len(a)-4:]
}
