package views

import (
	"context"
	"time"

	"github.com/brayden967/hl-tickers/TUI/views/portfolio"
)

// Opens the wallet portfolio view. With neither a wallet or any manual positions configured it shows the wallet entry popup instead
func (m *Model) openPortfolio() {
	if m.cfg.Wallet == "" && !m.store.HasManual() {
		m.walletInput = true
		m.walletBuf = ""
		m.status = "Add a wallet or manual positions to view your portfolio"
		return
	}
	m.portfolio = portfolio.New(m.store, m.cfg.Wallet)
	m.portfolio.SetSize(m.width, m.height)

	pctx, pcancel := context.WithCancel(m.ctx)
	m.portfolioCancel = pcancel
	go m.refreshAccount() // immediate freshness for live data
	m.fetchPortfolio()
	go m.portfolioRefreshLoop(pctx)
}

func (m *Model) closePortfolio() {
	if m.portfolio == nil {
		return
	}
	if m.portfolioCancel != nil {
		m.portfolioCancel()
		m.portfolioCancel = nil
	}
	m.portfolio = nil
}

func (m *Model) portfolioRefreshLoop(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if m.portfolio != nil {
				m.fetchPortfolio()
				go m.refreshAccount()
			}
		}
	}
}

func (m *Model) fetchPortfolio() {
	p := m.portfolio
	if p == nil {
		return
	}
	addr := p.Address()
	if addr == "" {
		return // manual-only: no wallet history/fills to fetch
	}
	go func() {
		pf, err := m.client.Portfolio(m.ctx, addr)
		if err != nil {
			return
		}
		if m.portfolio != nil && m.portfolio.Address() == addr {
			m.portfolio.SetPortfolio(pf, time.Now())
		}
	}()
	go func() {
		fills, err := m.client.UserFills(m.ctx, addr)
		if err != nil {
			return
		}
		if m.portfolio != nil && m.portfolio.Address() == addr {
			m.portfolio.SetFills(fills)
		}
	}()
}
