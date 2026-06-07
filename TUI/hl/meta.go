package hl

import (
	"context"
	"strings"
)

// Classifies an asset for display/grouping.
type Kind int

const (
	KindCrypto Kind = iota
	KindEquity
	KindCommodity
	KindForex
	KindIndex
	KindOther
)

func (k Kind) String() string {
	switch k {
	case KindCrypto:
		return "crypto"
	case KindEquity:
		return "equity"
	case KindCommodity:
		return "commodity"
	case KindForex:
		return "forex"
	case KindIndex:
		return "index"
	default:
		return "perp"
	}
}

type Listing struct {
	Coin       string // exact coin id used in API calls (e.g. "BTC", "xyz:GOLD")
	Display    string // short symbol shown in the UI (e.g. "BTC", "GOLD")
	Dex        string // "" for native crypto, otherwise builder dex name
	Kind       Kind
	SzDecimals int
	Snapshot   AssetCtx // initial market context
}

type Universe struct {
	Listings []Listing
	byCoin   map[string]int
	byUpper  map[string]int
}

// Maps common spoken names to coin ids (optional, helps with discovery)
var aliasTable = map[string]string{
	"OIL":    "xyz:CL",
	"CRUDE":  "xyz:CL",
	"WTI":    "xyz:CL",
	"GOLD":   "xyz:GOLD",
	"XAU":    "xyz:GOLD",
	"SILVER": "xyz:SI",
	"XAG":    "xyz:SI",
	"SPX":    "xyz:SP500",
	"SP500":  "xyz:SP500",
	"NDX":    "xyz:NDX",
	"NASDAQ": "xyz:NDX",
	"PEPE":   "kPEPE",
	"SHIB":   "kSHIB",
	"BONK":   "kBONK",

	"EURUSD": "xyz:EUR",
	"CADUSD": "xyz:CAD",
	"USDJPY": "xyz:JPY",
	"GBPUSD": "xyz:GBP",
	"USDKRW": "xyz:KRW",
	"DOLLAR": "xyz:DXY",
	"USDX":   "xyz:DXY",
}

func Aliases() map[string]string {
	out := make(map[string]string, len(aliasTable))
	for k, v := range aliasTable {
		out[k] = v
	}
	return out
}

// classify guesses an asset Kind from its dex and symbol. Needs to be re-worked.
func classify(dex, sym string) Kind {
	if dex == "" {
		return KindCrypto
	}
	up := strings.ToUpper(sym)
	switch up {
	case "GOLD", "SILVER", "CL", "WTI", "BRENT", "NG", "NGAS", "HG", "SI", "PL", "PA", "XAU", "XAG", "COPPER":
		return KindCommodity
	case "SP500", "SPX500", "NDX", "NAS100", "NASDAQ", "DJI", "US30", "RUT", "VIX", "DAX", "FTSE":
		return KindIndex
	}
	// HL quotes currencies as bare codes vs USD (EUR, JPY…) plus the dollar index.
	if up == "DXY" || (len(up) == 3 && fiat[up]) {
		return KindForex
	}
	if len(up) == 6 && isForexPair(up) {
		return KindForex
	}
	return KindEquity
}

var fiat = map[string]bool{
	"USD": true, "EUR": true, "JPY": true, "GBP": true, "CHF": true,
	"AUD": true, "CAD": true, "NZD": true, "CNH": true, "HKD": true,
	"KRW": true, "SGD": true, "SEK": true, "NOK": true, "MXN": true,
	"INR": true, "CNY": true, "BRL": true, "ZAR": true, "TRY": true,
}

func isForexPair(up string) bool {
	return fiat[up[:3]] && fiat[up[3:]]
}

// BuildUniverse fetches the native crypto perps plus every builder dex (HIP-3)
func BuildUniverse(ctx context.Context, c *Client) (*Universe, bool, error) {
	listings := make([]Listing, 0, 512)

	// 1. Native crypto perps.
	meta, ctxs, err := c.MetaAndAssetCtxs(ctx, "")
	if err != nil {
		return nil, false, err
	}
	for i, a := range meta.Universe {
		if a.IsDelisted {
			continue
		}
		var snap AssetCtx
		if i < len(ctxs) {
			snap = ctxs[i]
		}
		listings = append(listings, Listing{
			Coin:       a.Name,
			Display:    a.Name,
			Dex:        "",
			Kind:       KindCrypto,
			SzDecimals: a.SzDecimals,
			Snapshot:   snap,
		})
	}

	// 2. Builder-deployed perps (equities/commodities/fx/indices).
	complete := true
	dexs, err := c.PerpDexs(ctx)
	if err != nil {
		complete = false
	} else {
		for _, d := range dexs {
			if d == nil || d.Name == "" {
				continue
			}
			dmeta, dctxs, derr := c.MetaAndAssetCtxs(ctx, d.Name)
			if derr != nil {
				complete = false
				continue
			}
			for i, a := range dmeta.Universe {
				if a.IsDelisted {
					continue
				}
				// Builder-dex names may arrive already prefixed ("xyz:GOLD").
				coin := a.Name
				bare := a.Name
				if idx := strings.IndexByte(a.Name, ':'); idx >= 0 {
					bare = a.Name[idx+1:]
				} else {
					coin = d.Name + ":" + a.Name
				}
				var snap AssetCtx
				if i < len(dctxs) {
					snap = dctxs[i]
				}
				listings = append(listings, Listing{
					Coin:       coin,
					Display:    bare,
					Dex:        d.Name,
					Kind:       classify(d.Name, bare),
					SzDecimals: a.SzDecimals,
					Snapshot:   snap,
				})
			}
		}
	}

	u := &Universe{Listings: listings}
	u.Index()
	return u, complete, nil
}

// Builds and indexes a Universe from existing listings (e.g. a cache).
func NewUniverse(listings []Listing) *Universe {
	u := &Universe{Listings: listings}
	u.Index()
	return u
}

// Index (re)builds the coin/display lookup maps
func (u *Universe) Index() {
	u.byCoin = make(map[string]int, len(u.Listings))
	u.byUpper = make(map[string]int, len(u.Listings)*2)
	for i, l := range u.Listings {
		u.byCoin[l.Coin] = i
		if _, ok := u.byUpper[strings.ToUpper(l.Display)]; !ok {
			u.byUpper[strings.ToUpper(l.Display)] = i
		}
		u.byUpper[strings.ToUpper(l.Coin)] = i
	}
}

func (u *Universe) Get(coin string) (Listing, bool) {
	i, ok := u.byCoin[coin]
	if !ok {
		return Listing{}, false
	}
	return u.Listings[i], true
}

// Resolve maps a user-entered symbol to a coin id, trying in order: exact coin id,
// xyz:<UP> builder symbol, alias table, then case-insensitive display match. The
// xyz/alias steps come first so well-known tickers land on the canonical listing.
func (u *Universe) Resolve(input string) (Listing, bool) {
	q := strings.TrimSpace(input)
	if q == "" {
		return Listing{}, false
	}

	if l, ok := u.Get(q); ok {
		return l, true
	}

	up := strings.ToUpper(q)

	if l, ok := u.Get("xyz:" + up); ok {
		return l, true
	}

	if coin, ok := aliasTable[up]; ok {
		if l, ok := u.Get(coin); ok {
			return l, true
		}
	}

	if i, ok := u.byUpper[up]; ok {
		return u.Listings[i], true
	}

	return Listing{}, false
}
