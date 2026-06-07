package market

import (
	"strconv"
	"sync"

	"github.com/brayden967/hl-tickers/TUI/hl"
)

type Store struct {
	mu  sync.RWMutex
	uni *hl.Universe

	order      []string        // coin ids in display order (the watchlist)
	inList     map[string]bool // membership in order
	favourites map[string]bool // favourited coins

	price    map[string]float64 // every mid price seen (all of allMids)
	ctx      map[string]hl.AssetCtx
	spark    map[string][]float64
	dayHigh  map[string]float64
	dayLow   map[string]float64
	weekHigh map[string]float64
	weekLow  map[string]float64

	positions map[string]hl.Position
	manual    map[string]ManualPosition // config-tracked positions (no wallet)
	account   hl.Account
	hasWallet bool
	address   string

	trades map[string][]hl.Trade // recent trades per coin
	books  map[string]hl.Book    // latest L2 order book per coin
}

const maxTrades = 100

func NewStore(uni *hl.Universe) *Store {
	return &Store{
		uni:        uni,
		inList:     make(map[string]bool),
		favourites: make(map[string]bool),
		price:      make(map[string]float64, 512),
		ctx:        make(map[string]hl.AssetCtx, 64),
		spark:      make(map[string][]float64, 64),
		dayHigh:    make(map[string]float64, 64),
		dayLow:     make(map[string]float64, 64),
		weekHigh:   make(map[string]float64, 64),
		weekLow:    make(map[string]float64, 64),
		positions:  make(map[string]hl.Position),
		manual:     make(map[string]ManualPosition),
		trades:     make(map[string][]hl.Trade),
		books:      make(map[string]hl.Book),
	}
}

func (s *Store) SetBook(coin string, book hl.Book) {
	s.mu.Lock()
	s.books[coin] = book
	s.mu.Unlock()
}

func (s *Store) Book(coin string) hl.Book {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.books[coin]
}

func (s *Store) ClearBook(coin string) {
	s.mu.Lock()
	delete(s.books, coin)
	s.mu.Unlock()
}

func (s *Store) AddTrades(coin string, trades []hl.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Newest first.
	buf := make([]hl.Trade, 0, maxTrades)
	for i := len(trades) - 1; i >= 0; i-- {
		buf = append(buf, trades[i])
	}
	buf = append(buf, s.trades[coin]...)
	if len(buf) > maxTrades {
		buf = buf[:maxTrades]
	}
	s.trades[coin] = buf
}

func (s *Store) Trades(coin string) []hl.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.trades[coin]
	out := make([]hl.Trade, len(src))
	copy(out, src)
	return out
}

func (s *Store) ClearTrades(coin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.trades, coin)
}

func (s *Store) AssetByCoin(coin string) Asset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buildAsset(coin)
}

func (s *Store) Universe() *hl.Universe { return s.uni }

// Returns the best available mark for a coin (live mid, else mark)
func (s *Store) markPrice(coin string) float64 {
	if p := s.price[coin]; p != 0 {
		return p
	}
	return parseF(s.ctx[coin].MarkPx)
}

func parseF(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
