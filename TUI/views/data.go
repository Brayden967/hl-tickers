package views

import "time"

// Drains websocket channels into the store
func (m *Model) consumeWS() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case mids := <-m.ws.Mids:
			m.store.ApplyMids(mids)
		case cu := <-m.ws.Ctxs:
			m.store.ApplyCtx(cu.Coin, cu.Ctx)
		case tu := <-m.ws.Trades:
			m.store.AddTrades(tu.Coin, tu.Trades)
			// Feed the realtime feed (volume + last trade price) if this is the open asset.
			if d := m.detail; d != nil && d.Coin() == tu.Coin {
				var buy, sell, lastPx float64
				var lastT int64
				for _, tr := range tu.Trades {
					usd := tr.Sz * tr.Px
					if tr.IsBuy {
						buy += usd
					} else {
						sell += usd
					}
					if tr.Time >= lastT {
						lastT, lastPx = tr.Time, tr.Px
					}
				}
				d.AddTradeVolume(buy, sell, lastPx)
			}
		case bk := <-m.ws.Books:
			m.store.SetBook(bk.Coin, bk)
		case <-m.ws.Errs:
		}
	}
}

func (m *Model) accountLoop() {
	t := time.NewTicker(accountInterval)
	defer t.Stop()
	for {
		m.refreshAccount()
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (m *Model) refreshAccount() {
	if m.cfg.Wallet == "" {
		return
	}
	acct, err := m.client.Account(m.ctx, m.cfg.Wallet)
	if err != nil {
		return
	}
	m.store.SetAccount(m.cfg.Wallet, acct)
}

func (m *Model) candleLoop() {
	t := time.NewTicker(candleInterval)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			for _, coin := range m.store.WatchedCoins() {
				m.fetchCandles(coin)
			}
		}
	}
}

func (m *Model) fetchCandles(coin string) {
	end := time.Now().UnixMilli()
	start := end - int64(candleHours)*3600*1000
	candles, err := m.client.CandleSnapshot(m.ctx, coin, "1h", start, end)
	if err != nil {
		return
	}
	m.store.SetCandles(coin, candles)
}

func (m *Model) reconcile() {
	want := make(map[string]bool)
	for _, coin := range m.store.WatchedCoins() {
		want[coin] = true
		if !m.subscribed[coin] {
			m.ws.Subscribe(coin)
			m.subscribed[coin] = true
		}
		if !m.candleFetching[coin] {
			m.candleFetching[coin] = true
			go m.fetchCandles(coin)
		}
	}
	for coin := range m.subscribed {
		if !want[coin] {
			m.ws.Unsubscribe(coin)
			delete(m.subscribed, coin)
			delete(m.candleFetching, coin)
		}
	}
	m.reconciledCount = len(want)
}
