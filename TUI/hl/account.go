package hl

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
)

// parsed perp position for one coin.
type Position struct {
	Coin             string
	Size             float64
	IsLong           bool
	EntryPx          float64
	PositionValue    float64
	UnrealizedPnl    float64
	ReturnOnEquity   float64
	LiquidationPx    float64
	MarginUsed       float64
	LeverageType     string
	LeverageValue    int
	FundingSinceOpen float64
}

// parsed wallet account state.
type Account struct {
	AccountValue      float64
	TotalNtlPos       float64
	TotalMarginUsed   float64
	MaintenanceMargin float64
	Withdrawable      float64
	Positions         []Position
}

// IsValidAddress does a cheap sanity check on a 0x addresses
func IsValidAddress(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 42 || !strings.HasPrefix(s, "0x") {
		return false
	}
	for _, r := range s[2:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

// Fetches and parses a wallet's perp account state
func (c *Client) Account(ctx context.Context, user string) (Account, error) {
	state, err := c.ClearinghouseState(ctx, user)
	if err != nil {
		return Account{}, err
	}

	acct := Account{
		AccountValue:      parseFloat(state.MarginSummary.AccountValue),
		TotalNtlPos:       parseFloat(state.MarginSummary.TotalNtlPos),
		TotalMarginUsed:   parseFloat(state.MarginSummary.TotalMarginUsed),
		MaintenanceMargin: parseFloat(state.CrossMaintenanceMarginUsed),
		Withdrawable:      parseFloat(state.Withdrawable),
		Positions:         make([]Position, 0, len(state.AssetPositions)),
	}

	for _, w := range state.AssetPositions {
		p := w.Position
		size := parseFloat(p.Szi)
		if size == 0 {
			continue
		}
		acct.Positions = append(acct.Positions, Position{
			Coin:             p.Coin,
			Size:             size,
			IsLong:           size > 0,
			EntryPx:          parseFloat(p.EntryPx),
			PositionValue:    parseFloat(p.PositionValue),
			UnrealizedPnl:    parseFloat(p.UnrealizedPnl),
			ReturnOnEquity:   parseFloat(p.ReturnOnEquity),
			LiquidationPx:    parseFloat(p.LiquidationPx),
			MarginUsed:       parseFloat(p.MarginUsed),
			LeverageType:     p.Leverage.Type,
			LeverageValue:    p.Leverage.Value,
			FundingSinceOpen: parseFloat(p.CumFunding.SinceOpen),
		})
	}

	return acct, nil
}

// Fetches a wallet's historical account-value / PnL / volume
func (c *Client) Portfolio(ctx context.Context, user string) (Portfolio, error) {
	var raw [][]json.RawMessage
	if err := c.info(ctx, map[string]string{"type": "portfolio", "user": user}, &raw); err != nil {
		return nil, err
	}

	out := make(Portfolio, len(raw))
	for _, pair := range raw {
		if len(pair) != 2 {
			continue
		}
		var name string
		if err := json.Unmarshal(pair[0], &name); err != nil {
			continue
		}
		var pd struct {
			AccountValueHistory [][2]json.RawMessage `json:"accountValueHistory"`
			PnlHistory          [][2]json.RawMessage `json:"pnlHistory"`
			Vlm                 string               `json:"vlm"`
		}
		if err := json.Unmarshal(pair[1], &pd); err != nil {
			continue
		}
		out[name] = PortfolioPeriod{
			AccountValue: parsePoints(pd.AccountValueHistory),
			Pnl:          parsePoints(pd.PnlHistory),
			Vlm:          parseFloat(pd.Vlm),
		}
	}
	return out, nil
}

// Fetches the wallet's recent executed fills
func (c *Client) UserFills(ctx context.Context, user string) ([]Fill, error) {
	var raw []struct {
		Coin      string `json:"coin"`
		Px        string `json:"px"`
		Sz        string `json:"sz"`
		Side      string `json:"side"`
		Dir       string `json:"dir"`
		ClosedPnl string `json:"closedPnl"`
		Fee       string `json:"fee"`
		Time      int64  `json:"time"`
	}
	if err := c.info(ctx, map[string]string{"type": "userFills", "user": user}, &raw); err != nil {
		return nil, err
	}

	out := make([]Fill, 0, len(raw))
	for _, f := range raw {
		out = append(out, Fill{
			Coin:      f.Coin,
			Dir:       f.Dir,
			IsBuy:     f.Side == "B",
			Px:        parseFloat(f.Px),
			Sz:        parseFloat(f.Sz),
			ClosedPnl: parseFloat(f.ClosedPnl),
			Fee:       parseFloat(f.Fee),
			Time:      f.Time,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out, nil
}

func parsePoints(raw [][2]json.RawMessage) []PortfolioPoint {
	out := make([]PortfolioPoint, 0, len(raw))
	for _, pt := range raw {
		var ts int64
		var v string
		_ = json.Unmarshal(pt[0], &ts)
		_ = json.Unmarshal(pt[1], &v)
		out = append(out, PortfolioPoint{Time: ts, Value: parseFloat(v)})
	}
	return out
}
