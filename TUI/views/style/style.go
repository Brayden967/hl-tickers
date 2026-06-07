// Colour palette and lipgloss styles for the UI.
package style

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Returns the colour a fraction t (0..1) for tick fade out
func Blend(from, to lipgloss.Color, t float64) lipgloss.Color {
	a, err1 := colorful.Hex(string(from))
	b, err2 := colorful.Hex(string(to))
	if err1 != nil || err2 != nil {
		return from
	}
	return lipgloss.Color(a.BlendLab(b, t).Clamped().Hex())
}

// Palette colours
var (
	ColorUp       = lipgloss.Color("#26d07c") // green
	ColorDown     = lipgloss.Color("#ff5c5c") // red
	ColorUpNeon   = lipgloss.Color("#2bff8c") // neon green for chart lines / volume bars
	ColorDownNeon = lipgloss.Color("#ff3355") // neon red for chart lines / volume bars
	ColorFlat     = lipgloss.Color("#8a8a8a")
	ColorText     = lipgloss.Color("#e4e4e4")
	ColorLabel    = lipgloss.Color("#7a7a7a")
	ColorDim      = lipgloss.Color("#5a5a5a")
	ColorAccent   = lipgloss.Color("#26d07c") // neon green
	ColorWarn     = lipgloss.Color("#ff8700") // caution / warning amber
	ColorStar     = lipgloss.Color("#ffd75f")
	ColorBg       = lipgloss.Color("#1c1c1c")
	ColorSelBg    = lipgloss.Color("#303030")
	ColorFlashUp  = lipgloss.Color("#0a3d24")
	ColorFlashDn  = lipgloss.Color("#4d1414")
)

// Reusable styles
var (
	Logo     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffd7")).Background(ColorAccent).Bold(true)
	Bold     = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	Text     = lipgloss.NewStyle().Foreground(ColorText)
	Label    = lipgloss.NewStyle().Foreground(ColorLabel)
	Dim      = lipgloss.NewStyle().Foreground(ColorDim)
	Up       = lipgloss.NewStyle().Foreground(ColorUp)
	Down     = lipgloss.NewStyle().Foreground(ColorDown)
	Flat     = lipgloss.NewStyle().Foreground(ColorFlat)
	Star     = lipgloss.NewStyle().Foreground(ColorStar)
	Accent   = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	Warn     = lipgloss.NewStyle().Foreground(ColorWarn)
	Selected = lipgloss.NewStyle().Background(ColorSelBg)
	Help     = lipgloss.NewStyle().Foreground(ColorDim)
)

// Returns the style appropriate to a signed value.
func ForChange(v float64) lipgloss.Style {
	switch {
	case v > 0:
		return Up
	case v < 0:
		return Down
	default:
		return Flat
	}
}

// Returns the trend style for a direction sign: +1 up, -1 down,  0 (flat/unknown) dim. Used for chart lines, sparklines, and live markers.
func ForDirection(dir int) lipgloss.Style {
	switch {
	case dir > 0:
		return Up
	case dir < 0:
		return Down
	default:
		return Dim
	}
}

func Tag(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d0d0d0")).
		Background(lipgloss.Color("#3a3a3a")).
		Render(" " + s + " ")
}

// Ramps up pulse colors
var connPulse = []string{"#0f5130", "#1c6b3f", "#26d07c", "#5cff9d", "#26d07c", "#1c6b3f"}

const connPulseDivisor = 6

// Connection indicator for websocket health
func ConnIndicator(connected bool, frame int) string {
	if !connected {
		return lipgloss.NewStyle().Foreground(ColorDown).Render("● disconnected")
	}
	if frame < 0 {
		frame = -frame
	}
	shade := connPulse[(frame/connPulseDivisor)%len(connPulse)]
	return lipgloss.NewStyle().Foreground(lipgloss.Color(shade)).Render("●") +
		lipgloss.NewStyle().Foreground(ColorUp).Render(" connected")
}
