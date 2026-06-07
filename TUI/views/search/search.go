package search

import (
	"sort"
	"strconv"
	"strings"

	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

const maxResults = 12

// Looks up a live price for a coin (for display in results)
type PriceFn func(coin string) float64

// Reports whether a coin is already in the watchlist
type WatchedFn func(coin string) bool

// Search palette state
type Model struct {
	uni         *hl.Universe
	priceFn     PriceFn
	watchedFn   WatchedFn
	query       string
	results     []hl.Listing
	selected    int
	width       int
	aliasByCoin map[string][]string // coin id -> friendly aliases (e.g. EURUSD)
}

func New(uni *hl.Universe, priceFn PriceFn, watchedFn WatchedFn) *Model {
	m := &Model{uni: uni, priceFn: priceFn, watchedFn: watchedFn, width: 60}
	m.aliasByCoin = make(map[string][]string)
	for name, coin := range hl.Aliases() {
		m.aliasByCoin[coin] = append(m.aliasByCoin[coin], name)
	}
	m.recompute()
	return m
}

func (m *Model) SetWidth(w int) { m.width = w }

func (m *Model) Reset() {
	m.query = ""
	m.selected = 0
	m.recompute()
}

func (m *Model) Query() string { return m.query }

func (m *Model) Insert(s string) {
	m.query += s
	m.selected = 0
	m.recompute()
}

func (m *Model) Backspace() {
	if r := []rune(m.query); len(r) > 0 {
		m.query = string(r[:len(r)-1])
		m.selected = 0
		m.recompute()
	}
}

func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *Model) MoveDown() {
	if m.selected < len(m.results)-1 {
		m.selected++
	}
}

// Selected returns the highlighted listing.
func (m *Model) Selected() (hl.Listing, bool) {
	if m.selected < 0 || m.selected >= len(m.results) {
		return hl.Listing{}, false
	}
	return m.results[m.selected], true
}

type scored struct {
	l     hl.Listing
	score int
	vol   float64 // 24h notional volume, for ranking active markets first
}

// rRe-runs the fuzzy match, ranks by relevance then volume, and collapses duplicate tickers (e.g. the same symbol listed on several dexes)
func (m *Model) recompute() {
	q := strings.ToUpper(strings.TrimSpace(m.query))
	listings := m.uni.Listings

	if q == "" {
		// Default: a spread of popular assets
		m.results = topDefaults(listings)
		return
	}

	matches := make([]scored, 0, 64)
	for _, l := range listings {
		s := scoreMatch(q, l)
		if s == 0 {
			// Fall back to friendly aliases so common spellings (EURUSD, DOLLAR) surface the underlying listing (xyz:EUR, xyz:DXY).
			s = aliasScore(q, m.aliasByCoin[l.Coin])
		}
		if s > 0 {
			matches = append(matches, scored{l, s, volOf(l)})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].vol != matches[j].vol {
			return matches[i].vol > matches[j].vol
		}
		return len(matches[i].l.Display) < len(matches[j].l.Display)
	})

	out := make([]hl.Listing, 0, maxResults)
	seen := make(map[string]bool, maxResults)
	for _, mm := range matches {
		key := strings.ToUpper(mm.l.Display)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, mm.l)
		if len(out) >= maxResults {
			break
		}
	}
	m.results = out
}

func volOf(l hl.Listing) float64 {
	v, err := strconv.ParseFloat(l.Snapshot.DayNtlVlm, 64)
	if err != nil {
		return 0
	}
	return v
}

// Returns a relevance score (0 = no match). Higher is better
func scoreMatch(q string, l hl.Listing) int {
	disp := strings.ToUpper(l.Display)
	coin := strings.ToUpper(l.Coin)

	switch {
	case disp == q || coin == q:
		return 100
	case strings.HasPrefix(disp, q):
		return 80
	case strings.HasPrefix(coin, q):
		return 70
	case strings.Contains(disp, q):
		return 50
	case strings.Contains(coin, q):
		return 40
	case subsequence(q, disp):
		return 20
	default:
		return 0
	}
}

func aliasScore(q string, aliases []string) int {
	best := 0
	for _, a := range aliases {
		up := strings.ToUpper(a)
		switch {
		case up == q:
			return 90
		case strings.HasPrefix(up, q) && best < 60:
			best = 60
		case strings.Contains(up, q) && best < 35:
			best = 35
		}
	}
	return best
}

func subsequence(q, s string) bool {
	i := 0
	for j := 0; i < len(q) && j < len(s); j++ {
		if q[i] == s[j] {
			i++
		}
	}
	return i == len(q)
}

func topDefaults(listings []hl.Listing) []hl.Listing {
	want := []string{"BTC", "ETH", "SOL", "HYPE", "xyz:GOLD", "xyz:SP500", "xyz:CL", "xyz:NDX", "DOGE", "XRP", "AVAX", "LINK"}
	byCoin := make(map[string]hl.Listing, len(listings))
	for _, l := range listings {
		byCoin[l.Coin] = l
	}
	out := make([]hl.Listing, 0, maxResults)
	for _, c := range want {
		if l, ok := byCoin[c]; ok {
			out = append(out, l)
		}
		if len(out) >= maxResults {
			break
		}
	}
	return out
}

// Renders the palette as a bordered overlay box
func (m *Model) View() string {
	w := m.width
	if w > 72 {
		w = 72
	}
	inner := w - 4

	prompt := style.Accent.Render("/ ") + style.Text.Render(m.query) + style.Accent.Render("▏")
	header := lipgloss.NewStyle().Width(inner).Render(prompt)

	lines := []string{header, style.Dim.Render(strings.Repeat("─", inner))}

	if len(m.results) == 0 {
		lines = append(lines, style.Label.Render("No matches. Try BTC, GOLD, SPX, OIL…"))
	}

	for i, l := range m.results {
		cursor := "  "
		nameStyle := style.Text
		if i == m.selected {
			cursor = style.Accent.Render("▸ ")
			nameStyle = style.Bold
		}
		watched := ""
		if m.watchedFn(l.Coin) {
			watched = style.Star.Render(" ★")
		}
		kind := ""
		if l.Kind != hl.KindCrypto {
			kind = style.Dim.Render(" " + l.Kind.String())
		}
		price := ""
		if p := m.priceFn(l.Coin); p > 0 {
			price = style.Label.Render(format.Price(p, l.SzDecimals))
		}
		left := cursor + nameStyle.Render(pad(l.Display, 12)) + kind + watched
		row := lipgloss.NewStyle().Width(inner).Render(
			left + lipgloss.NewStyle().Width(inner-lipgloss.Width(left)).Align(lipgloss.Right).Render(price),
		)
		lines = append(lines, row)
	}

	lines = append(lines,
		style.Dim.Render(strings.Repeat("─", inner)),
		style.Help.Render("↑↓ move · enter add · esc close"),
	)

	box := lipgloss.NewStyle().
		Width(w).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ColorAccent).
		Padding(0, 1)
	return box.Render(strings.Join(lines, "\n"))
}

func pad(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}
