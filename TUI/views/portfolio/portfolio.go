// Renders the full-screen wallet portfolio screen
package portfolio

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/market"
	"github.com/brayden967/hl-tickers/TUI/views/chart"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

// distance-to-liquidation thresholds (%)
const (
	dangerDistPct  = 5
	cautionDistPct = 15
)

type PosSort int

const (
	SortRisk  PosSort = iota // default: nearest liquidation first
	SortPnl                  // largest uPnL
	SortValue                // largest notional
)

func (s PosSort) label() string {
	switch s {
	case SortPnl:
		return "by uPnL"
	case SortValue:
		return "by value"
	default:
		return "by liquidation risk"
	}
}

type Model struct {
	store   *market.Store
	address string

	width, height int

	sort      PosSort
	posOffset int // scroll offset
	posVis    int // rows visible last render
	posCount  int // total positions last render

	mu      sync.Mutex
	pf      hl.Portfolio
	fills   []hl.Fill
	loaded  bool
	updated time.Time
}

const maxFills = 14

func New(store *market.Store, address string) *Model {
	return &Model{store: store, address: address}
}

func (m *Model) Address() string { return m.address }

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

func (m *Model) CycleSort() {
	m.sort = (m.sort + 1) % 3
	m.posOffset = 0
}

func (m *Model) ScrollPositions(delta int) {
	m.posOffset += delta
	maxOff := m.posCount - m.posVis
	if maxOff < 0 {
		maxOff = 0
	}
	if m.posOffset > maxOff {
		m.posOffset = maxOff
	}
	if m.posOffset < 0 {
		m.posOffset = 0
	}
}

func (m *Model) SetPortfolio(pf hl.Portfolio, t time.Time) {
	m.mu.Lock()
	m.pf = pf
	m.loaded = true
	m.updated = t
	m.mu.Unlock()
}

func (m *Model) SetFills(fills []hl.Fill) {
	m.mu.Lock()
	m.fills = fills
	m.mu.Unlock()
}

func (m *Model) snapshot() (hl.Portfolio, []hl.Fill, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pf, m.fills, m.loaded
}

func (m *Model) View(connected bool, frame int) string {
	acct := m.store.AccountSummary()
	positions := m.store.Positions()
	m.applySort(positions)
	pf, fills, loaded := m.snapshot()

	var top strings.Builder
	top.WriteString("\n")
	top.WriteString(m.header())
	top.WriteString("\n")
	top.WriteString(m.rule())
	top.WriteString("\n\n")
	top.WriteString(m.kpis(acct, positions))
	top.WriteString("\n\n")
	top.WriteString(m.subLine(acct))
	top.WriteString("\n\n")
	top.WriteString(m.history(acct, pf, loaded))
	top.WriteString("\n\n")
	top.WriteString(m.rule())
	topLines := strings.Split(top.String(), "\n")

	contentH := m.height - 1 // reserve the footer line
	if contentH < 1 {
		contentH = 1
	}

	// Trades take a bounded share so positions always have room to scroll into.
	lowerH := contentH - len(topLines)
	if lowerH < 8 {
		lowerH = 8
	}
	tradeRows := len(fills)
	if tradeRows > maxFills {
		tradeRows = maxFills
	}
	tradesH := tradeRows + 3 // title, blank, column header
	if half := lowerH / 2; tradesH > half {
		tradesH = half
	}
	if tradesH < 4 {
		tradesH = 4
	}
	posH := lowerH - 1 - tradesH // 1 = divider rule
	if posH < 4 {
		posH = 4
	}

	lines := append([]string{}, topLines...)
	lines = append(lines, fitLines(m.positions(positions, posH), posH)...)
	lines = append(lines, m.rule())
	lines = append(lines, fitLines(m.recentTrades(fills, tradesH-3), tradesH)...)

	if len(lines) > contentH {
		lines = lines[:contentH]
	}
	for len(lines) < contentH {
		lines = append(lines, "")
	}
	lines = append(lines, m.footer(connected, frame))
	return strings.Join(lines, "\n")
}

// fitLines pads or clips s to exactly h lines.
func fitLines(s string, h int) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		return lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

func (m *Model) rule() string {
	return style.Dim.Render(strings.Repeat("─", m.width))
}

