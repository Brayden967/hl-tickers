package views

import (
	"context"
	"sort"
	"time"

	"github.com/brayden967/hl-tickers/TUI/config"
	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/market"
	"github.com/brayden967/hl-tickers/TUI/views/detail"
	"github.com/brayden967/hl-tickers/TUI/views/portfolio"
	"github.com/brayden967/hl-tickers/TUI/views/search"
	"github.com/brayden967/hl-tickers/TUI/views/watchlist"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	tickInterval    = 120 * time.Millisecond
	accountInterval = 8 * time.Second
	candleInterval  = 60 * time.Second
	candleHours     = 168 // 7 days of hourly candles: 24h trend chart + day & week ranges
	cacheMaxAge     = 24 * time.Hour
	detailRefresh   = 15 * time.Second
)

type phase int

const (
	phaseLoading phase = iota
	phaseReady
)

type sortMode int

const (
	sortUser sortMode = iota
	sortChange
	sortAlpha
)

func (s sortMode) label() string {
	switch s {
	case sortChange:
		return "change"
	case sortAlpha:
		return "alpha"
	default:
		return "manual"
	}
}

type Model struct {
	ctx    context.Context
	cancel context.CancelFunc

	client     *hl.Client
	cfg        *config.Config
	addSymbols []string

	phase        phase
	spinnerFrame int
	loadErr      error

	store     *market.Store
	uni       *hl.Universe
	ws        *hl.WS
	wl        *watchlist.Model
	search    *search.Model
	detail    *detail.Model
	portfolio *portfolio.Model

	detailCancel    context.CancelFunc
	portfolioCancel context.CancelFunc

	width, height int
	ready         bool

	searching   bool
	walletInput bool
	walletBuf   string

	showHelp bool // 'h' help overlay (also the cold-start screen)

	sort   sortMode
	acct   market.AccountSummary
	status string

	subscribed      map[string]bool
	candleFetching  map[string]bool
	reconciledCount int // watched-coin count
}

type tickMsg time.Time

type universeMsg struct {
	uni       *hl.Universe
	err       error
	fromCache bool
}

func New(client *hl.Client, cfg *config.Config, addSymbols []string) *Model {
	ctx, cancel := context.WithCancel(context.Background())
	return &Model{
		ctx:        ctx,
		cancel:     cancel,
		client:     client,
		cfg:        cfg,
		addSymbols: addSymbols,
		phase:      phaseLoading,
		wl: watchlist.New(watchlist.Toggles{
			Spark:   cfg.ShowSpark,
			Funding: cfg.ShowFunding,
			Volume:  cfg.ShowVolume,
			Range:   cfg.ShowRange,
		}),
		subscribed:     make(map[string]bool),
		candleFetching: make(map[string]bool),
		status:         "Press / to search markets",
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) }),
		m.loadUniverse,
	)
}

// Loads the market universe from cache (if fresh) otherwise makes request
func (m *Model) loadUniverse() tea.Msg {
	if uni, ok := config.LoadMarkets(cacheMaxAge); ok {
		return universeMsg{uni: uni, fromCache: true}
	}
	uni, complete, err := hl.BuildUniverse(m.ctx, m.client)
	if err == nil && complete {
		_ = config.SaveMarkets(uni, time.Now())
	}
	return universeMsg{uni: uni, err: err}
}

// Rebuilds the universe in the background and updates the cache
// so the next launch is current (used after a cache hit)
func (m *Model) refreshCache() {
	uni, complete, err := hl.BuildUniverse(m.ctx, m.client)
	if err == nil && complete {
		_ = config.SaveMarkets(uni, time.Now())
	}
}

func (m *Model) onUniverseReady(msg universeMsg) {
	m.uni = msg.uni
	m.store = market.NewStore(msg.uni)
	m.search = search.New(msg.uni, m.store.Price, m.store.IsWatched)

	favs := m.cfg.Favourites
	if len(favs) == 0 {
		favs = resolveDefaults(msg.uni, config.DefaultBoard)
	}
	m.store.SeedFavourites(favs)
	anyAdded := false
	for _, sym := range m.addSymbols {
		if l, ok := msg.uni.Resolve(sym); ok {
			if m.store.AddFavourite(l.Coin) {
				anyAdded = true
			}
		}
	}
	if anyAdded {
		m.persistFavourites()
	}

	// Manual positions: resolve friendly coin names to known ids
	if len(m.cfg.ManualPositions) > 0 {
		mps := make([]market.ManualPosition, 0, len(m.cfg.ManualPositions))
		for _, p := range m.cfg.ManualPositions {
			if l, ok := msg.uni.Resolve(p.Coin); ok {
				mps = append(mps, market.ManualPosition{Coin: l.Coin, Size: p.Size, Entry: p.Entry})
			}
		}
		m.store.SetManualPositions(mps)
	}

	m.ws = hl.NewWS()
	go m.ws.Run(m.ctx)
	go m.consumeWS()
	go m.accountLoop()
	go m.candleLoop()
	if msg.fromCache {
		go m.refreshCache()
	}
	m.reconcile()
	m.maybeEnterReady()
}

