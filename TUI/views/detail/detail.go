package detail

import (
	"math"
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

type Timeframe struct {
	Label    string
	Interval string
	Bars     int
}

var Timeframes = []Timeframe{
	{"15M", "1m", 15},
	{"1H", "1m", 60},
	{"4H", "5m", 48},
	{"24H", "15m", 96},
	{"7D", "1h", 168},
	{"30D", "4h", 180},
}

type Panel int

const (
	PanelTrades Panel = iota
	PanelBook
)

type Model struct {
	coin  string
	store *market.Store

	width, height int
	tfIndex       int
	panel         Panel
	bookAggIdx    int

	// VolumeBars shows the experimental volume histogram under the chart ('v')
	VolumeBars bool
	// Realtime swaps the candle chart for a live 1-second series ('r')
	Realtime bool

	mu            sync.Mutex
	candles       []float64
	volumes       []float64
	rt            []rtPoint // live 1-second samples
	rtBuy, rtSell float64   // taker USD accumulating for the current second
	rtLastPx      float64   // most recent trade price
}

type rtPoint struct {
	price, buy, sell float64
}

const rtMaxPoints = 300

type BookAgg struct {
	NSigFigs int
	Mantissa int
}

var bookAggLevels = []BookAgg{
	{},            // finest (default)
	{5, 2},        // 2×
	{5, 5},        // 5×
	{NSigFigs: 4}, // 10×
	{NSigFigs: 3}, // 100×
	{NSigFigs: 2}, // 1000×
}

func New(coin string, store *market.Store) *Model {
	return &Model{coin: coin, store: store, tfIndex: 3, VolumeBars: true} // default 24H
}

func (m *Model) Coin() string { return m.coin }

// Returns the active timeframe.
func (m *Model) Timeframe() Timeframe { return Timeframes[m.tfIndex] }

// Advances to the next timeframe and returns it.
func (m *Model) CycleTimeframe() Timeframe {
	m.tfIndex = (m.tfIndex + 1) % len(Timeframes)
	m.mu.Lock()
	m.candles = nil // clear until new candles arrive
	m.volumes = nil
	m.mu.Unlock()
	return Timeframes[m.tfIndex]
}

func (m *Model) Panel() Panel { return m.panel }

func (m *Model) TogglePanel() Panel {
	if m.panel == PanelTrades {
		m.panel = PanelBook
	} else {
		m.panel = PanelTrades
	}
	return m.panel
}

func (m *Model) BookAggParams() BookAgg { return bookAggLevels[m.bookAggIdx] }

func (m *Model) CycleBookAgg() BookAgg {
	m.bookAggIdx = (m.bookAggIdx + 1) % len(bookAggLevels)
	return bookAggLevels[m.bookAggIdx]
}

func (m *Model) SetSize(w, h int) { m.width, m.height = w, h }

func (m *Model) SetCandles(closes, volumes []float64) {
	m.mu.Lock()
	m.candles = closes
	m.volumes = volumes
	m.mu.Unlock()
}

func (m *Model) closes() []float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]float64, len(m.candles))
	copy(out, m.candles)
	return out
}

func (m *Model) vols() []float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]float64, len(m.volumes))
	copy(out, m.volumes)
	return out
}

func (m *Model) AddTradeVolume(buy, sell, lastPx float64) {
	m.mu.Lock()
	m.rtBuy += buy
	m.rtSell += sell
	if lastPx > 0 {
		m.rtLastPx = lastPx
	}
	m.mu.Unlock()
}

func (m *Model) SampleRealtime(midFallback float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	price := m.rtLastPx // hold at the last trade price
	if price <= 0 {
		price = midFallback // no trade yet — start from the mid
	}
	if price <= 0 && len(m.rt) > 0 {
		price = m.rt[len(m.rt)-1].price
	}
	if price <= 0 {
		return // no price yet — wait
	}
	m.rt = append(m.rt, rtPoint{price, m.rtBuy, m.rtSell})
	m.rtBuy, m.rtSell = 0, 0
	if len(m.rt) > rtMaxPoints {
		m.rt = append([]rtPoint(nil), m.rt[len(m.rt)-rtMaxPoints:]...)
	}
}

func (m *Model) chartData() (closes, volUsd, volBuy, volSell []float64, isRT bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Realtime && len(m.rt) >= 2 {
		for _, p := range m.rt {
			closes = append(closes, p.price)
			volBuy = append(volBuy, p.buy)
			volSell = append(volSell, p.sell)
			volUsd = append(volUsd, p.buy+p.sell)
		}
		return closes, volUsd, volBuy, volSell, true
	}
	closes = make([]float64, len(m.candles))
	copy(closes, m.candles)
	for i := 0; i < len(m.volumes) && i < len(m.candles); i++ {
		u := m.volumes[i] * m.candles[i] // coins × price = USD
		volUsd = append(volUsd, u)
		// Candle direction for buy/sell
		if i == 0 || m.candles[i] >= m.candles[i-1] {
			volBuy = append(volBuy, u)
			volSell = append(volSell, 0)
		} else {
			volBuy = append(volBuy, 0)
			volSell = append(volSell, u)
		}
	}
	return closes, volUsd, volBuy, volSell, false
}

