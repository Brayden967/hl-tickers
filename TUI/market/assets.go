package market

import (
	"github.com/brayden967/hl-tickers/TUI/hl"
)

// Price returns the latest mid price for any coin (0 if unknown)
func (s *Store) Price(coin string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if p, ok := s.price[coin]; ok && p != 0 {
		return p
	}
	if l, ok := s.uni.Get(coin); ok {
		return parseF(l.Snapshot.MidPx)
	}
	return 0
}

// Merge a full mid-price snapshot
func (s *Store) ApplyMids(mids map[string]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range mids {
		s.price[k] = v
	}
}

func (s *Store) ApplyCtx(coin string, ctx hl.AssetCtx) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx[coin] = ctx
}

func (s *Store) SetCandles(coin string, candles []hl.Candle) {
	if len(candles) == 0 {
		return
	}

	// Trend chart uses the most recent 24h of closes.
	dayStart := len(candles) - 24
	if dayStart < 0 {
		dayStart = 0
	}
	spark := make([]float64, 0, len(candles)-dayStart)
	for i := dayStart; i < len(candles); i++ {
		spark = append(spark, parseF(candles[i].C))
	}

	dayHigh, dayLow := candleRange(candles[dayStart:])
	weekHigh, weekLow := candleRange(candles)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.spark[coin] = spark
	s.dayHigh[coin] = dayHigh
	s.dayLow[coin] = dayLow
	s.weekHigh[coin] = weekHigh
	s.weekLow[coin] = weekLow
}

// Returns the high/low across a slice of candles
func candleRange(candles []hl.Candle) (high, low float64) {
	for i, cd := range candles {
		h, l := parseF(cd.H), parseF(cd.L)
		if i == 0 {
			high, low = h, l
		}
		if h > high {
			high = h
		}
		if l < low || low == 0 {
			low = l
		}
	}
	return high, low
}

func (s *Store) Snapshot() []Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Asset, 0, len(s.order))
	for _, coin := range s.order {
		out = append(out, s.buildAsset(coin))
	}
	return out
}

// Merges all sources for one coin
func (s *Store) buildAsset(coin string) Asset {
	l, _ := s.uni.Get(coin)
	a := Asset{
		Coin:        coin,
		Display:     l.Display,
		Kind:        l.Kind,
		SzDecimals:  l.SzDecimals,
		IsFavourite: s.favourites[coin],
		Spark:       s.spark[coin],
		DayHigh:     s.dayHigh[coin],
		DayLow:      s.dayLow[coin],
		WeekHigh:    s.weekHigh[coin],
		WeekLow:     s.weekLow[coin],
	}
	if a.Display == "" {
		a.Display = coin
	}

	price := s.price[coin]
	ctx, hasCtx := s.ctx[coin]
	if hasCtx {
		a.MarkPx = parseF(ctx.MarkPx)
		a.OraclePx = parseF(ctx.OraclePx)
		a.PrevDayPx = parseF(ctx.PrevDayPx)
		a.Funding = parseF(ctx.Funding)
		a.OpenInterest = parseF(ctx.OpenInterest)
		a.DayVolume = parseF(ctx.DayNtlVlm)
		if price == 0 {
			price = parseF(ctx.MidPx)
		}
		if price == 0 {
			price = a.MarkPx
		}
	}
	a.Price = price

	if a.PrevDayPx != 0 && a.Price != 0 {
		a.Change = a.Price - a.PrevDayPx
		a.ChangePercent = (a.Change / a.PrevDayPx) * 100
	}

	if p, ok := s.positions[coin]; ok {
		a.HasPosition = true
		a.Position = p
	} else if mp, ok := s.manual[coin]; ok {
		a.HasPosition = true
		a.Manual = true
		a.Position = s.manualPosition(coin, mp)
	}

	return a
}
