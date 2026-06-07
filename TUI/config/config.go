package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// ManualPosition is a user-tracked position with no connected wallet
type ManualPosition struct {
	Coin  string  `yaml:"coin"`
	Size  float64 `yaml:"size"`
	Entry float64 `yaml:"entry"`
}

type Config struct {
	Wallet          string           `yaml:"wallet"`
	Favourites      []string         `yaml:"favourites"`
	ManualPositions []ManualPosition `yaml:"manual_positions"`

	ShowFunding bool `yaml:"show_funding"`
	ShowVolume  bool `yaml:"show_volume"`
	ShowRange   bool `yaml:"show_range"`
	ShowSummary bool `yaml:"show_summary"`
	ShowSpark   bool `yaml:"show_spark"`

	// Ephemeral disables persistence (used in tests).
	Ephemeral bool `yaml:"-"`

	path string     `yaml:"-"`
	mu   sync.Mutex `yaml:"-"`
}

// Default asset list if none are provided by user
var DefaultBoard = []string{"BTC", "xyz:GOLD", "xyz:NVDA", "xyz:AAPL", "xyz:SP500", "xyz:CL", "HYPE"}

func xdgPath(envVar, fallbackDir string, parts ...string) string {
	base := os.Getenv(envVar)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.Getenv("HOME")
		}
		base = filepath.Join(home, fallbackDir)
	}
	return filepath.Join(append([]string{base}, parts...)...)
}

func Path() string {
	return xdgPath("XDG_CONFIG_HOME", ".config", "hl-tickers", "config.yaml")
}

func (c *Config) FilePath() string { return c.path }

// Load reads the config file, returning defaults if it does not exist.
func Load() (*Config, error) {
	path := Path()
	cfg := &Config{
		path:        path,
		ShowSpark:   true,
		ShowSummary: true,
		ShowRange:   true,
		ShowFunding: true,
		ShowVolume:  true,
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}
	cfg.path = path
	return cfg, nil
}

// Provides sample config parameters in the generated config file
const configHeader = `# hl-tickers configuration
#
# Edit this file while the app is closed. Changing settings in-app rewrites the
# file and will overwrite any manual edits.
#
# wallet: optional public 0x address used to load your live perp positions.
#   wallet: "0xabc...def"
#
# favourites: coin ids on your board, in display order.
#   favourites:
#     - BTC
#     - xyz:GOLD
#
# manual_positions: track positions without a wallet. size is signed
# (+ long / - short); PnL and value are computed live from the mark price.
#   manual_positions:
#     - coin: BTC
#       size: 0.5
#       entry: 60000
#     - coin: ETH
#       size: -2
#       entry: 3500
#
# Display toggles (also changeable in-app):
#   show_funding: funding-rate / open-interest column (key: o)
#   show_volume:  24h volume column (key: v)
#   show_range:   day / week range column
#   show_summary: top account / market summary bar
#   show_spark:   inline 24h trend chart

`

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Ephemeral {
		return nil
	}
	if c.path == "" {
		c.path = Path()
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}

	body, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	data := append([]byte(configHeader), body...)

	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

func (c *Config) EnsureFile() error {
	if c.Ephemeral || c.path == "" {
		return nil
	}
	if _, err := os.Stat(c.path); err == nil {
		return nil // already exists
	}
	return c.Save()
}

func (c *Config) SetFavourites(coins []string) error {
	c.mu.Lock()
	c.Favourites = coins
	c.mu.Unlock()
	return c.Save()
}

func (c *Config) SetWallet(addr string) error {
	c.mu.Lock()
	c.Wallet = addr
	c.mu.Unlock()
	return c.Save()
}
