package views

import (
	"context"
	"strconv"
	"time"

	"github.com/brayden967/hl-tickers/TUI/views/detail"
)

func (m *Model) openDetail(coin string) {
	m.detail = detail.New(coin, m.store)
	m.detail.SetSize(m.width, m.height)
	m.ws.Subscribe(coin) // ensure ctx stream
	m.subscribed[coin] = true
	m.ws.SubscribeTrades(coin)
	agg := m.detail.BookAggParams()
	m.ws.SubscribeBook(coin, agg.NSigFigs, agg.Mantissa)

	dctx, dcancel := context.WithCancel(m.ctx)
	m.detailCancel = dcancel
	m.fetchDetailCandles()
	go m.detailRefreshLoop(dctx, coin)
	go m.realtimeSampler(dctx, coin)
}

// Tracks real-time prices for live chart when toggled
func (m *Model) realtimeSampler(ctx context.Context, coin string) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			d := m.detail
			if d == nil || d.Coin() != coin {
				return
			}
			d.SampleRealtime(m.store.Price(coin))
		}
	}
}

func (m *Model) closeDetail() {
	if m.detail == nil {
		return
	}
	coin := m.detail.Coin()
	if m.detailCancel != nil {
		m.detailCancel()
		m.detailCancel = nil
	}
	m.ws.UnsubscribeTrades(coin)
	m.ws.UnsubscribeBook(coin)
	m.store.ClearTrades(coin)
	m.store.ClearBook(coin)
	m.detail = nil
}

func (m *Model) detailRefreshLoop(ctx context.Context, coin string) {
	t := time.NewTicker(detailRefresh)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if m.detail != nil && m.detail.Coin() == coin {
				m.fetchDetailCandles()
			}
		}
	}
}

func (m *Model) fetchDetailCandles() {
	d := m.detail
	if d == nil {
		return
	}
	coin := d.Coin()
	tf := d.Timeframe()
	go func() {
		end := time.Now().UnixMilli()
		start := end - barDurationMs(tf.Interval)*int64(tf.Bars)
		cs, err := m.client.CandleSnapshot(m.ctx, coin, tf.Interval, start, end)
		if err != nil {
			return
		}
		closes := make([]float64, 0, len(cs))
		volumes := make([]float64, 0, len(cs))
		for _, c := range cs {
			if v, err := strconv.ParseFloat(c.C, 64); err == nil {
				closes = append(closes, v)
				vol, _ := strconv.ParseFloat(c.V, 64)
				volumes = append(volumes, vol)
			}
		}
		if m.detail != nil && m.detail.Coin() == coin {
			m.detail.SetCandles(closes, volumes)
		}
	}()
}
