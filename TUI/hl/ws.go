package hl

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type CtxUpdate struct {
	Coin string
	Ctx  AssetCtx
}

type TradesUpdate struct {
	Coin   string
	Trades []Trade
}

type WS struct {
	url string

	Mids   chan map[string]float64 // every mid price by coin
	Ctxs   chan CtxUpdate          // per-coin funding/OI/mark/vol/prevDay
	Trades chan TradesUpdate       // per-coin live trades
	Books  chan Book               // per-coin L2 order book
	Errs   chan error

	mu         sync.Mutex
	subscribed map[string]bool
	subsTrades map[string]bool    // coins with an active trades sub
	subsBook   map[string]bookSub // coins with an active l2Book sub + its aggregation
	sendCh     chan []byte

	connected atomic.Bool // true while a live connection is serving
}

func (w *WS) Connected() bool { return w.connected.Load() }

func NewWS() *WS {
	return &WS{
		url:        MainnetWsURL,
		Mids:       make(chan map[string]float64, 8),
		Ctxs:       make(chan CtxUpdate, 256),
		Trades:     make(chan TradesUpdate, 256),
		Books:      make(chan Book, 64),
		Errs:       make(chan error, 8),
		subscribed: make(map[string]bool),
		subsTrades: make(map[string]bool),
		subsBook:   make(map[string]bookSub),
		sendCh:     make(chan []byte, 256),
	}
}

type wsSub struct {
	Type     string `json:"type"`
	Coin     string `json:"coin,omitempty"`
	NSigFigs int    `json:"nSigFigs,omitempty"`
	Mantissa int    `json:"mantissa,omitempty"`
}

type bookSub struct {
	NSigFigs int
	Mantissa int
}

func l2BookSub(coin string, b bookSub) *wsSub {
	return &wsSub{Type: "l2Book", Coin: coin, NSigFigs: b.NSigFigs, Mantissa: b.Mantissa}
}

type wsRequest struct {
	Method       string `json:"method"`
	Subscription *wsSub `json:"subscription,omitempty"`
}

type wsInbound struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

func (w *WS) Subscribe(coin string) {
	w.mu.Lock()
	if w.subscribed[coin] {
		w.mu.Unlock()
		return
	}
	w.subscribed[coin] = true
	w.mu.Unlock()

	w.enqueue(wsRequest{Method: "subscribe", Subscription: &wsSub{Type: "activeAssetCtx", Coin: coin}})
}

func (w *WS) Unsubscribe(coin string) {
	w.mu.Lock()
	if !w.subscribed[coin] {
		w.mu.Unlock()
		return
	}
	delete(w.subscribed, coin)
	w.mu.Unlock()

	w.enqueue(wsRequest{Method: "unsubscribe", Subscription: &wsSub{Type: "activeAssetCtx", Coin: coin}})
}

func (w *WS) SubscribeTrades(coin string) {
	w.mu.Lock()
	if w.subsTrades[coin] {
		w.mu.Unlock()
		return
	}
	w.subsTrades[coin] = true
	w.mu.Unlock()
	w.enqueue(wsRequest{Method: "subscribe", Subscription: &wsSub{Type: "trades", Coin: coin}})
}

func (w *WS) UnsubscribeTrades(coin string) {
	w.mu.Lock()
	if !w.subsTrades[coin] {
		w.mu.Unlock()
		return
	}
	delete(w.subsTrades, coin)
	w.mu.Unlock()
	w.enqueue(wsRequest{Method: "unsubscribe", Subscription: &wsSub{Type: "trades", Coin: coin}})
}

func (w *WS) SubscribeBook(coin string, nSigFigs, mantissa int) {
	neu := bookSub{NSigFigs: nSigFigs, Mantissa: mantissa}
	w.mu.Lock()
	old, ok := w.subsBook[coin]
	if ok && old == neu {
		w.mu.Unlock()
		return
	}
	w.subsBook[coin] = neu
	w.mu.Unlock()
	if ok {
		w.enqueue(wsRequest{Method: "unsubscribe", Subscription: l2BookSub(coin, old)})
	}
	w.enqueue(wsRequest{Method: "subscribe", Subscription: l2BookSub(coin, neu)})
}

// Removes the l2Book subscription for a coin
func (w *WS) UnsubscribeBook(coin string) {
	w.mu.Lock()
	old, ok := w.subsBook[coin]
	if !ok {
		w.mu.Unlock()
		return
	}
	delete(w.subsBook, coin)
	w.mu.Unlock()
	w.enqueue(wsRequest{Method: "unsubscribe", Subscription: l2BookSub(coin, old)})
}