// Flips to the ready phase once the market list has loaded
func (m *Model) maybeEnterReady() {
	if m.phase == phaseReady || m.uni == nil {
		return
	}
	m.phase = phaseReady
	m.layout()
	// Populate the watchlist before the first board render so it doesn't flash
	// the empty state for one frame before the next tick's refreshView runs.
	m.refreshView()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.layout()
		// Repaint from scratch so stale cells from the old size don't linger.
		return m, tea.ClearScreen

	case universeMsg:
		if msg.err != nil || msg.uni == nil {
			m.loadErr = msg.err
			return m, nil
		}
		m.onUniverseReady(msg)
		m.layout()
		return m, nil

	case tickMsg:
		m.spinnerFrame++
		if m.phase == phaseReady {
			if m.store.WatchedCount() != m.reconciledCount {
				m.reconcile()
			}
			m.refreshView()
		}
		return m, tea.Tick(tickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	return m, nil
}

// Handles the scroll wheel. One notch moves exactly one row
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	delta := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		delta = -1
	case tea.MouseButtonWheelDown:
		delta = 1
	default:
		return m, nil
	}

	if m.portfolio != nil {
		m.portfolio.ScrollPositions(delta)
		return m, nil
	}
	listFocused := m.phase == phaseReady && m.detail == nil &&
		!m.searching && !m.walletInput && !m.showHelp
	if listFocused {
		m.wl.Scroll(delta)
	}
	return m, nil
}

func (m *Model) layout() {
	if m.detail != nil {
		m.detail.SetSize(m.width, m.height)
	}
	if m.portfolio != nil {
		m.portfolio.SetSize(m.width, m.height)
	}
	headerH := 0
	if m.cfg.ShowSummary {
		headerH = 2
	}
	footerH := 1
	topPad := 1 // matches the leading blank line in View()
	listH := m.height - topPad - headerH - footerH
	if listH < 1 {
		listH = 1
	}
	m.wl.SetSize(m.width, listH)
	if m.search != nil {
		m.search.SetWidth(m.width)
	}
}

func (m *Model) refreshView() {
	assets := m.store.Snapshot()
	m.applySort(assets)
	m.wl.Tick()
	m.wl.SetAssets(assets)
	m.acct = m.store.AccountSummary()
}

func (m *Model) applySort(assets []market.Asset) {
	switch m.sort {
	case sortChange:
		sort.SliceStable(assets, func(i, j int) bool {
			return assets[i].ChangePercent > assets[j].ChangePercent
		})
	case sortAlpha:
		sort.SliceStable(assets, func(i, j int) bool {
			return assets[i].Display < assets[j].Display
		})
	}
}

func (m *Model) persistFavourites() {
	_ = m.cfg.SetFavourites(m.store.Favourites())
}

func (m *Model) View() string {
	if m.phase == phaseLoading {
		return m.renderLoading()
	}
	if !m.ready {
		return "\n  Initialising…"
	}

	if m.showHelp {
		return m.renderHelp()
	}
	connected := m.ws != nil && m.ws.Connected()
	if m.portfolio != nil {
		return m.portfolio.View(connected, m.spinnerFrame)
	}
	if m.detail != nil {
		return m.detail.View(connected, m.spinnerFrame)
	}

	body := "\n" // top padding so the board isn't flush against terminal tabs
	if m.cfg.ShowSummary {
		body += m.renderSummary() + "\n"
	}
	body += m.wl.View() + "\n"
	body += m.renderFooter()

	if m.searching {
		return m.overlay(body, m.search.View())
	}
	if m.walletInput {
		return m.overlay(body, m.renderWalletPrompt())
	}
	return body
}