func (m *Model) View(connected bool, frame int) string {
	a := m.store.AssetByCoin(m.coin)
	trades := m.store.Trades(m.coin)
	closes, volUsd, volBuy, volSell, isRT := m.chartData()

	header := m.renderHeader(a)

	bodyH := m.height - 5
	if bodyH < 4 {
		bodyH = 4
	}
	overhead := axisWidth + chartPad*2 + 3
	inner := m.width - overhead
	tradesW := tradesTableWidth
	if tradesW > inner-24 {
		tradesW = inner - 24
	}
	if tradesW < 18 {
		tradesW = 18
	}
	chartW := inner - tradesW
	if chartW < 20 {
		chartW = 20
	}

	body := m.renderBody(a, trades, chartW, tradesW, bodyH, closes, volUsd, volBuy, volSell, isRT)
	footer := m.renderFooter(a, connected, frame)

	return "\n" + header + "\n\n" + body + "\n\n" + footer
}

func (m *Model) renderHeader(a market.Asset) string {
	price := style.Bold.Render(format.Price(a.Price, a.SzDecimals))
	chg := style.Dim.Render("—")
	if a.PrevDayPx != 0 && a.Price != 0 {
		arrow := "→"
		if a.Change > 0 {
			arrow = "↑"
		} else if a.Change < 0 {
			arrow = "↓"
		}
		chg = style.ForChange(a.Change).Render(arrow + " " + format.Signed(a.Change, 2) + " (" + format.Percent(a.ChangePercent) + ")")
	}
	title := style.Accent.Render("◂ ") + style.Bold.Render(a.Display)
	if a.IsFavourite {
		title += " " + style.Star.Render("★")
	}
	title += style.Dim.Render("  " + a.Kind.String())

	pair := func(label, val string) string {
		return style.Label.Render(label+" ") + style.Text.Render(val)
	}
	sep := style.Dim.Render("  •  ")
	mark := a.MarkPx
	if mark == 0 {
		mark = a.Price
	}
	stats := pair("Funding", style.ForChange(-a.Funding).Render(format.Funding(a.Funding))) + sep +
		pair("OI", format.Compact(a.OpenInterest)+style.Dim.Render(" ($"+format.Compact(a.OpenInterest*mark)+")")) + sep +
		pair("24h Vol", "$"+format.Compact(a.DayVolume))

	left := title + style.Label.Render("    ") + price + "  " + chg + sep + stats
	return lipgloss.NewStyle().Width(m.width).Render(left)
}

