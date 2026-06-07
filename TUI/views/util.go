package views

import "github.com/brayden967/hl-tickers/TUI/hl"

func barDurationMs(interval string) int64 {
	switch interval {
	case "1m":
		return 60 * 1000
	case "3m":
		return 3 * 60 * 1000
	case "5m":
		return 5 * 60 * 1000
	case "15m":
		return 15 * 60 * 1000
	case "30m":
		return 30 * 60 * 1000
	case "1h":
		return 3600 * 1000
	case "2h":
		return 2 * 3600 * 1000
	case "4h":
		return 4 * 3600 * 1000
	case "1d":
		return 24 * 3600 * 1000
	default:
		return 3600 * 1000
	}
}

// Maps curated default symbols to coin ids present in the HL universe
func resolveDefaults(uni *hl.Universe, defaults []string) []string {
	out := make([]string, 0, len(defaults))
	for _, sym := range defaults {
		if l, ok := uni.Resolve(sym); ok {
			out = append(out, l.Coin)
		}
	}
	return out
}
