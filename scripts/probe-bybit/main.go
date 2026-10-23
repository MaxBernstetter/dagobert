// One-off probe for Bybit EU demo balance endpoints. Run from repo root:
//   set -a && source .secrets && set +a && go run ./scripts/probe-bybit
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	base := strings.TrimRight(env("BYBIT_BASE_URL", "https://api-demo.bybit.eu"), "/")
	key := strings.TrimSpace(os.Getenv("BYBIT_API_KEY"))
	secret := strings.TrimSpace(os.Getenv("BYBIT_API_SECRET"))
	if key == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "BYBIT_API_KEY and BYBIT_API_SECRET required")
		os.Exit(1)
	}
	fmt.Printf("host: %s\n\n", base)

	queries := []string{
		"GET /v5/account/info | (no query)",
		"GET /v5/account/wallet-balance | accountType=UNIFIED",
		"GET /v5/account/wallet-balance | accountType=SPOT",
		"GET /v5/account/transaction-log | accountType=UNIFIED&limit=50",
		"GET /v5/asset/transfer/query-account-coins-balance | accountType=UNIFIED&coin=USDC,BTC,ETH",
		"GET /v5/asset/transfer/query-account-coins-balance | accountType=SPOT&coin=USDC,BTC,ETH",
		"GET /v5/asset/transfer/query-account-coins-balance | accountType=FUND&coin=USDC,BTC,ETH",
		"GET /v5/asset/withdraw/withdrawable-amount | coin=USDC",
	}

	paths := []struct {
		path  string
		query string
	}{
		{"/v5/account/info", ""},
		{"/v5/account/wallet-balance", "accountType=UNIFIED"},
		{"/v5/account/wallet-balance", "accountType=SPOT"},
		{"/v5/account/transaction-log", "accountType=UNIFIED&limit=50"},
		{"/v5/asset/transfer/query-account-coins-balance", "accountType=UNIFIED&coin=USDC,BTC,ETH"},
		{"/v5/asset/transfer/query-account-coins-balance", "accountType=SPOT&coin=USDC,BTC,ETH"},
		{"/v5/asset/transfer/query-account-coins-balance", "accountType=FUND&coin=USDC,BTC,ETH"},
		{"/v5/asset/withdraw/withdrawable-amount", "coin=USDC"},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	for i, p := range paths {
		fmt.Println(queries[i])
		status, body, err := signedGet(client, base, key, secret, p.path, p.query)
		if err != nil {
			fmt.Printf("  error: %v\n\n", err)
			continue
		}
		summarize(status, body)
		fmt.Println()
	}
}

func signedGet(client *http.Client, base, key, secret, path, query string) (int, []byte, error) {
	url := base + path
	if query != "" {
		url += "?" + query
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recv := "5000"
	signBody := query
	payload := ts + key + recv + signBody
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-BAPI-API-KEY", key)
	req.Header.Set("X-BAPI-SIGN", sig)
	req.Header.Set("X-BAPI-TIMESTAMP", ts)
	req.Header.Set("X-BAPI-RECV-WINDOW", recv)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func summarize(status int, body []byte) {
	fmt.Printf("  HTTP %d\n", status)
	trim := strings.TrimSpace(string(body))
	if trim == "" {
		fmt.Println("  (empty body)")
		return
	}
	var outer struct {
		RetCode int             `json:"retCode"`
		RetMsg  string          `json:"retMsg"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		fmt.Printf("  body (not JSON): %s\n", clip(trim, 200))
		return
	}
	fmt.Printf("  retCode=%d retMsg=%q\n", outer.RetCode, outer.RetMsg)
	if outer.RetCode != 0 || len(outer.Result) == 0 {
		fmt.Printf("  result: %s\n", clip(string(outer.Result), 300))
		return
	}

	var tx struct {
		List []struct {
			Currency        string `json:"currency"`
			CashBalance     string `json:"cashBalance"`
			TransactionTime string `json:"transactionTime"`
		} `json:"list"`
	}
	if json.Unmarshal(outer.Result, &tx) == nil && len(tx.List) > 0 && tx.List[0].Currency != "" {
		fmt.Printf("  transaction-log rows: %d\n", len(tx.List))
		latest := map[string]string{}
		for _, row := range tx.List {
			latest[row.Currency] = row.CashBalance
		}
		for cur, bal := range latest {
			fmt.Printf("    %s latest cashBalance=%s (from last row in page)\n", cur, bal)
		}
		return
	}

	var wallet struct {
		List []struct {
			AccountType string `json:"accountType"`
			Coin        []struct {
				Coin                string `json:"coin"`
				WalletBalance       string `json:"walletBalance"`
				AvailableToWithdraw string `json:"availableToWithdraw"`
				Equity              string `json:"equity"`
			} `json:"coin"`
		} `json:"list"`
	}
	if json.Unmarshal(outer.Result, &wallet) == nil && len(wallet.List) > 0 && wallet.List[0].Coin != nil {
		for _, acct := range wallet.List {
			fmt.Printf("  accountType=%s coins=%d\n", acct.AccountType, len(acct.Coin))
			for _, c := range acct.Coin {
				if c.Coin == "" {
					continue
				}
				fmt.Printf("    %s walletBalance=%s availableToWithdraw=%s equity=%s\n",
					c.Coin, c.WalletBalance, c.AvailableToWithdraw, c.Equity)
			}
		}
		return
	}

	var coins struct {
		Balance []struct {
			Coin            string `json:"coin"`
			WalletBalance   string `json:"walletBalance"`
			TransferBalance string `json:"transferBalance"`
		} `json:"balance"`
	}
	if json.Unmarshal(outer.Result, &coins) == nil && len(coins.Balance) > 0 {
		fmt.Printf("  asset balances: %d\n", len(coins.Balance))
		for _, c := range coins.Balance {
			fmt.Printf("    %s walletBalance=%s transferBalance=%s\n",
				c.Coin, c.WalletBalance, c.TransferBalance)
		}
		return
	}

	var info map[string]json.RawMessage
	if json.Unmarshal(outer.Result, &info) == nil {
		fmt.Printf("  result keys: %v\n", keys(info))
		return
	}
	fmt.Printf("  result: %s\n", clip(string(outer.Result), 400))
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func env(k, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fallback
}