func (m *Model) renderBody(a market.Asset, trades []hl.Trade, chartW, tradesW, h int, closesIn, volUsd, volBuy, volSell []float64, isRT bool) string {
	rightLines := m.rightPanel(a, trades, tradesW, h)
	pad := strings.Repeat(" ", chartPad)

	if m.Realtime && !isRT {
		waiting := m.renderWaiting(chartW, h)
		out := make([]string, h)
		for i := 0; i < h; i++ {
			right := ""
			if i < len(rightLines) {
				right = rightLines[i]
			}
			cl := lipgloss.NewStyle().Width(chartW).MaxWidth(chartW).Render(waiting[i])
			out[i] = strings.Repeat(" ", axisWidth) + pad + cl + pad +
				style.Dim.Render(" │ ") + right
		}
		return strings.Join(out, "\n")
	}

	closes := append([]float64(nil), closesIn...)
	if !isRT && a.Price > 0 {
		closes = append(closes, a.Price)
	}

	dir := chart.Direction(closes)
	cst := style.ForDirection(dir)
	lo, hi := format.MinMax(closes)

	priceH, volH := h, 0
	const divH = 1
	if m.VolumeBars && len(volUsd) > 0 {
		volH = h / 4
		if volH > 6 {
			volH = 6
		}
		if volH < 3 || h-volH-divH < 6 {
			volH = 0 // too short to be worth it
		}
		if volH > 0 {
			priceH = h - volH - divH
		}
	}
	// Keep the price area an odd number of rows so the gridlines — every other row,
	// H pinned to the top and L to the bottom always divide evenly
	if priceH%2 == 0 && priceH > 1 {
		priceH--
	}

	chartLines := m.renderArea(closes, chartW, priceH, dir, hi, lo)
	var fullBar float64
	if volH > 0 {
		var volLines []string
		volLines, fullBar = m.renderVolume(volUsd, volBuy, volSell, chartW, volH)
		chartLines = append(chartLines, volDivider(chartW)) // "·····volume·····" separator
		chartLines = append(chartLines, volLines...)
	}

	// The live marker sits on the row of the last point
	liveRow := -1
	if len(closes) > 0 {
		if hi > lo {
			liveRow = int(math.Round((hi - closes[len(closes)-1]) / (hi - lo) * float64(priceH-1)))
		} else {
			liveRow = (priceH - 1) / 2
		}
		if liveRow < 0 {
			liveRow = 0
		} else if liveRow >= priceH {
			liveRow = priceH - 1
		}
	}
	// The chevron tracks the same trend as the chart line
	liveStyle := cst
	chevronRow := -1
	if liveRow >= 0 && priceH >= 3 {
		chevronRow = liveRow
		if chevronRow < 1 {
			chevronRow = 1
		} else if chevronRow > priceH-2 {
			chevronRow = priceH - 2
		}
	}
	// Width of a price label, so a chevron on a row with no gridline price still
	// aligns in the H/L marker column instead of jumping to the right edge
	repWidth := lipgloss.Width(format.Price(hi, a.SzDecimals))

	leftAxis := lipgloss.NewStyle().Width(axisWidth).MaxWidth(axisWidth).Align(lipgloss.Right)

	out := make([]string, h)
	for i := 0; i < h; i++ {
		cl := strings.Repeat(" ", chartW)
		if i < len(chartLines) {
			cl = chartLines[i] // already styled per-cell by renderArea
		}

		leftLabel := ""
		switch {
		case hi > lo && i < priceH && i%2 == 0:
			v := hi - (float64(i)/float64(priceH-1))*(hi-lo)
			price := format.Price(v, a.SzDecimals)
			switch i {
			case 0:
				leftLabel = style.Bold.Render("H ") + style.Bold.Render(price)
			case priceH - 1:
				leftLabel = style.Bold.Render("L ") + style.Bold.Render(price)
			default:
				leftLabel = style.Dim.Render(price)
			}
		case volH > 0 && i >= priceH+divH:
			leftLabel = volAxisLabel(i-priceH-divH, volH, fullBar)
		}
		if i == chevronRow {
			chev := liveStyle.Bold(true).Render("▶ ")
			if leftLabel == "" {
				leftLabel = chev + strings.Repeat(" ", repWidth)
			} else {
				leftLabel = chev + leftLabel
			}
		}

		chartBlock := lipgloss.NewStyle().Width(chartW).MaxWidth(chartW).Render(cl)
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}
		out[i] = leftAxis.Render(leftLabel) + pad + chartBlock + pad +
			style.Dim.Render(" │ ") + right
	}
	return strings.Join(out, "\n")
}

func (m *Model) rightPanel(a market.Asset, trades []hl.Trade, tradesW, h int) []string {
	if m.panel == PanelBook {
		return m.renderBook(a, m.store.Book(m.coin), tradesW, h)
	}
	return m.renderTrades(a, trades, tradesW, h)
}

func (m *Model) renderWaiting(width, height int) []string {
	out := chart.Blank(width, height)
	const waitAnimMs = 500
	n := 1 + int(time.Now().UnixMilli()/waitAnimMs)%3 // 1–3 dots, one step / 500ms
	dots := strings.Repeat(".", n) + strings.Repeat(" ", 3-n)
	msg := style.Up.Render("● ") + style.Label.Render("waiting for live trades"+dots)
	row := height / 2
	if row >= 0 && row < height {
		out[row] = lipgloss.NewStyle().Width(width).MaxWidth(width).Align(lipgloss.Center).Render(msg)
	}
	return out
}

const (
	axisWidth        = 11
	chartPad         = 2
	tradesTableWidth = 47
)

func (m *Model) renderFooter(a market.Asset, connected bool, frame int) string {
	favLabel := "f favourite"
	if a.IsFavourite {
		favLabel = "f unfavourite"
	}
	// The → hint flips to whatever panel you'd switch to
	panelHint := "→ order book"
	if m.panel == PanelBook {
		panelHint = "→ live trades"
	}
	rtState := "off"
	tfHint := style.Help.Render("t timeframe (" + m.Timeframe().Label + ")")
	if m.Realtime {
		rtState = style.Up.Render("on")
		tfHint = style.Dim.Render("t timeframe")
	}
	left := style.Help.Render("  esc/← back · r live: ") + rtState +
		style.Help.Render(" · ") + tfHint +
		style.Help.Render(" · "+favLabel+" · q quit · ") + style.Bold.Render(panelHint)

	right := style.ConnIndicator(connected, frame) + " "
	if m.panel == PanelBook {
		label := "n depth"
		if tick := bookTick(m.store.Book(m.coin)); tick > 0 {
			label += " (" + format.Price(tick, a.SzDecimals) + ")"
		}
		right = style.Accent.Render(label) + style.Help.Render("   ") + right
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

func padR(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return s + strings.Repeat(" ", n-len(r))
}

func padL(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return string(r[:n])
	}
	return strings.Repeat(" ", n-len(r)) + s
}