func (m *Model) header() string {
	sub := format.ShortAddr(m.address)
	if m.address == "" {
		sub = "manual positions"
	}
	left := style.Accent.Render("◂ ") + style.Bold.Render("Portfolio") +
		style.Label.Render("    "+sub)
	right := ""
	if !m.updated.IsZero() {
		right = style.Help.Render("↻ " + m.updated.Format("15:04:05") + " ")
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) kpis(a market.AccountSummary, ps []market.PositionView) string {
	card := func(w int, label, value string) string {
		cs := lipgloss.NewStyle().Width(w)
		return cs.Render(style.Label.Render(label)) + "\n" + cs.Render(value)
	}

	indent := "  \n  " // two-line indent keeps title and value rows aligned

	valCard := card(24, "ACCOUNT VALUE", style.Bold.Render(format.Money(a.AccountValue)))

	pnlPct := 0.0
	if a.AccountValue-a.UnrealizedPnl != 0 {
		pnlPct = a.UnrealizedPnl / (a.AccountValue - a.UnrealizedPnl) * 100
	}
	pnlVal := style.ForChange(a.UnrealizedPnl).Render(pnl(a.UnrealizedPnl) + " (" + format.Percent(pnlPct) + ")")
	pnlCard := card(26, "UNREALIZED PnL", pnlVal)

	usage := 0.0
	if a.AccountValue > 0 {
		usage = a.MarginUsed / a.AccountValue
	}
	usageVal := strconv.Itoa(int(usage*100+0.5)) + "%  " + bar(usage, 10)
	usageCard := card(26, "MARGIN USAGE", usageVal)

	label, st := health(a, ps)
	healthCard := card(28, "ACCOUNT HEALTH", st.Render(label))

	return lipgloss.JoinHorizontal(lipgloss.Top, indent, valCard, "  ", pnlCard, "  ", usageCard, "  ", healthCard)
}

func (m *Model) subLine(a market.AccountSummary) string {
	sep := style.Dim.Render("   ·   ")
	item := func(label, val string) string { return style.Label.Render(label+" ") + style.Text.Render(val) }
	parts := []string{
		item("Withdrawable", format.Money(a.Withdrawable)),
		item("Notional", "$"+format.Compact(a.TotalNtlPos)),
		item("Maint. margin", format.Money(a.MaintenanceMargin)),
		item("Positions", strconv.Itoa(a.PositionCount)),
	}
	return "  " + strings.Join(parts, sep)
}

// Renders the PnL-by-window row and an equity sparkline
func (m *Model) history(a market.AccountSummary, pf hl.Portfolio, loaded bool) string {
	if !loaded {
		if m.address == "" {
			return "  " + style.Dim.Render("PnL history and recent trades need a connected wallet.")
		}
		return "  " + style.Dim.Render("Loading history…")
	}

	win := func(label, key string) string {
		v := pf[key].PnlChange()
		return style.Label.Render(label+" ") + style.ForChange(v).Render(pnl(v))
	}
	pnlRow := "  " + style.Label.Render("PnL") + "    " +
		win("24h", "day") + "    " + win("7d", "week") + "    " +
		win("30d", "month") + "    " + win("All", "allTime") +
		style.Dim.Render("        ") +
		style.Label.Render("Vol(30d) ") + style.Text.Render("$"+format.Compact(pf["month"].Vlm))

	eq := pf["month"].Equity()
	sparkW := 48
	if sparkW > m.width-20 {
		sparkW = m.width - 20
	}
	line := chart.Line(eq, sparkW, 1)
	curve := ""
	if len(line) > 0 {
		curve = style.ForDirection(chart.Direction(eq)).Render(line[0])
	}
	eqRow := "  " + style.Label.Render("Equity ") + curve + style.Dim.Render(" (30d)")

	return pnlRow + "\n\n" + eqRow
}

// Renders the sorted positions table as a viewport fitting h lines
func (m *Model) positions(ps []market.PositionView, h int) string {
	title := "  " + style.Label.Render("POSITIONS") + style.Dim.Render(" · "+m.sort.label())
	if len(ps) == 0 {
		m.posCount, m.posVis = 0, 0
		return title + "\n\n  " + style.Dim.Render("No open positions.")
	}

	cols := []struct {
		w     int
		align lipgloss.Position
		head  string
	}{
		{8, lipgloss.Left, "COIN"},
		{6, lipgloss.Left, "SIDE"},
		{5, lipgloss.Right, "LEV"},
		{12, lipgloss.Right, "SIZE"},
		{12, lipgloss.Right, "ENTRY"},
		{12, lipgloss.Right, "MARK"},
		{12, lipgloss.Right, "LIQ"},
		{8, lipgloss.Right, "DIST"},
		{12, lipgloss.Right, "VALUE"},
		{22, lipgloss.Right, "uPnL"},
		{10, lipgloss.Right, "MARGIN"},
	}
	cell := func(i int, s string) string {
		return lipgloss.NewStyle().Width(cols[i].w).MaxWidth(cols[i].w).Align(cols[i].align).Render(s)
	}

	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = cell(i, style.Dim.Render(c.head))
	}
	header2 := "  " + strings.Join(header, " ")

	rowsBudget := h - 4 // title, blank, header, indicator
	if rowsBudget < 1 {
		rowsBudget = 1
	}
	m.posCount, m.posVis = len(ps), rowsBudget
	if maxOff := len(ps) - rowsBudget; m.posOffset > maxOff {
		m.posOffset = maxOff
	}
	if m.posOffset < 0 {
		m.posOffset = 0
	}
	end := m.posOffset + rowsBudget
	if end > len(ps) {
		end = len(ps)
	}

	var rows []string
	for _, p := range ps[m.posOffset:end] {
		side, sideStyle := "LONG", style.Up
		if !p.IsLong {
			side, sideStyle = "SHORT", style.Down
		}
		dist := style.Dim.Render("—")
		if p.LiquidationPx > 0 {
			dist = dangerStyle(p.DistToLiqPct).Render(distPct(p.DistToLiqPct))
		}
		liq := style.Dim.Render("—")
		if p.LiquidationPx > 0 {
			liq = style.Text.Render(format.Price(p.LiquidationPx, p.SzDecimals))
		}
		name := style.Bold.Render(p.Display)
		if p.Manual {
			name += style.Dim.Render("ᴹ") // manual (config-tracked) position
		}
		c := []string{
			cell(0, name),
			cell(1, sideStyle.Render(side)),
			cell(2, style.Dim.Render(strconv.Itoa(p.LeverageValue)+"x")),
			cell(3, style.Text.Render(format.Size(absf(p.Size)))),
			cell(4, style.Text.Render(format.Price(p.EntryPx, p.SzDecimals))),
			cell(5, style.Text.Render(format.Price(p.MarkPx, p.SzDecimals))),
			cell(6, liq),
			cell(7, dist),
			cell(8, style.Text.Render("$"+format.Compact(p.PositionValue))),
			cell(9, style.ForChange(p.UnrealizedPnl).Render(pnl(p.UnrealizedPnl)+" ("+format.Percent(p.ReturnOnEquity*100)+")")),
			cell(10, style.Text.Render("$"+format.Compact(p.MarginUsed))),
		}
		rows = append(rows, "  "+strings.Join(c, " "))
	}

	indicator := m.scrollIndicator(m.posOffset, len(ps)-end)
	return title + "\n\n" + header2 + "\n" + strings.Join(rows, "\n") + "\n" + indicator
}

