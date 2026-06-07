package views

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/views/format"
	"github.com/brayden967/hl-tickers/TUI/views/style"
	"github.com/charmbracelet/lipgloss"
)

// Some terminals allow click to copy. Copy config address if terminal supports otherwise render text
func fileHyperlink(path, text string) string {
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	return "\x1b]8;;" + uri + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func (m *Model) renderSummary() string {
	var left, right string

	if m.acct.HasWallet && m.acct.Address != "" {
		pnlStyle := style.ForChange(m.acct.UnrealizedPnl)
		left = style.Accent.Render("HL ") +
			style.Label.Render(format.ShortAddr(m.acct.Address)) +
			style.Dim.Render("  •  ") +
			style.Label.Render("Equity ") + style.Text.Render(format.Money(m.acct.AccountValue)) +
			style.Dim.Render("  •  ") +
			style.Label.Render("uPnL ") + pnlStyle.Render(format.Signed(m.acct.UnrealizedPnl, 2)) +
			style.Dim.Render("  •  ") +
			style.Label.Render("Positions ") + style.Text.Render(strconv.Itoa(m.acct.PositionCount))
	} else {
		left = style.Accent.Render("HL tickers ") +
			style.Label.Render("• ") +
			style.Text.Render(strconv.Itoa(m.wl.Count())) + style.Label.Render(" markets watched") +
			style.Dim.Render("  •  ") +
			style.Label.Render("press ") + style.Text.Render("w") + style.Label.Render(" to add a wallet for live positions")
	}

	rule := style.Dim.Render(strings.Repeat("─", m.width))
	bar := lipgloss.NewStyle().Width(m.width).MaxWidth(m.width).Render(left + right)
	return bar + "\n" + rule
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func splashHeader() string {
	tagline := style.Dim.Render("crypto · equities · commodities — one source, no keys")
	return lipgloss.JoinVertical(lipgloss.Center, "", tagline)
}

func (m *Model) renderLoading() string {
	w, h := m.width, m.height
	if w == 0 {
		w, h = 80, 24
	}

	var line string
	switch {
	case m.loadErr != nil:
		line = style.Down.Render("✗ Could not reach Hyperliquid: " + m.loadErr.Error())
	default:
		spin := style.Accent.Render(spinnerFrames[m.spinnerFrame%len(spinnerFrames)])
		line = spin + "  " + style.Label.Render("Loading markets from Hyperliquid…")
	}

	content := lipgloss.JoinVertical(lipgloss.Center, splashHeader(), "", line)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) renderHelp() string {
	w, h := m.width, m.height
	if w == 0 {
		w, h = 80, 24
	}
	key := func(k, desc string) string {
		ks := lipgloss.NewStyle().Width(13).Foreground(style.ColorAccent).Bold(true).Render(k)
		return ks + style.Label.Render(desc)
	}
	keys := lipgloss.JoinVertical(lipgloss.Left,
		key("/", "search all markets"),
		key("↑ ↓ · j k", "move selection"),
		key("shift+↑↓ · K J", "reorder favourite"),
		key("enter", "open asset explorer (chart + trades)"),
		key("f", "favourite / unfavourite"),
		key("d · x", "remove from watchlist"),
		key("w", "add / change wallet"),
		key("p", "portfolio / positions"),
		key("s", "cycle sort (manual · change% · alpha)"),
		key("o · v", "toggle funding/OI · volume columns"),
		key("h · ?", "this help"),
		key("q · esc", "quit"),
	)
	// Shows the exact config file this run reads/writes
	cfgLine := ""
	if p := m.cfg.FilePath(); p != "" {
		cfgLine = style.Dim.Render("config  ") + fileHyperlink(p, style.Label.Render(p))
	}

	hint := style.Help.Render("press any key to close")
	content := lipgloss.JoinVertical(lipgloss.Center, splashHeader(), "", keys, "", cfgLine, "", hint)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) renderFooter() string {
	logo := style.Logo.Render(" hl-tickers ")
	right := style.ConnIndicator(m.ws != nil && m.ws.Connected(), m.spinnerFrame) + " "

	favLabel := "f favourite"
	if a, ok := m.wl.Selected(); ok && a.IsFavourite {
		favLabel = "f unfavourite"
	}
	hints := []string{"/ search", "↵ explore", "p portfolio", favLabel, "w wallet", "h help", "q quit"}

	// Keep as many hints as fit, dropping from the right rather than hiding them all.
	avail := m.width - lipgloss.Width(logo) - lipgloss.Width(right)
	plain := ""
	for i, h := range hints {
		seg := h
		if i > 0 {
			seg = " · " + h
		}
		if lipgloss.Width(" "+plain+seg) > avail {
			break
		}
		plain += seg
	}
	help := style.Help.Render(" " + plain)

	gap := avail - lipgloss.Width(help)
	if gap < 0 {
		gap = 0
	}
	return logo + help + strings.Repeat(" ", gap) + right
}

// inline wallet-entry
func (m *Model) renderWalletPrompt() string {
	title := style.Accent.Render("Add wallet address")
	hint := style.Label.Render("Paste a public 0x address to pull live perp positions.")
	input := style.Text.Render(m.walletBuf) + style.Accent.Render("▏")
	field := lipgloss.NewStyle().
		Width(46).
		Border(lipgloss.NormalBorder()).
		BorderForeground(style.ColorDim).
		Padding(0, 1).
		Render(input)

	var status string
	switch buf := strings.TrimSpace(m.walletBuf); {
	case buf == "":
		status = style.Dim.Render("Leave empty to clear a saved wallet.")
	case hl.IsValidAddress(buf):
		status = style.Up.Render("✓ valid address")
	default:
		status = style.Down.Render("✗ invalid — expected 0x followed by 40 hex characters")
	}

	help := style.Help.Render("enter save · esc cancel")

	box := lipgloss.NewStyle().
		Width(54).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(style.ColorAccent).
		Padding(1, 2)
	return box.Render(strings.Join([]string{title, hint, "", field, "", status, help}, "\n"))
}

func (m *Model) overlay(body, box string) string {
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceChars(" "))
}
