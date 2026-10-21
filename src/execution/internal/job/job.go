package job

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/trading-copilot/src/execution/internal/bybit"
	"github.com/MaxBernstetter/trading-copilot/src/execution/internal/config"
	"github.com/MaxBernstetter/trading-copilot/src/execution/internal/store"
	"github.com/MaxBernstetter/trading-copilot/src/execution/internal/strategyaction"
)

const threeHoursMs int64 = 3 * 60 * 60 * 1000

var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)

type KeyFields struct {
	OrderLinkID  string `json:"orderLinkId"`
	Playbook     string `json:"playbook"`
	Symbol       string `json:"symbol"`
	VenueSymbol  string `json:"venueSymbol"`
	CandleClose  int64  `json:"candleClose"`
	Decision     string `json:"decision,omitempty"`
	OrderID      string `json:"orderId,omitempty"`
	OrderStatus  string `json:"orderStatus,omitempty"`
	Side         string `json:"side,omitempty"`
	Qty          string `json:"qty,omitempty"`
	CumExecQty   string `json:"cumExecQty,omitempty"`
	CumExecValue string `json:"cumExecValue,omitempty"`
	AvgPrice     string `json:"avgPrice,omitempty"`
	CumExecFee   string `json:"cumExecFee,omitempty"`
	FeeCurrency  string `json:"feeCurrency,omitempty"`
	MarketUnit   string `json:"marketUnit,omitempty"`
	RetCode      int    `json:"retCode,omitempty"`
	RetMsg       string `json:"retMsg,omitempty"`
}

func Run(args []string) (exitCode int, detail int) {
	symbol, candleClose, ok := parseArgs(args)
	if !ok {
		return 1, 10
	}

	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	playbookPath := strings.TrimSpace(os.Getenv("PLAYBOOK"))
	if dataDir == "" || playbookPath == "" {
		return 1, 10
	}

	playbookID := filepath.Base(playbookPath)
	orderLinkID := orderLinkID(playbookID, symbol, candleClose)
	venueSymbol, ok := venueSymbol(symbol)
	if !ok {
		return 1, 10
	}

	action, err := strategyaction.Read(dataDir, playbookPath, symbol, candleClose)
	if errors.Is(err, strategyaction.ErrNotFound) {
		return 2, 20
	}
	if err != nil {
		return 1, 30
	}

	st, err := store.Open(dataDir)
	if err != nil {
		return 1, 30
	}

	existing, err := st.GetOrderResult(orderLinkID)
	if err == nil {
		switch existing.Kind {
		case store.KindFill, store.KindRejection, store.KindNoTrade:
			return 2, 1
		case store.KindPending:
			return pollPending(context.Background(), st, existing, config.BaseURL())
		default:
			return 2, 1
		}
	}
	if !errors.Is(err, store.ErrNotFound) {
		return 1, 30
	}

	retryCfg, err := config.RetryFromEnv()
	if err != nil {
		return 1, 10
	}

	ctx := context.Background()

	switch action.Decision {
	case "no-trade":
		client, err := bybit.NewClient(config.BaseURL(), retryCfg)
		if err != nil {
			return 1, 30
		}
		if err := refreshBalance(ctx, client, st); err != nil {
			if errors.Is(err, bybit.ErrRetryExhausted) {
				return 1, 30
			}
			return 1, 30
		}
		body := KeyFields{
			OrderLinkID: orderLinkID,
			Playbook:    playbookID,
			Symbol:      symbol,
			VenueSymbol: venueSymbol,
			CandleClose: candleClose,
			Decision:    "no-trade",
		}
		if err := st.InsertOrderResult(orderLinkID, store.KindNoTrade, body); err != nil {
			return 1, 30
		}
		return 0, 2

	case "buy", "sell":
		return placeOrder(ctx, st, retryCfg, action, orderLinkID, playbookID, symbol, venueSymbol, candleClose)

	default:
		return 1, 30
	}
}

func pollPending(ctx context.Context, st *store.DB, existing store.OrderResult, baseURL string) (int, int) {
	var body KeyFields
	if err := json.Unmarshal(existing.Body, &body); err != nil || body.OrderID == "" {
		return 1, 30
	}
	retryCfg, err := config.RetryFromEnv()
	if err != nil {
		return 1, 10
	}
	client, err := bybit.NewClient(baseURL, retryCfg)
	if err != nil {
		return 1, 30
	}
	item, err := client.OrderHistory(ctx, body.OrderID)
	if errors.Is(err, bybit.ErrRetryExhausted) {
		return 1, 30
	}
	if err != nil {
		return 1, 30
	}
	if item.OrderStatus == "Filled" {
		fillBody := mergeFill(body, item)
		if err := st.UpdateOrderResult(body.OrderLinkID, store.KindFill, fillBody); err != nil {
			return 1, 30
		}
		_ = st.UpsertJournal(journalFrom(fillBody))
		if err := refreshBalance(ctx, client, st); err != nil {
			if errors.Is(err, bybit.ErrRetryExhausted) {
				return 1, 30
			}
			return 1, 30
		}
		return 0, 0
	}
	return 2, 31
}

func settle(ctx context.Context, client *bybit.Client, st *store.DB, key KeyFields, item bybit.OrderHistoryItem) (int, int) {
	if item.OrderStatus == "Filled" {
		fillBody := mergeFill(key, item)
		if err := st.InsertOrderResult(key.OrderLinkID, store.KindFill, fillBody); err != nil {
			return 1, 30
		}
		_ = st.UpsertJournal(journalFrom(fillBody))
		if err := refreshBalance(ctx, client, st); err != nil {
			fmt.Fprintf(os.Stderr, "execution: balance: %s\n", err.Error())
			return 1, 30
		}
		return 0, 0
	}
	pendingBody := key
	pendingBody.OrderID = item.OrderID
	pendingBody.OrderStatus = item.OrderStatus
	if err := st.InsertOrderResult(key.OrderLinkID, store.KindPending, pendingBody); err != nil {
		return 1, 30
	}
	return 2, 31
}

