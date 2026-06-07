package views

import (
	"strings"

	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/views/detail"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.phase == phaseLoading {
		if m.coldPrompt && !m.coldPromptDone {
			return m.handleColdWalletKey(msg)
		}
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}
		return m, nil
	}
	if m.showHelp {
		return m.handleHelpKey(msg)
	}
	if m.portfolio != nil {
		return m.handlePortfolioKey(msg)
	}
	if m.detail != nil {
		return m.handleDetailKey(msg)
	}
	if m.walletInput {
		return m.handleWalletKey(msg)
	}
	if m.searching {
		return m.handleSearchKey(msg)
	}
	return m.handleListKey(msg)
}

func (m *Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.cancel()
		return m, tea.Quit
	case "/":
		m.searching = true
		m.search.Reset()
	case "enter", "right":
		if a, ok := m.wl.Selected(); ok {
			m.openDetail(a.Coin)
		}
	case "up", "k":
		m.wl.MoveUp()
	case "down", "j":
		m.wl.MoveDown()
	case "shift+up", "K":
		if a, ok := m.wl.Selected(); ok && m.store.Move(a.Coin, -1) {
			m.sort = sortUser
			m.persistFavourites()
			m.refreshView()
			m.wl.SelectCoin(a.Coin)
		}
	case "shift+down", "J":
		if a, ok := m.wl.Selected(); ok && m.store.Move(a.Coin, 1) {
			m.sort = sortUser
			m.persistFavourites()
			m.refreshView()
			m.wl.SelectCoin(a.Coin)
		}
	case "f":
		if a, ok := m.wl.Selected(); ok {
			m.store.ToggleFavourite(a.Coin)
			m.persistFavourites()
			m.reconcile()
		}
	case "d", "x":
		if a, ok := m.wl.Selected(); ok {
			m.store.Remove(a.Coin)
			m.persistFavourites()
			m.reconcile()
		}
	case "w":
		m.walletInput = true
		m.walletBuf = m.cfg.Wallet
	case "h", "?":
		m.showHelp = true
	case "p":
		m.openPortfolio()
	case "s":
		m.sort = (m.sort + 1) % 3
	case "o", "F": // toggle the funding · OI column (F kept as a legacy alias)
		m.cfg.ShowFunding = !m.cfg.ShowFunding
		m.wl.Toggles.Funding = m.cfg.ShowFunding
		_ = m.cfg.Save()
	case "v", "V": // toggle the 24h volume column
		m.cfg.ShowVolume = !m.cfg.ShowVolume
		m.wl.Toggles.Volume = m.cfg.ShowVolume
		_ = m.cfg.Save()
	}
	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
	case "enter":
		if l, ok := m.search.Selected(); ok {
			m.store.ToggleFavourite(l.Coin)
			m.persistFavourites()
			m.reconcile()
			m.status = "Added " + l.Display
		}
		m.searching = false
	case "up":
		m.search.MoveUp()
	case "down":
		m.search.MoveDown()
	case "backspace":
		m.search.Backspace()
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit
	default:
		if msg.Type == tea.KeyRunes {
			m.search.Insert(string(msg.Runes))
		} else if msg.String() == " " {
			m.search.Insert(" ")
		}
	}
	return m, nil
}

func (m *Model) handleWalletKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.walletInput = false
	case "enter":
		addr := m.walletBuf
		if addr == "" || hl.IsValidAddress(addr) {
			m.cfg.Wallet = addr
			_ = m.cfg.SetWallet(addr)
			m.walletInput = false
			if addr != "" {
				go m.refreshAccount()
				m.status = "Loading positions…"
			} else {
				m.status = "Wallet cleared"
			}
		} else {
			m.status = "Invalid address (expected 0x + 40 hex chars)"
		}
	case "backspace":
		if r := []rune(m.walletBuf); len(r) > 0 {
			m.walletBuf = string(r[:len(r)-1])
		}
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit
	default:
		if msg.Type == tea.KeyRunes {
			m.walletBuf += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *Model) handleColdWalletKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "enter":
		addr := strings.TrimSpace(m.walletBuf)
		switch {
		case addr == "":
			m.finishColdPrompt("") // skip
		case hl.IsValidAddress(addr):
			m.finishColdPrompt(addr)
		default:
			m.status = "Invalid address — expected 0x + 40 hex chars"
		}
	case "esc":
		m.finishColdPrompt("") // skip
	case "backspace":
		if r := []rune(m.walletBuf); len(r) > 0 {
			m.walletBuf = string(r[:len(r)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.walletBuf += string(msg.Runes)
		}
	}
	return m, nil
}

// Records the first-run wallet choice and proceeds to the board once the universe is ready. An empty address skips wallet setup
func (m *Model) finishColdPrompt(addr string) {
	m.coldPromptDone = true
	m.walletBuf = ""
	if addr != "" {
		m.cfg.Wallet = addr
		_ = m.cfg.SetWallet(addr)
		if m.store != nil { // streams already up: pull positions now
			go m.refreshAccount()
		}
		m.status = "Loading positions…"
	}
	m.maybeEnterReady()
}

// Dismisses the runtime help overlay on any key (ctrl+c still quits).
func (m *Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.cancel()
		return m, tea.Quit
	}
	m.showHelp = false
	return m, nil
}

func (m *Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "left":
		m.closeDetail()
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "right":
		m.detail.TogglePanel()
	case "v":
		m.detail.VolumeBars = !m.detail.VolumeBars
	case "r":
		m.detail.Realtime = !m.detail.Realtime
	case "n":
		// Cycle order-book depth (only used and seen on detailed asset view when orderbook is toggled)
		if m.detail.Panel() == detail.PanelBook {
			agg := m.detail.CycleBookAgg()
			m.ws.SubscribeBook(m.detail.Coin(), agg.NSigFigs, agg.Mantissa)
		}
	case "t":
		m.detail.CycleTimeframe()
		m.fetchDetailCandles()
	case "f":
		m.store.ToggleFavourite(m.detail.Coin())
		m.persistFavourites()
		m.reconcile()
	}
	return m, nil
}

func (m *Model) handlePortfolioKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "p", "left", "h":
		m.closePortfolio()
	case "ctrl+c":
		m.cancel()
		return m, tea.Quit
	case "s":
		m.portfolio.CycleSort()
	case "down", "j":
		m.portfolio.ScrollPositions(1)
	case "up", "k":
		m.portfolio.ScrollPositions(-1)
	}
	return m, nil
}
