package detail

import (
	"time"

	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/market"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderTrades(a market.Asset, trades []hl.Trade, w, h int) []string {
	const (
		timeW = 9
		pxW   = 13
		szW   = 10
		usdW  = 12
	)
	fit := lipgloss.NewStyle().Width(w).MaxWidth(w)

	lines := make([]string, 0, h)
	lines = append(lines, fit.Render(style.Dim.Render(
		padR("TIME", timeW)+padL("PRICE", pxW)+padL("SIZE", szW)+padL("USD", usdW))))

	for i := 0; i < len(trades) && len(lines) < h; i++ {
		t := trades[i]
		ts := time.UnixMilli(t.Time).Format("15:04:05")
		sideStyle := style.Down
		if t.IsBuy {
			sideStyle = style.Up
		}
		row := style.Label.Render(padR(ts, timeW)) +
			sideStyle.Render(padL(format.Price(t.Px, a.SzDecimals), pxW)) +
			style.Text.Render(padL(format.Size(t.Sz), szW)) +
			style.Label.Render(padL("$"+format.Compact(t.Px*t.Sz), usdW))
		lines = append(lines, fit.Render(row))
	}
	if len(trades) == 0 {
		lines = append(lines, style.Dim.Render("  waiting for trades…"))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}