// Centers a "▲ N above · ▼ M below" hint, "" when everything fits.
func (m *Model) scrollIndicator(above, below int) string {
	var parts []string
	if above > 0 {
		parts = append(parts, "▲ "+strconv.Itoa(above)+" more above")
	}
	if below > 0 {
		parts = append(parts, "▼ "+strconv.Itoa(below)+" more below")
	}
	if len(parts) == 0 {
		return ""
	}
	txt := strings.Join(parts, "   ")
	pad := (m.width - len([]rune(txt))) / 2
	if pad < 0 {
		pad = 0
	}
	return strings.Repeat(" ", pad) + style.Accent.Render(txt)
}

// rRenders up to maxRows of the wallet's fills, newest first
func (m *Model) recentTrades(fills []hl.Fill, maxRows int) string {
	title := "  " + style.Label.Render("RECENT TRADES")
	if len(fills) == 0 {
		return title + "\n\n  " + style.Dim.Render("No recent trades.")
	}
	if maxRows < 1 {
		maxRows = 1
	}

	cols := []struct {
		w     int
		align lipgloss.Position
		head  string
	}{
		{9, lipgloss.Left, "TIME"},
		{8, lipgloss.Left, "COIN"},
		{13, lipgloss.Left, "DIRECTION"},
		{12, lipgloss.Right, "SIZE"},
		{12, lipgloss.Right, "PRICE"},
		{12, lipgloss.Right, "VALUE"},
		{14, lipgloss.Right, "CLOSED PnL"},
		{9, lipgloss.Right, "FEE"},
	}
	cell := func(i int, s string) string {
		return lipgloss.NewStyle().Width(cols[i].w).MaxWidth(cols[i].w).Align(cols[i].align).Render(s)
	}

	header := make([]string, len(cols))
	for i, c := range cols {
		header[i] = cell(i, style.Dim.Render(c.head))
	}

	var rows []string
	for i, f := range fills {
		if i >= maxRows {
			break
		}
		dirStyle := style.Text
		if strings.Contains(f.Dir, "Long") || f.Dir == "Buy" {
			dirStyle = style.Up
		} else if strings.Contains(f.Dir, "Short") || f.Dir == "Sell" {
			dirStyle = style.Down
		}
		pnlCell := style.Dim.Render("—")
		if f.ClosedPnl != 0 {
			pnlCell = style.ForChange(f.ClosedPnl).Render(pnl(f.ClosedPnl))
		}
		c := []string{
			cell(0, style.Label.Render(time.UnixMilli(f.Time).Format("15:04:05"))),
			cell(1, style.Bold.Render(displayCoin(f.Coin))),
			cell(2, dirStyle.Render(f.Dir)),
			cell(3, style.Text.Render(format.Size(f.Sz))),
			cell(4, style.Text.Render(format.Price(f.Px, m.szDecimals(f.Coin)))),
			cell(5, style.Text.Render("$"+format.Compact(f.Px*f.Sz))),
			cell(6, pnlCell),
			cell(7, feeCell(f.Fee)),
		}
		rows = append(rows, "  "+strings.Join(c, " "))
	}

	return title + "\n\n  " + strings.Join(header, " ") + "\n" + strings.Join(rows, "\n")
}

