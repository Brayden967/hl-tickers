package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/brayden967/hl-tickers/TUI/hl"
)

const marketCacheVersion = 1

type marketCache struct {
	Version   int          `json:"version"`
	FetchedAt int64        `json:"fetched_at"`
	Listings  []hl.Listing `json:"listings"`
}

func MarketsCachePath() string {
	return xdgPath("XDG_CACHE_HOME", ".cache", "hl-tickers", "markets.json")
}

// LoadMarkets returns a cached asset list if the cache exists
func LoadMarkets(maxAge time.Duration) (*hl.Universe, bool) {
	data, err := os.ReadFile(MarketsCachePath())
	if err != nil {
		return nil, false
	}
	var mc marketCache
	if err := json.Unmarshal(data, &mc); err != nil {
		return nil, false
	}
	if mc.Version != marketCacheVersion || len(mc.Listings) == 0 {
		return nil, false
	}
	age := time.Since(time.Unix(mc.FetchedAt, 0))
	if age < 0 || age > maxAge {
		return nil, false
	}
	return hl.NewUniverse(mc.Listings), true
}

// SaveMarkets writes the asset list fetch to the cache file for faster subsequant starts
func SaveMarkets(uni *hl.Universe, now time.Time) error {
	mc := marketCache{
		Version:   marketCacheVersion,
		FetchedAt: now.Unix(),
		Listings:  uni.Listings,
	}
	data, err := json.Marshal(mc)
	if err != nil {
		return err
	}
	path := MarketsCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
