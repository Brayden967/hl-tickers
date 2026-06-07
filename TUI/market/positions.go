package market

import (
	"math"

	"github.com/brayden967/hl-tickers/TUI/hl"
)

func (s *Store) SetAccount(address string, acct hl.Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.account = acct
	s.hasWallet = true
	s.address = address
	s.positions = make(map[string]hl.Position, len(acct.Positions))
	for _, p := range acct.Positions {
		s.positions[p.Coin] = p
		if !s.inList[p.Coin] {
			s.order = append(s.order, p.Coin)
			s.inList[p.Coin] = true
			s.seedSnapshot(p.Coin)
		}
	}
}

func (s *Store) SetManualPositions(ps []ManualPosition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.manual = make(map[string]ManualPosition, len(ps))
	for _, p := range ps {
		if p.Coin == "" || p.Size == 0 || p.Entry <= 0 {
			continue
		}
		s.manual[p.Coin] = p
		if !s.inList[p.Coin] {
			s.order = append(s.order, p.Coin)
			s.inList[p.Coin] = true
			s.seedSnapshot(p.Coin)
		}
	}
}

// Are manual positions configured
func (s *Store) HasManual() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.manual) > 0
}

// Builds a live hl.Position from a manual entry and the current mark price
// Liquidation/leverage are unknown for manual positions
func (s *Store) manualPosition(coin string, mp ManualPosition) hl.Position {
	mark := s.markPrice(coin)
	if mark == 0 {
		mark = mp.Entry
	}
	cost := math.Abs(mp.Size) * mp.Entry
	pnl := mp.Size * (mark - mp.Entry)
	roe := 0.0
	if cost != 0 {
		roe = pnl / cost
	}
	return hl.Position{
		Coin:           coin,
		Size:           mp.Size,
		IsLong:         mp.Size > 0,
		EntryPx:        mp.Entry,
		PositionValue:  math.Abs(mp.Size) * mark,
		UnrealizedPnl:  pnl,
		ReturnOnEquity: roe,
		LiquidationPx:  0, // unknown without leverage/margin
		MarginUsed:     cost,
		LeverageType:   "manual",
		LeverageValue:  1,
	}
}

// Returns the wallet account summary
func (s *Store) AccountSummary() AccountSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var pnl float64
	for _, p := range s.positions {
		pnl += p.UnrealizedPnl
	}
	count := len(s.positions)

	var manualPnl, manualVal, manualCost float64
	for coin, mp := range s.manual {
		if _, ok := s.positions[coin]; ok {
			continue
		}
		p := s.manualPosition(coin, mp)
		manualPnl += p.UnrealizedPnl
		manualVal += p.PositionValue
		manualCost += p.MarginUsed
		count++
	}

	sum := AccountSummary{
		HasWallet:         s.hasWallet,
		Address:           s.address,
		AccountValue:      s.account.AccountValue,
		TotalNtlPos:       s.account.TotalNtlPos,
		UnrealizedPnl:     pnl + manualPnl,
		PositionCount:     count,
		MarginUsed:        s.account.TotalMarginUsed,
		MaintenanceMargin: s.account.MaintenanceMargin,
		Withdrawable:      s.account.Withdrawable,
	}
	// Manual-only (no wallet)
	if !s.hasWallet && len(s.manual) > 0 {
		sum.AccountValue = manualCost + manualPnl
		sum.TotalNtlPos = manualVal
	}
	return sum
}

func (s *Store) Positions() []PositionView {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]PositionView, 0, len(s.positions)+len(s.manual))
	for coin, p := range s.positions {
		out = append(out, s.positionView(coin, p, false))
	}
	// Manual positions (computed live), skipping coins already held in the wallet.
	for coin, mp := range s.manual {
		if _, ok := s.positions[coin]; ok {
			continue
		}
		out = append(out, s.positionView(coin, s.manualPosition(coin, mp), true))
	}
	return out
}

func (s *Store) positionView(coin string, p hl.Position, manual bool) PositionView {
	mark := s.markPrice(coin)
	dist := 0.0
	if p.LiquidationPx > 0 && mark > 0 {
		dist = math.Abs(mark-p.LiquidationPx) / mark * 100
	}
	l, _ := s.uni.Get(coin)
	display := l.Display
	if display == "" {
		display = coin
	}
	return PositionView{
		Position:     p,
		Display:      display,
		SzDecimals:   l.SzDecimals,
		MarkPx:       mark,
		DistToLiqPct: dist,
		Manual:       manual,
	}
}