func (m *Model) footer(connected bool, frame int) string {
	left := style.Help.Render("  esc back · ← watchlist · ") + style.Accent.Render("s sort ("+m.sort.label()+")") +
		style.Help.Render(" · ↑↓ scroll positions · p / q close")
	right := style.ConnIndicator(connected, frame) + " "
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) applySort(ps []market.PositionView) {
	switch m.sort {
	case SortPnl:
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].UnrealizedPnl > ps[j].UnrealizedPnl })
	case SortValue:
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].PositionValue > ps[j].PositionValue })
	default:
		sortByRisk(ps)
	}
}

func sortByRisk(ps []market.PositionView) {
	sort.SliceStable(ps, func(i, j int) bool {
		// closest to liquidation first; positions without a liq price sink down
		hi, hj := ps[i].LiquidationPx > 0, ps[j].LiquidationPx > 0
		if hi != hj {
			return hi
		}
		return ps[i].DistToLiqPct < ps[j].DistToLiqPct
	})
}

func health(a market.AccountSummary, ps []market.PositionView) (string, lipgloss.Style) {
	if len(ps) == 0 {
		return "No exposure", style.Label
	}
	minDist := 0.0
	for _, p := range ps {
		if p.LiquidationPx > 0 && (minDist == 0 || p.DistToLiqPct < minDist) {
			minDist = p.DistToLiqPct
		}
	}
	mmRatio := 0.0
	if a.AccountValue > 0 {
		mmRatio = a.MaintenanceMargin / a.AccountValue
	}

	switch {
	case (minDist > 0 && minDist < dangerDistPct) || mmRatio > 0.8:
		return "⚠ Danger  (liq " + distPct(minDist) + ")", style.Down
	case (minDist > 0 && minDist < cautionDistPct) || mmRatio > 0.5:
		return "Caution  (liq " + distPct(minDist) + ")", style.Warn
	default:
		return "Healthy", style.Up
	}
}

func distPct(v float64) string {
	return strconv.FormatFloat(v, 'f', 1, 64) + "%"
}

func (m *Model) szDecimals(coin string) int {
	if l, ok := m.store.Universe().Get(coin); ok {
		return l.SzDecimals
	}
	return 2
}

func displayCoin(coin string) string {
	if i := strings.IndexByte(coin, ':'); i >= 0 {
		return coin[i+1:]
	}
	return coin
}

func dangerStyle(distPct float64) lipgloss.Style {
	switch {
	case distPct < dangerDistPct:
		return style.Down
	case distPct < cautionDistPct:
		return style.Warn
	default:
		return style.Dim
	}
}

func bar(frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	st := style.Up
	switch {
	case frac > 0.8:
		st = style.Down
	case frac > 0.5:
		st = style.Warn
	}
	return st.Render(strings.Repeat("█", filled)) + style.Dim.Render(strings.Repeat("░", width-filled))
}

// Show fee rebates as green
func feeCell(fee float64) string {
	switch {
	case fee < 0:
		return style.Up.Render("+$" + feeAmt(-fee))
	case fee > 0:
		return style.Dim.Render("-$" + feeAmt(fee))
	default:
		return style.Dim.Render("$0")
	}
}

func feeAmt(v float64) string {
	dec := 2
	if v < 1 {
		dec = 4
	}
	s := strconv.FormatFloat(v, 'f', dec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

func pnl(v float64) string {
	if v >= 0 {
		return "+" + format.Money(v)
	}
	return "-" + format.Money(-v)
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