func (w *WS) enqueue(req wsRequest) {
	b, err := json.Marshal(req)
	if err != nil {
		return
	}
	select {
	case w.sendCh <- b:
	default: // drop if backed up
	}
}

func (w *WS) Run(ctx context.Context) {
	const (
		baseBackoff = time.Second
		maxBackoff  = 30 * time.Second
		stableFor   = 30 * time.Second
	)
	backoff := baseBackoff
	for {
		if ctx.Err() != nil {
			return
		}

		start := time.Now()
		if err := w.connectAndServe(ctx); err != nil {
			select {
			case w.Errs <- err:
			default:
			}
		}

		if ctx.Err() != nil {
			return
		}

		if time.Since(start) >= stableFor {
			backoff = baseBackoff
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (w *WS) connectAndServe(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, w.url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	const maxWSMessageBytes = 16 << 20
	conn.SetReadLimit(maxWSMessageBytes)

	w.connected.Store(true)
	defer w.connected.Store(false)

	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	w.enqueue(wsRequest{Method: "subscribe", Subscription: &wsSub{Type: "allMids"}})
	w.mu.Lock()
	for coin := range w.subscribed {
		w.enqueue(wsRequest{Method: "subscribe", Subscription: &wsSub{Type: "activeAssetCtx", Coin: coin}})
	}
	for coin := range w.subsTrades {
		w.enqueue(wsRequest{Method: "subscribe", Subscription: &wsSub{Type: "trades", Coin: coin}})
	}
	for coin, b := range w.subsBook {
		w.enqueue(wsRequest{Method: "subscribe", Subscription: l2BookSub(coin, b)})
	}
	w.mu.Unlock()

	go func() {
		ping := time.NewTicker(20 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-connCtx.Done():
				return
			case b := <-w.sendCh:
				if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
					cancel()
					return
				}
			case <-ping.C:
				if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"method":"ping"}`)); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Handle auto reconnects on socket
	const readTimeout = 40 * time.Second
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	for {
		if connCtx.Err() != nil {
			return connCtx.Err()
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			cancel()
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		w.handle(data)
	}
}

func (w *WS) handle(data []byte) {
	var in wsInbound
	if err := json.Unmarshal(data, &in); err != nil {
		return
	}

	switch in.Channel {
	case "allMids":
		var payload struct {
			Mids map[string]string `json:"mids"`
		}
		if err := json.Unmarshal(in.Data, &payload); err != nil {
			return
		}
		mids := make(map[string]float64, len(payload.Mids))
		for k, v := range payload.Mids {
			mids[k] = parseFloat(v)
		}
		select {
		case w.Mids <- mids:
		default:
		}

	case "activeAssetCtx":
		var payload struct {
			Coin string   `json:"coin"`
			Ctx  AssetCtx `json:"ctx"`
		}
		if err := json.Unmarshal(in.Data, &payload); err != nil {
			return
		}
		select {
		case w.Ctxs <- CtxUpdate{Coin: payload.Coin, Ctx: payload.Ctx}:
		default:
		}

	case "trades":
		var raw []WsTrade
		if err := json.Unmarshal(in.Data, &raw); err != nil || len(raw) == 0 {
			return
		}
		trades := make([]Trade, 0, len(raw))
		for _, t := range raw {
			trades = append(trades, Trade{
				Coin:  t.Coin,
				IsBuy: t.Side == "B",
				Px:    parseFloat(t.Px),
				Sz:    parseFloat(t.Sz),
				Time:  t.Time,
			})
		}
		select {
		case w.Trades <- TradesUpdate{Coin: raw[0].Coin, Trades: trades}:
		default:
		}

	case "l2Book":
		var payload struct {
			Coin   string `json:"coin"`
			Levels [2][]struct {
				Px string `json:"px"`
				Sz string `json:"sz"`
				N  int    `json:"n"`
			} `json:"levels"`
		}
		if err := json.Unmarshal(in.Data, &payload); err != nil {
			return
		}
		book := Book{Coin: payload.Coin}
		for _, lv := range payload.Levels[0] {
			book.Bids = append(book.Bids, BookLevel{Px: parseFloat(lv.Px), Sz: parseFloat(lv.Sz), N: lv.N})
		}
		for _, lv := range payload.Levels[1] {
			book.Asks = append(book.Asks, BookLevel{Px: parseFloat(lv.Px), Sz: parseFloat(lv.Sz), N: lv.N})
		}
		select {
		case w.Books <- book:
		default:
		}
	}
}
