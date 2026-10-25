package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
)

type exchangeInfoResp struct {
	Symbols []exchangeSymbol `json:"symbols"`
}

type exchangeSymbol struct {
	Symbol     string   `json:"symbol"`
	Status     string   `json:"status"`
	BaseAsset  string   `json:"baseAsset"`
	QuoteAsset string   `json:"quoteAsset"`
	Permissions []string `json:"permissions"`
}

// ListPairs returns all Binance spot TRADING markets (base ≠ quote), sorted by base then quote.
func (a *Adapter) ListPairs(ctx context.Context) ([]provider.Pair, error) {
	infoURL, err := exchangeInfoURL(a.endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Full spot exchangeInfo is ~18MB today; keep headroom above that.
	const maxExchangeInfoBytes = 32 << 20
	limited := io.LimitReader(resp.Body, maxExchangeInfoBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxExchangeInfoBytes {
		return nil, fmt.Errorf("binance exchangeInfo exceeds %d bytes", maxExchangeInfoBytes)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance exchangeInfo status %d", resp.StatusCode)
	}
	return parseExchangeInfoPairs(body)
}

func exchangeInfoURL(klineEndpoint string) (string, error) {
	u, err := url.Parse(klineEndpoint)
	if err != nil {
		return "", err
	}
	u.Path = "/api/v3/exchangeInfo"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func parseExchangeInfoPairs(body []byte) ([]provider.Pair, error) {
	var info exchangeInfoResp
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	out := make([]provider.Pair, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		if !strings.EqualFold(s.Status, "TRADING") {
			continue
		}
		if !isSpotSymbol(s) {
			continue
		}
		base := strings.ToUpper(strings.TrimSpace(s.BaseAsset))
		quote := strings.ToUpper(strings.TrimSpace(s.QuoteAsset))
		if base == "" || quote == "" || base == quote {
			continue
		}
		native := strings.ToUpper(strings.TrimSpace(s.Symbol))
		if native == "" {
			native = NativeSymbol(base, quote)
		}
		out = append(out, provider.Pair{
			Base:         base,
			Quote:        quote,
			NativeSymbol: native,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Base != out[j].Base {
			return out[i].Base < out[j].Base
		}
		return out[i].Quote < out[j].Quote
	})
	return out, nil
}

func isSpotSymbol(s exchangeSymbol) bool {
	if len(s.Permissions) == 0 {
		return true
	}
	for _, p := range s.Permissions {
		if strings.EqualFold(p, "SPOT") {
			return true
		}
	}
	return false
}
