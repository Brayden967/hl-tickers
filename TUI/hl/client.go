package hl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	MainnetInfoURL = "https://api.hyperliquid.xyz/info"
	MainnetWsURL   = "wss://api.hyperliquid.xyz/ws"

	// caps a single /info response body
	maxInfoResponseBytes = 64 << 20
)

type Client struct {
	infoURL string
	http    *http.Client
}

func NewClient() *Client {
	return &Client{
		infoURL: MainnetInfoURL,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) info(ctx context.Context, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.infoURL, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("post /info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("hyperliquid /info status %d: %s", resp.StatusCode, string(b))
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxInfoResponseBytes)).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (c *Client) MetaAndAssetCtxs(ctx context.Context, dex string) (Meta, []AssetCtx, error) {
	body := map[string]string{"type": "metaAndAssetCtxs"}
	if dex != "" {
		body["dex"] = dex
	}

	var raw []json.RawMessage
	if err := c.info(ctx, body, &raw); err != nil {
		return Meta{}, nil, err
	}
	if len(raw) != 2 {
		return Meta{}, nil, fmt.Errorf("metaAndAssetCtxs: expected 2 elements, got %d", len(raw))
	}

	var meta Meta
	if err := json.Unmarshal(raw[0], &meta); err != nil {
		return Meta{}, nil, fmt.Errorf("decode meta: %w", err)
	}

	var ctxs []AssetCtx
	if err := json.Unmarshal(raw[1], &ctxs); err != nil {
		return Meta{}, nil, fmt.Errorf("decode asset ctxs: %w", err)
	}

	return meta, ctxs, nil
}

// Fetches HIP-3 dexes
func (c *Client) PerpDexs(ctx context.Context) ([]*PerpDex, error) {
	var dexs []*PerpDex
	if err := c.info(ctx, map[string]string{"type": "perpDexs"}, &dexs); err != nil {
		return nil, err
	}
	return dexs, nil
}

func (c *Client) ClearinghouseState(ctx context.Context, user string) (ClearinghouseState, error) {
	var state ClearinghouseState
	err := c.info(ctx, map[string]string{"type": "clearinghouseState", "user": user}, &state)
	return state, err
}

// Fetches OHLCV history for a coin between start and end timestamp
func (c *Client) CandleSnapshot(ctx context.Context, coin, interval string, startMs, endMs int64) ([]Candle, error) {
	body := map[string]any{
		"type": "candleSnapshot",
		"req": map[string]any{
			"coin":      coin,
			"interval":  interval,
			"startTime": startMs,
			"endTime":   endMs,
		},
	}
	var candles []Candle
	if err := c.info(ctx, body, &candles); err != nil {
		return nil, err
	}
	return candles, nil
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}
