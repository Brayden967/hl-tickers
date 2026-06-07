package market

import "github.com/brayden967/hl-tickers/TUI/hl"

// Asset is the fully-merged view of one watchlist entry
type Asset struct {
	Coin        string
	Display     string
	Kind        hl.Kind
	SzDecimals  int
	IsFavourite bool

	Price        float64
	PrevDayPx    float64
	MarkPx       float64
	OraclePx     float64
	Funding      float64 // hourly funding rate (fraction)
	OpenInterest float64
	DayVolume    float64
	DayHigh      float64
	DayLow       float64
	WeekHigh     float64
	WeekLow      float64

	Change        float64
	ChangePercent float64

	HasPosition bool
	Position    hl.Position
	Manual      bool // position is a config-tracked manual position, not a wallet one

	Spark []float64 // recent closes for the inline sparkline
}

type AccountSummary struct {
	HasWallet         bool
	Address           string
	AccountValue      float64
	TotalNtlPos       float64
	UnrealizedPnl     float64
	PositionCount     int
	MarginUsed        float64
	MaintenanceMargin float64
	Withdrawable      float64
}

type PositionView struct {
	hl.Position
	Display      string
	SzDecimals   int
	MarkPx       float64
	DistToLiqPct float64 // % distance from mark to liquidation (0 = no liq price)
	Manual       bool    // config-tracked manual position, not from a wallet
}

// ManualPosition is a user-tracked position (no wallet). Size is signed: positive = long, negative = short. PnL is calculated live from the mark price.
type ManualPosition struct {
	Coin  string
	Size  float64
	Entry float64
}
