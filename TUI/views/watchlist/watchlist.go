// Renders the scrollable table of watched assets
package watchlist

import (
	"strings"

	"github.com/brayden967/hl-tickers/TUI/market"
	"github.com/brayden967/hl-tickers/TUI/views/chart"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

const (
	flashTicks  = 3
	rowHeight   = 2
	rowGap      = 1
	gutter      = 2
	chartMax    = 48
	chartLabelW = 9

	wInd      = 2 // selection rail + 1 space
	wAsset    = 15
	wFunding  = 11
	wVolume   = 10
	wRange    = 20
	wPosition = 24
	wPrice    = 13
)

var (
	stAsset    = lipgloss.NewStyle().Width(wAsset).MaxWidth(wAsset).Align(lipgloss.Left)
	stFunding  = lipgloss.NewStyle().Width(wFunding).MaxWidth(wFunding).Align(lipgloss.Right)
	stVolume   = lipgloss.NewStyle().Width(wVolume).MaxWidth(wVolume).Align(lipgloss.Right)
	stRange    = lipgloss.NewStyle().Width(wRange).MaxWidth(wRange).Align(lipgloss.Left)
	stPosition = lipgloss.NewStyle().Width(wPosition).MaxWidth(wPosition).Align(lipgloss.Right)
	stPrice    = lipgloss.NewStyle().Width(wPrice).Align(lipgloss.Right)
)

func twoStyled(st lipgloss.Style, l1, l2 string) string {
	return st.Render(l1) + "\n" + st.Render(l2)
}

type Toggles struct {
	Spark   bool
	Funding bool
	Volume  bool
	Range   bool
}

type rowState struct {
	lastPrice float64
	flash     int
	dir       int
}

type Model struct {
	assets   []market.Asset
	states   map[string]*rowState
	selected int
	offset   int
	width    int
	height   int
	Toggles  Toggles
}

func New(t Toggles) *Model {
	return &Model{
		states:  make(map[string]*rowState),
		Toggles: t,
		width:   100,
		height:  20,
	}
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) visibleRows() int {
	n := (m.height - 2) / (rowHeight + rowGap)
	if n < 1 {
		n = 1
	}
	return n
}

func (m *Model) SetAssets(assets []market.Asset) {
	for i := range assets {
		a := &assets[i]
		st, ok := m.states[a.Coin]
		if !ok {
			m.states[a.Coin] = &rowState{lastPrice: a.Price}
			continue
		}
		if a.Price != st.lastPrice && st.lastPrice != 0 && a.Price != 0 {
			if a.Price > st.lastPrice {
				st.dir = 1
			} else {
				st.dir = -1
			}
			st.flash = flashTicks
		}
		st.lastPrice = a.Price
	}
	// forget coins that are no longer in the list
	if len(m.states) > len(assets) {
		live := make(map[string]bool, len(assets))
		for i := range assets {
			live[assets[i].Coin] = true
		}
		for coin := range m.states {
			if !live[coin] {
				delete(m.states, coin)
			}
		}
	}
	m.assets = assets
	if m.selected >= len(assets) {
		m.selected = max(0, len(assets)-1)
	}
}

func (m *Model) Tick() {
	for _, st := range m.states {
		if st.flash > 0 {
			st.flash--
		}
	}
}

func (m *Model) Selected() (market.Asset, bool) {
	if m.selected < 0 || m.selected >= len(m.assets) {
		return market.Asset{}, false
	}
	return m.assets[m.selected], true
}

func (m *Model) SelectCoin(coin string) {
	for i := range m.assets {
		if m.assets[i].Coin == coin {
			m.selected = i
			m.clampScroll()
			return
		}
	}
}

func (m *Model) Count() int { return len(m.assets) }

func (m *Model) MoveUp() {
	if n := len(m.assets); n > 0 {
		m.selected = (m.selected - 1 + n) % n
	}
	m.clampScroll()
}

func (m *Model) MoveDown() {
	if n := len(m.assets); n > 0 {
		m.selected = (m.selected + 1) % n
	}
	m.clampScroll()
}

// Scroll moves the cursor by delta without wrapping (used for the mouse wheel).
func (m *Model) Scroll(delta int) {
	n := len(m.assets)
	if n == 0 {
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= n {
		m.selected = n - 1
	}
	m.clampScroll()
}

func (m *Model) clampScroll() {
	vis := m.visibleRows()
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+vis {
		m.offset = m.selected - vis + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *Model) anyPosition() bool {
	for i := range m.assets {
		if m.assets[i].HasPosition {
			return true
		}
	}
	return false
}

const chartColMax = chartMax + chartLabelW + 1

func (m *Model) flexWidths(showPos bool) (chartW, gapW int) {
	fixed := wAsset + wRange + wPrice
	cols := 5 // asset, chart, range, gap, price
	if showPos {
		fixed += wPosition
		cols++
	}
	if m.Toggles.Funding {
		fixed += wFunding
		cols++
	}
	if m.Toggles.Volume {
		fixed += wVolume
		cols++
	}
	slack := m.width - fixed - gutter*(cols-1)
	if slack < 0 {
		slack = 0
	}
	chartW = slack
	if chartW > chartColMax {
		chartW = chartColMax
	}
	gapW = slack - chartW
	return chartW, gapW
}

func (m *Model) View() string {
	if m.width < 60 {
		return style.Down.Render("Terminal too narrow — widen to at least 60 columns.")
	}
	if len(m.assets) == 0 {
		return m.emptyState()
	}

	m.clampScroll()
	showPos := m.anyPosition()
	chartW, gapW := m.flexWidths(showPos)

	lines := make([]string, 0, m.height)
	lines = append(lines, m.renderHeader(chartW, gapW, showPos))
	lines = append(lines, "")

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.assets) {
		end = len(m.assets)
	}
	for i := m.offset; i < end; i++ {
		row := m.renderRow(m.assets[i], i == m.selected, chartW, gapW, showPos)
		lines = append(lines, strings.Split(row, "\n")...)
		if i < end-1 {
			lines = append(lines, "")
		}
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:m.height], "\n")
}

func (m *Model) emptyState() string {
	msg := lipgloss.JoinVertical(lipgloss.Center,
		style.Label.Render("No assets in your watchlist yet."),
		"",
		style.Text.Render("Press ")+style.Accent.Render("/")+style.Text.Render(" to search all markets — crypto, equities, commodities."),
		style.Label.Render("Press ")+style.Accent.Render("f")+style.Label.Render(" on any result to favourite it."),
	)
	box := lipgloss.NewStyle().Width(m.width).Height(m.height).Align(lipgloss.Center, lipgloss.Center)
	return box.Render(msg)
}

func (m *Model) renderHeader(chartW, gapW int, showPos bool) string {
	h := func(w int, align lipgloss.Position, label string) string {
		return lipgloss.NewStyle().Width(w).MaxWidth(w).Align(align).Render(style.Dim.Render(label))
	}
	blocks := []string{
		h(wAsset, lipgloss.Left, strings.Repeat(" ", wInd)+"MARKET"),
	}
	if chartW > 0 {
		blocks = append(blocks, m.chartHeader(chartW))
	}
	if m.Toggles.Funding {
		blocks = append(blocks, h(wFunding, lipgloss.Right, "FUND · OI"))
	}
	if m.Toggles.Volume {
		blocks = append(blocks, h(wVolume, lipgloss.Right, "24H VOL"))
	}
	blocks = append(blocks, h(wRange, lipgloss.Left, "DAY / WK RANGE"))
	blocks = append(blocks, h(gapW, lipgloss.Left, ""))
	if showPos {
		blocks = append(blocks, h(wPosition, lipgloss.Right, "POSITION"))
	}
	blocks = append(blocks, h(wPrice, lipgloss.Right, "PRICE"))
	return joinCols(blocks)
}

func (m *Model) chartHeader(cw int) string {
	trend := "24HR TREND"
	pct := "24H %"
	w := chartDrawW(cw)

	out := trend
	if pad := (w + 2) - lipgloss.Width(out); pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	out += pct
	if pad := cw - lipgloss.Width(out); pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return lipgloss.NewStyle().MaxWidth(cw).Render(style.Dim.Render(out))
}

func (m *Model) renderRow(a market.Asset, selected bool, chartW, gapW int, showPos bool) string {
	st := m.states[a.Coin]

	blocks := []string{
		m.assetCol(a, selected),
	}
	if chartW > 0 {
		blocks = append(blocks, m.chartCol(a, chartW))
	}
	if m.Toggles.Funding {
		blocks = append(blocks, m.fundingCol(a))
	}
	if m.Toggles.Volume {
		blocks = append(blocks, m.volumeCol(a))
	}
	blocks = append(blocks, m.rangeCol(a))
	blocks = append(blocks, gapCol(gapW))
	if showPos {
		blocks = append(blocks, m.positionCol(a))
	}
	blocks = append(blocks, m.priceCol(a, st))
	return joinCols(blocks)
}

func gapCol(w int) string {
	if w < 0 {
		w = 0
	}
	s := strings.Repeat(" ", w)
	return s + "\n" + s
}

func joinCols(blocks []string) string {
	gut := strings.Repeat(" ", gutter)
	parts := make([]string, 0, len(blocks)*2)
	for i, b := range blocks {
		if i > 0 {
			parts = append(parts, gut)
		}
		parts = append(parts, b)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func two(w int, align lipgloss.Position, l1, l2 string) string {
	st := lipgloss.NewStyle().Width(w).MaxWidth(w).Align(align)
	return st.Render(l1) + "\n" + st.Render(l2)
}

func (m *Model) assetCol(a market.Asset, selected bool) string {
	ind := " "
	if selected {
		ind = style.Accent.Render("│") // tiles into a continuous rail across both rows
	}
	prefix := ind + " "

	marker := ""
	switch {
	case a.IsFavourite:
		marker = style.Star.Render("★") + " "
	case a.HasPosition:
		marker = style.Accent.Render("●") + " "
	default:
		marker = "  "
	}
	sym := prefix + marker + style.Bold.Render(a.Display)
	kind := prefix + style.Dim.Render("  "+a.Kind.String())
	return twoStyled(stAsset, sym, kind)
}

func chartDrawW(cw int) int {
	w := cw - chartLabelW
	if w > chartMax {
		w = chartMax
	}
	return w
}

func (m *Model) chartCol(a market.Asset, cw int) string {
	w := chartDrawW(cw)
	if w < 8 || len(a.Spark) < 2 {
		return two(cw, lipgloss.Left, "", "")
	}
	lines := chart.LineSmoothGapped(a.Spark, w, rowHeight)
	if len(lines) < 2 {
		return two(cw, lipgloss.Left, "", "")
	}

	// colour the line by the same change that drives the % label so they agree
	st := style.Dim
	if a.PrevDayPx != 0 && a.Price != 0 {
		st = style.ForChange(a.Change)
	} else {
		st = style.ForDirection(chart.Direction(a.Spark))
	}

	label := ""
	if a.PrevDayPx != 0 && a.Price != 0 {
		label = style.ForChange(a.Change).Render(format.Percent(a.ChangePercent))
	}

	line0 := st.Render(lines[0]) + "  " + label
	line0 += strings.Repeat(" ", max(0, cw-lipgloss.Width(line0)))
	line1 := st.Render(lines[1]) + strings.Repeat(" ", cw-w)
	return line0 + "\n" + line1
}

func (m *Model) fundingCol(a market.Asset) string {
	fund := style.ForChange(-a.Funding).Render(format.Funding(a.Funding))
	mark := a.MarkPx
	if mark == 0 {
		mark = a.Price
	}
	oi := style.Dim.Render("—") // OI is reported in tokenn, show USD notional
	if a.OpenInterest > 0 && mark > 0 {
		oi = style.Text.Render("$" + format.Compact(a.OpenInterest*mark))
	}
	return twoStyled(stFunding, fund, oi)
}

func (m *Model) volumeCol(a market.Asset) string {
	if a.DayVolume <= 0 {
		return twoStyled(stVolume, style.Dim.Render("—"), "")
	}
	usd := style.Text.Render("$" + format.Compact(a.DayVolume))
	tok := style.Dim.Render("—") // token equivalent volume (notional ÷ price)
	if a.Price > 0 {
		tok = style.Label.Render(format.Compact(a.DayVolume / a.Price))
	}
	return twoStyled(stVolume, usd, tok)
}

func (m *Model) rangeCol(a market.Asset) string {
	rng := func(label string, low, high float64) string {
		lbl := style.Bold.Render(label + " ")
		if low == 0 || high == 0 {
			return lbl + style.Dim.Render("—")
		}
		return lbl +
			style.Label.Render(format.Human(low)) +
			style.Dim.Render(" – ") +
			style.Label.Render(format.Human(high))
	}
	return twoStyled(stRange,
		rng("D", a.DayLow, a.DayHigh),
		rng("W", a.WeekLow, a.WeekHigh))
}

func (m *Model) positionCol(a market.Asset) string {
	if !a.HasPosition {
		return twoStyled(stPosition, "", "")
	}
	p := a.Position
	pnl := style.ForChange(p.UnrealizedPnl).Render(
		"$" + format.Signed(p.UnrealizedPnl, 2) + " (" + format.Percent(p.ReturnOnEquity*100) + ")")

	side := "L"
	sideStyle := style.Up
	if !p.IsLong {
		side, sideStyle = "S", style.Down
	}
	meta := sideStyle.Render(side+itoa(p.LeverageValue)+"x") +
		style.Dim.Render(" "+format.Size(absf(p.Size))+"@"+format.Price(p.EntryPx, a.SzDecimals))
	if p.LiquidationPx > 0 {
		meta += style.Dim.Render(" liq " + format.Price(p.LiquidationPx, a.SzDecimals))
	}
	return twoStyled(stPosition, pnl, meta)
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (m *Model) priceCol(a market.Asset, st *rowState) string {
	priceStr := format.Price(a.Price, a.SzDecimals)
	// flash toward green/red on a price tick, then fade back
	fg := style.ColorText
	if st != nil && st.flash > 0 {
		tick := style.ColorUp
		if st.dir < 0 {
			tick = style.ColorDown
		}
		fg = style.Blend(style.ColorText, tick, float64(st.flash)/float64(flashTicks))
	}
	line1 := stPrice.Foreground(fg).Render(priceStr)

	var change string
	if a.Price == 0 || a.PrevDayPx == 0 {
		change = stPrice.Render(style.Dim.Render("—"))
	} else {
		arrow := "→"
		if a.Change > 0 {
			arrow = "↑"
		} else if a.Change < 0 {
			arrow = "↓"
		}
		txt := arrow + " " + format.Percent(a.ChangePercent)
		change = stPrice.Foreground(style.ForChange(a.Change).GetForeground()).Render(txt)
	}
	return line1 + "\n" + change
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
