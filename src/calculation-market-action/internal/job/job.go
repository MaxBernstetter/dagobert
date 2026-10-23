package job

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MaxBernstetter/trading-copilot/src/internal/instruments"
	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/bybit"
	"github.com/MaxBernstetter/trading-copilot/src/internal/kpi"
	"github.com/MaxBernstetter/trading-copilot/src/internal/signalstore"
	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/config"
	"github.com/MaxBernstetter/trading-copilot/src/internal/venue/paper"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-market-action/internal/allocate"
)

// Run sizes a stored signal into an executable action using live venue balances.
func Run(args []string) (exitCode int, detail int) {
	symbol, candleClose, playbookID, ok := parseArgs(args)
	if !ok {
		return 1, 10
	}
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" || playbookID == "" {
		return 1, 10
	}

	db, err := signalstore.Open(dataDir)
	if err != nil {
		return 1, 30
	}
	defer db.Close()

	if ok, err := signalstore.HasAction(db, playbookID, symbol, candleClose); err != nil {
		return 1, 30
	} else if ok {
		return 2, 1
	}

	sig, err := signalstore.ReadSignal(db, playbookID, symbol, candleClose)
	if errors.Is(err, signalstore.ErrNotFound) {
		return 2, 20
	}
	if err != nil {
		return 1, 30
	}

	inst, ok := instruments.BySeries(symbol)
	if !ok {
		return 1, 10
	}

	var quoteCash, baseHeld float64
	if paper.Enabled() {
		quoteCash, baseHeld, err = paper.QuoteAndBase(dataDir, inst.Quote, inst.Base)
		if err != nil {
			return 1, 30
		}
	} else {
		var balances []bybit.BalanceSnapshot
		retryCfg, err := config.RetryFromEnv()
		if err != nil {
			return 1, 10
		}
		client, err := bybit.NewClient(config.BaseURL(), retryCfg)
		if err != nil {
			return 1, 30
		}
		balances, err = client.AccountBalances(context.Background(), instruments.WalletCoins())
		if err != nil {
			return 1, 30
		}
		quoteCash, baseHeld = venueBalances(balances, inst.Quote, inst.Base)
	}

	aggMS := signalstore.AggregationDurationMS()
	barOpen := candleClose - aggMS
	mark, err := markPrice(dataDir, symbol, barOpen)
	if err != nil {
		return 1, 30
	}

	pol := allocate.PolicyFromEnv()

	out := allocate.Size(allocate.Input{
		SignalDecision: sig.Decision,
		QuoteCash:      quoteCash,
		BaseHeld:       baseHeld,
		MarkPrice:      mark,
	}, pol)

	rule := sig.RuleID
	if out.Rule != "" && out.Rule != "none" {
		rule = out.Rule
	}
	if out.Note != "" && out.Decision == "no-trade" {
		rule = out.Note
	}

	actionBody, err := encodeAction(playbookID, symbol, out.Decision, out.Size, rule, sig.RuleID)
	if err != nil {
		return 1, 30
	}
	if err := signalstore.CommitAction(db, playbookID, symbol, candleClose, actionBody); err != nil {
		if errors.Is(err, signalstore.ErrDuplicateAction) {
			return 2, 1
		}
		return 1, 30
	}

	if out.Decision == "no-trade" {
		return 0, 2
	}
	return 0, 0
}

func venueBalances(rows []bybit.BalanceSnapshot, quote, base string) (float64, float64) {
	var q, b float64
	for _, row := range rows {
		if row.Currency == quote {
			q, _ = allocate.ParseBalance(row.CashBalance)
		}
		if row.Currency == base {
			b, _ = allocate.ParseBalance(row.CashBalance)
		}
	}
	return q, b
}

func markPrice(dataDir, symbol string, barOpen int64) (float64, error) {
	results, err := kpi.Read(dataDir, symbol, barOpen)
	if err != nil {
		return 0, err
	}
	v, ok := kpi.MovingAverage(results, "sma", 3)
	if !ok {
		return 0, fmt.Errorf("no mark sma")
	}
	return v, nil
}

func encodeAction(playbookID, symbol, decision string, size *float64, rule, signalRule string) ([]byte, error) {
	inst, _ := instruments.BySeries(symbol)
	unit, unitKind := "", ""
	switch decision {
	case "buy":
		unit, unitKind = inst.Quote, "quote"
	case "sell":
		unit, unitKind = inst.Base, "base"
	}
	hash := sha256.Sum256([]byte("ma-cross:3"))
	body := map[string]any{
		"decision":      decision,
		"size":          size,
		"unit":          unit,
		"unit_kind":     unitKind,
		"rule":          rule,
		"signal_rule":   signalRule,
		"written_at":    time.Now().UTC().Format(time.RFC3339),
		"venue":         inst.Venue,
		"venue_symbol":  inst.VenueSymbol,
		"price_source":  inst.PriceSource,
		"playbook_hash": hex.EncodeToString(hash[:]),
		"algo":          "v1",
	}
	return json.Marshal(body)
}

func parseArgs(args []string) (symbol string, candleClose int64, playbook string, ok bool) {
	if len(args) != 2 {
		return "", 0, "", false
	}
	symbol = strings.TrimSpace(args[0])
	candleClose, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
	if err != nil || candleClose <= 0 || symbol == "" {
		return "", 0, "", false
	}
	playbook = strings.TrimSpace(os.Getenv("PLAYBOOK"))
	if playbook == "" {
		playbook = "ma-cross.yaml"
	}
	if strings.Contains(playbook, "/") {
		playbook = playbook[strings.LastIndex(playbook, "/")+1:]
	}
	return symbol, candleClose, playbook, true
}
