package detail

import (
	"math"
	"strings"

	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/market"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

// Hyperliquid API caps the depth of l2book
const maxBookLevels = 20

// renderBook renders the L2 order book with a summary beneath
func (m *Model) renderBook(a market.Asset, book hl.Book, w, h int) []string {
	fit := lipgloss.NewStyle().Width(w).MaxWidth(w)
	out := make([]string, 0, h)

	if len(book.Bids) == 0 && len(book.Asks) == 0 {
		out = append(out, style.Dim.Render("  loading order book…"))
		for len(out) < h {
			out = append(out, "")
		}
		return out
	}

	// Reserve two rows for the depth summary
	const summaryRows = 2
	bookRows := h - summaryRows
	if bookRows < 3 {
		bookRows = h // too short for a summary
	}
	rows := bookRows - 2
	if rows < 2 {
		rows = 2
	}
	perSide := rows / 2
	if perSide > maxBookLevels {
		perSide = maxBookLevels
	}
	nAsk, nBid := perSide, perSide
	if nAsk > len(book.Asks) {
		nAsk = len(book.Asks)
	}
	if nBid > len(book.Bids) {
		nBid = len(book.Bids)
	}

	const szW = 9
	pxW := len("PRICE")
	maxSz := 0.0
	consider := func(lv hl.BookLevel) {
		if n := len(format.Price(lv.Px, a.SzDecimals)); n > pxW {
			pxW = n
		}
		if lv.Sz > maxSz {
			maxSz = lv.Sz
		}
	}
	for i := 0; i < nAsk; i++ {
		consider(book.Asks[i])
	}
	for i := 0; i < nBid; i++ {
		consider(book.Bids[i])
	}

	barW := w - pxW - szW - 2
	if barW < 0 {
		barW = 0
	}

	out = append(out, fit.Render(style.Dim.Render(
		padR("PRICE", pxW)+" "+padR("SIZE", szW)+" "+padR("DEPTH", barW))))

	row := func(lv hl.BookLevel, st lipgloss.Style) string {
		bar := ""
		if maxSz > 0 && barW > 0 {
			n := int(lv.Sz/maxSz*float64(barW) + 0.5)
			if n > barW {
				n = barW
			}
			bar = st.Render(strings.Repeat("▰", n))
		}
		return fit.Render(st.Render(padL(format.Price(lv.Px, a.SzDecimals), pxW)) + " " +
			style.Text.Render(padR(format.Size(lv.Sz), szW)) + " " + bar)
	}

	// Best ask first
	for i := nAsk - 1; i >= 0; i-- {
		out = append(out, row(book.Asks[i], style.Down))
	}

	if len(book.Asks) > 0 && len(book.Bids) > 0 {
		spread := book.Asks[0].Px - book.Bids[0].Px
		mid := (book.Asks[0].Px + book.Bids[0].Px) / 2
		spreadPct := 0.0
		if mid > 0 {
			spreadPct = spread / mid * 100
		}
		out = append(out, fit.Render(style.Dim.Render(
			"spread "+format.Price(spread, a.SzDecimals)+" ("+format.Signed(spreadPct, 3)+"%)")))
	} else {
		out = append(out, "")
	}

	// Best bid first
	for i := 0; i < nBid; i++ {
		out = append(out, row(book.Bids[i], style.Up))
	}

	// Book depth summary for inbalances between buy vs sell pressure
	if h-len(out) >= summaryRows {
		var bidNotl, askNotl float64
		for _, lv := range book.Bids {
			bidNotl += lv.Px * lv.Sz
		}
		for _, lv := range book.Asks {
			askNotl += lv.Px * lv.Sz
		}
		imbal := 0.0
		if bidNotl+askNotl > 0 {
			imbal = (bidNotl - askNotl) / (bidNotl + askNotl) * 100
		}
		summary := style.Label.Render("bid ") + style.Up.Render("$"+format.Compact(bidNotl)) +
			style.Dim.Render("  ·  ") + style.Label.Render("ask ") + style.Down.Render("$"+format.Compact(askNotl)) +
			style.Dim.Render("  ·  ") + style.ForChange(imbal).Render(format.Signed(imbal, 1)+"% imbal")
		out = append(out, "")
		out = append(out, fit.Render(summary))
	}

	for len(out) < h {
		out = append(out, "")
	}
	return out[:h]
}

func bookTick(book hl.Book) float64 {
	tick := 0.0
	scan := func(levels []hl.BookLevel) {
		for i := 1; i < len(levels); i++ {
			d := math.Abs(levels[i].Px - levels[i-1].Px)
			if d > 0 && (tick == 0 || d < tick) {
				tick = d
			}
		}
	}
	scan(book.Asks)
	scan(book.Bids)
	return tick
}
