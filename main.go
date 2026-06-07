// hl-tickers is a fast terminal market watcher. It shows live crypto, equity, and commodity prices
// and — with an optional public wallet address — your live perp positions. No API keys, no login, and 100% free.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/brayden967/hl-tickers/TUI/config"
	"github.com/brayden967/hl-tickers/TUI/hl"
	"github.com/brayden967/hl-tickers/TUI/views"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var version = "v0.1.0"

func main() {
	var (
		addFlag     string
		showVersion bool
	)

	root := &cobra.Command{
		Use:           "hl-tickers",
		Short:         "Fast terminal market watcher powered by Hyperliquid",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Println("hl-tickers", version)
				return nil
			}
			return run(addFlag)
		},
	}

	root.Flags().StringVarP(&addFlag, "add", "a", "", "comma-separated symbols to add to the watchlist this run (e.g. BTC,GOLD,SPX)")
	root.Flags().BoolVar(&showVersion, "version", false, "print version and exit")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(addFlag string) error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not read config:", err)
	}

	if cfg.Wallet != "" && !hl.IsValidAddress(cfg.Wallet) {
		fmt.Fprintf(os.Stderr, "warning: ignoring invalid wallet address %q\n", cfg.Wallet)
		cfg.Wallet = ""
	}

	// Creates config file for user with sample config blocks commented out
	_ = cfg.EnsureFile()

	addSymbols := make([]string, 0)
	for _, raw := range strings.Split(addFlag, ",") {
		if sym := strings.TrimSpace(raw); sym != "" {
			addSymbols = append(addSymbols, sym)
		}
	}

	client := hl.NewClient()
	app := views.New(client, cfg, addSymbols)

	// Handle scrollwheel ourselves vs letting terminal handle it since there is multi-rows heights
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithFPS(120))
	_, err = p.Run()
	return err
}
