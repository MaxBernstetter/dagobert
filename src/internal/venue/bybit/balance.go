package bybit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// HTTPStatusError is returned when Bybit responds with a non-retryable HTTP status.
type HTTPStatusError struct {
	Status int
	Body   string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("bybit HTTP %d body %s", e.Status, e.Body)
}

// WalletBalance queries GET /v5/account/wallet-balance (UNIFIED).
// See https://bybit-exchange.github.io/docs/v5/account/wallet-balance
func (c *Client) WalletBalance(ctx context.Context, coins []string) ([]BalanceSnapshot, error) {
	query := "accountType=UNIFIED"
	if len(coins) > 0 {
		query += "&coin=" + strings.Join(coins, ",")
	}
	var resp apiResponse
	if err := c.doWithRetry(ctx, http.MethodGet, "/v5/account/wallet-balance", query, nil, &resp, false); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit wallet-balance retCode %d: %s", resp.RetCode, resp.RetMsg)
	}
	var wrapper struct {
		List []struct {
			Coin []struct {
				Coin          string `json:"coin"`
				WalletBalance string `json:"walletBalance"`
				Equity        string `json:"equity"`
			} `json:"coin"`
		} `json:"list"`
	}
	if err := json.Unmarshal(resp.Result, &wrapper); err != nil {
		return nil, err
	}
	now := time.Now().UnixMilli()
	out := make([]BalanceSnapshot, 0)
	for _, acct := range wrapper.List {
		for _, row := range acct.Coin {
			if row.Coin == "" {
				continue
			}
			bal := strings.TrimSpace(row.WalletBalance)
			if bal == "" {
				bal = strings.TrimSpace(row.Equity)
			}
			out = append(out, BalanceSnapshot{
				Currency:        row.Coin,
				CashBalance:     bal,
				TransactionTime: now,
			})
		}
	}
	return out, nil
}

// AccountBalances returns current coin balances for portfolio sizing and UI snapshots.
// It uses wallet-balance per Bybit docs; on HTTP 404 (api-demo.bybit.eu) it falls back to transaction-log.
func (c *Client) AccountBalances(ctx context.Context, coins []string) ([]BalanceSnapshot, error) {
	snaps, err := c.WalletBalance(ctx, coins)
	if err == nil {
		return snaps, nil
	}
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) || httpErr.Status != 404 {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "bybit: wallet-balance not routed (HTTP 404); falling back to transaction-log\n")
	rows, err := c.TransactionLog(ctx)
	if err != nil {
		return nil, err
	}
	return LatestBalances(rows), nil
}
