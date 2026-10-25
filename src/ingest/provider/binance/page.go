package binance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

// FetchPage performs one GET /api/v3/klines (no retries).
func (a *Adapter) FetchPage(ctx context.Context, req provider.PageRequest) provider.PageResult {
	m, err := shared.NormalizeMarket(req.Base, req.Quote)
	if err != nil {
		return provider.PageResult{RetryAfter: -1, Err: err}
	}
	native := NativeSymbol(m.Base, m.Quote)
	u, err := url.Parse(a.endpoint)
	if err != nil {
		return provider.PageResult{RetryAfter: -1, Err: err}
	}
	q := u.Query()
	q.Set("symbol", native)
	q.Set("interval", "1h")
	q.Set("startTime", strconv.FormatInt(req.Start, 10))
	q.Set("endTime", strconv.FormatInt(req.End, 10))
	q.Set("limit", strconv.Itoa(pageLimit))
	u.RawQuery = q.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return provider.PageResult{RetryAfter: -1, Err: err}
	}
	resp, err := a.http.Do(httpReq)
	if err != nil {
		return provider.PageResult{RetryAfter: -1, Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return provider.PageResult{StatusCode: resp.StatusCode, RetryAfter: -1, Err: err}
	}
	retryAfter, hasRA := parseRetryAfter(resp.Header.Get("Retry-After"))
	if resp.StatusCode != http.StatusOK {
		ra := time.Duration(-1)
		if hasRA {
			ra = retryAfter
		}
		return provider.PageResult{
			StatusCode: resp.StatusCode,
			RetryAfter: ra,
			Err:        fmt.Errorf("binance status %d", resp.StatusCode),
		}
	}
	rows, err := parseKlines(body)
	if err != nil {
		return provider.PageResult{StatusCode: resp.StatusCode, RetryAfter: -1, Err: err}
	}
	return provider.PageResult{Candles: rows, StatusCode: resp.StatusCode}
}

// NativeSymbol is Binance's concatenated wire id (no separator).
func NativeSymbol(base, quote string) string {
	return strings.ToUpper(strings.TrimSpace(base)) + strings.ToUpper(strings.TrimSpace(quote))
}

func parseRetryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if sec, err := strconv.Atoi(v); err == nil {
		if sec < 0 {
			return 0, true
		}
		return time.Duration(sec) * time.Second, true
	}
	when, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	d := time.Until(when)
	if d < 0 {
		return 0, true
	}
	return d, true
}

func parseKlines(body []byte) ([]provider.Candle, error) {
	var raw [][]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := make([]provider.Candle, 0, len(raw))
	for _, row := range raw {
		if len(row) < 7 {
			return nil, fmt.Errorf("short kline")
		}
		ot, err := rawInt(row[0])
		if err != nil {
			return nil, err
		}
		open, err := rawDec(row[1])
		if err != nil {
			return nil, err
		}
		high, err := rawDec(row[2])
		if err != nil {
			return nil, err
		}
		low, err := rawDec(row[3])
		if err != nil {
			return nil, err
		}
		cls, err := rawDec(row[4])
		if err != nil {
			return nil, err
		}
		vol, err := rawDec(row[5])
		if err != nil {
			return nil, err
		}
		ct, err := rawInt(row[6])
		if err != nil {
			return nil, err
		}
		out = append(out, provider.Candle{
			OpenTime:  ot,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     cls,
			Volume:    vol,
			CloseTime: ct,
		})
	}
	return out, nil
}

func rawInt(m json.RawMessage) (int64, error) {
	dec := json.NewDecoder(bytes.NewReader(m))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return 0, err
	}
	switch n := v.(type) {
	case json.Number:
		return n.Int64()
	case string:
		return strconv.ParseInt(n, 10, 64)
	default:
		return 0, fmt.Errorf("bad int")
	}
}

func rawDec(m json.RawMessage) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(m))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", err
	}
	switch n := v.(type) {
	case string:
		if n == "" {
			return "", fmt.Errorf("empty decimal")
		}
		return n, nil
	case json.Number:
		return n.String(), nil
	default:
		return "", fmt.Errorf("bad decimal")
	}
}
