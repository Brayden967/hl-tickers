package hl

// Raw JSON shapes returned by the Hyperliquid /info endpoint and websocket

type Meta struct {
	Universe []MetaAsset `json:"universe"`
}

type MetaAsset struct {
	Name         string `json:"name"`
	SzDecimals   int    `json:"szDecimals"`
	MaxLeverage  int    `json:"maxLeverage"`
	OnlyIsolated bool   `json:"onlyIsolated"`
	IsDelisted   bool   `json:"isDelisted"`
}

type AssetCtx struct {
	Funding      string   `json:"funding"`
	OpenInterest string   `json:"openInterest"`
	PrevDayPx    string   `json:"prevDayPx"`
	DayNtlVlm    string   `json:"dayNtlVlm"`
	Premium      string   `json:"premium"`
	OraclePx     string   `json:"oraclePx"`
	MarkPx       string   `json:"markPx"`
	MidPx        string   `json:"midPx"`
	ImpactPxs    []string `json:"impactPxs"`
}

// account state
type ClearinghouseState struct {
	AssetPositions             []AssetPositionWrapper `json:"assetPositions"`
	MarginSummary              MarginSummary          `json:"marginSummary"`
	CrossMarginSummary         MarginSummary          `json:"crossMarginSummary"`
	CrossMaintenanceMarginUsed string                 `json:"crossMaintenanceMarginUsed"`
	Withdrawable               string                 `json:"withdrawable"`
	Time                       int64                  `json:"time"`
}

// portfolio
type PortfolioPoint struct {
	Time  int64
	Value float64
}

type PortfolioPeriod struct {
	AccountValue []PortfolioPoint
	Pnl          []PortfolioPoint
	Vlm          float64
}

func (p PortfolioPeriod) PnlChange() float64 {
	if len(p.Pnl) < 2 {
		if len(p.Pnl) == 1 {
			return p.Pnl[0].Value
		}
		return 0
	}
	return p.Pnl[len(p.Pnl)-1].Value - p.Pnl[0].Value
}

// Equity returns the account-value series for chart
func (p PortfolioPeriod) Equity() []float64 {
	out := make([]float64, len(p.AccountValue))
	for i, pt := range p.AccountValue {
		out[i] = pt.Value
	}
	return out
}

type Portfolio map[string]PortfolioPeriod

type AssetPositionWrapper struct {
	Type     string      `json:"type"`
	Position RawPosition `json:"position"`
}

type RawPosition struct {
	Coin           string        `json:"coin"`
	Szi            string        `json:"szi"`
	EntryPx        string        `json:"entryPx"`
	PositionValue  string        `json:"positionValue"`
	UnrealizedPnl  string        `json:"unrealizedPnl"`
	ReturnOnEquity string        `json:"returnOnEquity"`
	LiquidationPx  string        `json:"liquidationPx"`
	MarginUsed     string        `json:"marginUsed"`
	MaxLeverage    int           `json:"maxLeverage"`
	Leverage       RawLeverage   `json:"leverage"`
	CumFunding     RawCumFunding `json:"cumFunding"`
}

type RawLeverage struct {
	Type   string `json:"type"` // isolated, cross
	Value  int    `json:"value"`
	RawUsd string `json:"rawUsd"`
}

type RawCumFunding struct {
	AllTime     string `json:"allTime"`
	SinceOpen   string `json:"sinceOpen"`
	SinceChange string `json:"sinceChange"`
}

type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

//perp dexs

type PerpDex struct {
	Name                  string  `json:"name"`
	FullName              string  `json:"full_name"`
	AssetToStreamingOiCap [][]any `json:"assetToStreamingOiCap"`
}

//trades

type WsTrade struct {
	Coin string `json:"coin"`
	Side string `json:"side"`
	Px   string `json:"px"`
	Sz   string `json:"sz"`
	Time int64  `json:"time"`
	Tid  int64  `json:"tid"`
	Hash string `json:"hash"`
}

type Trade struct {
	Coin  string
	IsBuy bool
	Px    float64
	Sz    float64
	Time  int64
}

type Fill struct {
	Coin      string
	Dir       string // "Open Long", "Close Short", "Buy", "Sell", …
	IsBuy     bool
	Px        float64
	Sz        float64
	ClosedPnl float64
	Fee       float64
	Time      int64
}

//order book

// one price level of the L2 order book
type BookLevel struct {
	Px float64
	Sz float64
	N  int // number of orders at this level
}

type Book struct {
	Coin string
	Bids []BookLevel
	Asks []BookLevel
}

//candles

type Candle struct {
	T  int64  `json:"t"` // open time (ms)
	TC int64  `json:"T"` // close time (ms)
	S  string `json:"s"` // symbol
	I  string `json:"i"` // interval
	O  string `json:"o"`
	C  string `json:"c"`
	H  string `json:"h"`
	L  string `json:"l"`
	V  string `json:"v"`
	N  int    `json:"n"`
}