func placeOrder(
	ctx context.Context,
	st *store.DB,
	retryCfg config.Retry,
	action strategyaction.Action,
	orderLinkID, playbookID, symbol, venueSymbol string,
	candleClose int64,
) (int, int) {
	if action.Size == nil {
		return 1, 30
	}
	client, err := bybit.NewClient(config.BaseURL(), retryCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execution: %s\n", err.Error())
		return 1, 30
	}

	key := KeyFields{
		OrderLinkID: orderLinkID,
		Playbook:    playbookID,
		Symbol:      symbol,
		VenueSymbol: venueSymbol,
		CandleClose: candleClose,
	}

	req := bybit.CreateOrderRequest{
		Symbol:      venueSymbol,
		Qty:         formatQty(*action.Size),
		OrderLinkID: orderLinkID,
	}
	switch action.Decision {
	case "buy":
		req.Side = "Buy"
		req.MarketUnit = "quoteCoin"
	case "sell":
		req.Side = "Sell"
		req.MarketUnit = "baseCoin"
	}

	created, err := client.CreateSpotOrder(ctx, req)
	var reject *bybit.OrderRejectError
	if errors.As(err, &reject) {
		if reject.Code == 170141 {
			item, lookupErr := client.OrderByLink(ctx, orderLinkID)
			if lookupErr == nil {
				return settle(ctx, client, st, key, item)
			}
		}
		rejBody := key
		rejBody.RetCode = reject.Code
		rejBody.RetMsg = reject.Msg
		if err := st.InsertOrderResult(orderLinkID, store.KindRejection, rejBody); err != nil {
			return 1, 30
		}
		fmt.Fprintf(os.Stderr, "execution: %s\n", reject.Error())
		if err := refreshBalance(ctx, client, st); err != nil && !errors.Is(err, bybit.ErrRetryExhausted) {
			fmt.Fprintf(os.Stderr, "execution: balance: %s\n", err.Error())
		}
		return 1, 40
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "execution: %s\n", err.Error())
		return 1, 30
	}

	item, err := client.OrderHistory(ctx, created.OrderID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execution: %s\n", err.Error())
		pending := key
		pending.OrderID = created.OrderID
		pending.Side = req.Side
		pending.Qty = req.Qty
		pending.MarketUnit = req.MarketUnit
		pending.OrderStatus = "Created"
		if insertErr := st.InsertOrderResult(orderLinkID, store.KindPending, pending); insertErr != nil {
			return 1, 30
		}
		return 2, 31
	}
	return settle(ctx, client, st, key, item)
}

func mergeFill(key KeyFields, item bybit.OrderHistoryItem) KeyFields {
	key.OrderID = item.OrderID
	key.OrderStatus = item.OrderStatus
	key.Side = item.Side
	key.Qty = item.Qty
	key.CumExecQty = item.CumExecQty
	key.CumExecValue = item.CumExecValue
	key.AvgPrice = item.AvgPrice
	key.MarketUnit = item.MarketUnit
	key.CumExecFee = item.CumExecFee
	if strings.HasSuffix(key.VenueSymbol, "USDC") {
		key.FeeCurrency = "USDC"
	}
	return key
}

func journalFrom(key KeyFields) store.Journal {
	return store.Journal{
		OrderLinkID:     key.OrderLinkID,
		Venue:           "bybit",
		VenueSymbol:     key.VenueSymbol,
		Side:            key.Side,
		BaseQty:         key.CumExecQty,
		QuoteQty:        key.CumExecValue,
		Price:           key.AvgPrice,
		Fee:             key.CumExecFee,
		FeeCurrency:     key.FeeCurrency,
		ExchangeOrderID: key.OrderID,
		PriceSource:     "binance",
	}
}

func refreshBalance(ctx context.Context, client *bybit.Client, st *store.DB) error {
	rows, err := client.TransactionLog(ctx)
	if err != nil {
		return err
	}
	latest := bybit.LatestBalances(rows)
	balRows := make([]store.BalanceRow, 0, len(latest))
	for _, r := range latest {
		balRows = append(balRows, store.BalanceRow{
			Currency:        r.Currency,
			CashBalance:     r.CashBalance,
			TransactionTime: int64(r.TransactionTime),
		})
	}
	return st.ReplaceBalances(balRows)
}

func parseArgs(args []string) (symbol string, candleClose int64, ok bool) {
	if len(args) != 2 {
		return "", 0, false
	}
	symbol = strings.TrimSpace(args[0])
	if !symbolPattern.MatchString(symbol) {
		return "", 0, false
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
	if err != nil || ms <= 0 {
		return "", 0, false
	}
	if ms-threeHoursMs < 0 {
		return "", 0, false
	}
	return symbol, ms, true
}

func venueSymbol(symbol string) (string, bool) {
	switch symbol {
	case "BTCUSDT":
		return "BTCUSDC", true
	case "ETHUSDT":
		return "ETHUSDC", true
	default:
		return "", false
	}
}

func orderLinkID(playbook, symbol string, candleClose int64) string {
	return playbook + ":" + symbol + ":" + strconv.FormatInt(candleClose, 10)
}

func formatQty(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 {
		return ""
	}
	s := strconv.FormatFloat(f, 'f', 8, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" {
		return "0"
	}
	return s
}
