package bybit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/config"
)

func TestWalletBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/account/wallet-balance" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[{"accountType":"UNIFIED","coin":[
			{"coin":"USDC","walletBalance":"123.45","equity":"123.45"},
			{"coin":"BTC","walletBalance":"0.01","equity":"0.01"}
		]}]}}`))
	}))
	defer srv.Close()

	os.Setenv("BYBIT_API_KEY", "k")
	os.Setenv("BYBIT_API_SECRET", "s")
	defer os.Unsetenv("BYBIT_API_KEY")
	defer os.Unsetenv("BYBIT_API_SECRET")

	client, err := NewClient(srv.URL, config.Retry{MaxRetries: 0, RetryWait: 0, MaxRetryWait: 0, BackoffFactor: 1})
	if err != nil {
		t.Fatal(err)
	}
	snaps, err := client.WalletBalance(context.Background(), []string{"USDC", "BTC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("got %d snapshots", len(snaps))
	}
	if snaps[0].Currency != "USDC" || snaps[0].CashBalance != "123.45" {
		t.Fatalf("usdc: %+v", snaps[0])
	}
}

func TestAccountBalancesFallsBackOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v5/account/wallet-balance":
			http.NotFound(w, r)
		case "/v5/account/transaction-log":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"retCode":0,"retMsg":"OK","result":{"list":[
				{"currency":"USDC","cashBalance":"99","transactionTime":1000}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	os.Setenv("BYBIT_API_KEY", "k")
	os.Setenv("BYBIT_API_SECRET", "s")
	defer os.Unsetenv("BYBIT_API_KEY")
	defer os.Unsetenv("BYBIT_API_SECRET")

	client, err := NewClient(srv.URL, config.Retry{MaxRetries: 0, RetryWait: 0, MaxRetryWait: 0, BackoffFactor: 1})
	if err != nil {
		t.Fatal(err)
	}
	snaps, err := client.AccountBalances(context.Background(), []string{"USDC"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].CashBalance != "99" {
		t.Fatalf("got %+v", snaps)
	}
}
