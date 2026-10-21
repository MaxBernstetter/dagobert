package bybit

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MaxBernstetter/trading-copilot/services/execution/internal/config"
)

var (
	ErrPermanentReject = errors.New("permanent order rejection")
	ErrRetryExhausted  = errors.New("retries exhausted")
	errEmptyHistory    = errors.New("empty order history")
)

type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	http       *http.Client
	retry      config.Retry
	recvWindow int
}

func NewClient(baseURL string, retry config.Retry) (*Client, error) {
	key := strings.TrimSpace(os.Getenv("BYBIT_API_KEY"))
	secret := strings.TrimSpace(os.Getenv("BYBIT_API_SECRET"))
	if key == "" || secret == "" {
		return nil, fmt.Errorf("BYBIT_API_KEY and BYBIT_API_SECRET are required")
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     key,
		apiSecret:  secret,
		recvWindow: config.RecvWindow,
		http:       &http.Client{Timeout: 60 * time.Second},
		retry:      retry,
	}, nil
}

type apiResponse struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

type CreateResult struct {
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
}

type OrderHistoryItem struct {
	OrderID      string `json:"orderId"`
	OrderLinkID  string `json:"orderLinkId"`
	Symbol       string `json:"symbol"`
	Side         string `json:"side"`
	OrderType    string `json:"orderType"`
	OrderStatus  string `json:"orderStatus"`
	Qty          string `json:"qty"`
	CumExecQty   string `json:"cumExecQty"`
	CumExecValue string `json:"cumExecValue"`
	CumExecFee   string `json:"cumExecFee"`
	AvgPrice     string `json:"avgPrice"`
	MarketUnit   string `json:"marketUnit"`
}

type TransactionLogRow struct {
	Currency        string  `json:"currency"`
	CashBalance     string  `json:"cashBalance"`
	TransactionTime FlexInt `json:"transactionTime"`
}

type FlexInt int64

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		*f = FlexInt(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

func (c *Client) CreateSpotOrder(ctx context.Context, req CreateOrderRequest) (CreateResult, error) {
	body := map[string]string{
		"category":    "spot",
		"symbol":      req.Symbol,
		"side":        req.Side,
		"orderType":   "Market",
		"qty":         req.Qty,
		"marketUnit":  req.MarketUnit,
		"orderLinkId": req.OrderLinkID,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return CreateResult{}, err
	}

	var resp apiResponse
	err = c.doWithRetry(ctx, http.MethodPost, "/v5/order/create", "", bodyJSON, &resp, true)
	if err != nil {
		return CreateResult{}, err
	}
	if resp.RetCode != 0 {
		return CreateResult{}, &OrderRejectError{Code: resp.RetCode, Msg: resp.RetMsg}
	}
	var result CreateResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return CreateResult{}, err
	}
	return result, nil
}

type CreateOrderRequest struct {
	Symbol      string
	Side        string
	Qty         string
	MarketUnit  string
	OrderLinkID string
}

type OrderRejectError struct {
	Code int
	Msg  string
}

func (e *OrderRejectError) Error() string {
	return fmt.Sprintf("bybit order rejected: %d %s", e.Code, e.Msg)
}

func (c *Client) OrderHistory(ctx context.Context, orderID string) (OrderHistoryItem, error) {
	return c.orderByQuery(ctx, "category=spot&orderId="+url.QueryEscape(orderID))
}

func (c *Client) OrderByLink(ctx context.Context, orderLinkID string) (OrderHistoryItem, error) {
	item, err := c.orderByQuery(ctx, "category=spot&orderLinkId="+url.QueryEscape(orderLinkID))
	if err == nil {
		return item, nil
	}
	return c.orderByQuery(ctx, "category=spot&openOnly=0&orderLinkId="+url.QueryEscape(orderLinkID))
}

func (c *Client) orderByQuery(ctx context.Context, query string) (OrderHistoryItem, error) {
	var resp apiResponse
	path := "/v5/order/history"
	if strings.Contains(query, "openOnly=") {
		path = "/v5/order/realtime"
	}
	if err := c.doWithRetry(ctx, http.MethodGet, path, query, nil, &resp, false); err != nil {
		return OrderHistoryItem{}, err
	}
	if resp.RetCode != 0 {
		return OrderHistoryItem{}, fmt.Errorf("bybit history retCode %d: %s", resp.RetCode, resp.RetMsg)
	}
	var wrapper struct {
		List []OrderHistoryItem `json:"list"`
	}
	if err := json.Unmarshal(resp.Result, &wrapper); err != nil {
		return OrderHistoryItem{}, err
	}
	if len(wrapper.List) == 0 {
		return OrderHistoryItem{}, errEmptyHistory
	}
	return wrapper.List[0], nil
}

func (c *Client) TransactionLog(ctx context.Context) ([]TransactionLogRow, error) {
	query := "accountType=UNIFIED&limit=50"
	var resp apiResponse
	if err := c.doWithRetry(ctx, http.MethodGet, "/v5/account/transaction-log", query, nil, &resp, false); err != nil {
		return nil, err
	}
	if resp.RetCode != 0 {
		return nil, fmt.Errorf("bybit transaction-log retCode %d: %s", resp.RetCode, resp.RetMsg)
	}
	var wrapper struct {
		List []TransactionLogRow `json:"list"`
	}
	if err := json.Unmarshal(resp.Result, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.List, nil
}

func (c *Client) doWithRetry(ctx context.Context, method, path, query string, body []byte, out *apiResponse, isCreate bool) error {
	var wait time.Duration
	for try := 0; ; try++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		signBody := query
		if method == http.MethodPost {
			signBody = string(body)
		}
		reqURL := c.baseURL + path
		if query != "" {
			reqURL += "?" + query
		}
		req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sig := sign(c.apiSecret, ts, c.apiKey, c.recvWindow, signBody)
		req.Header.Set("X-BAPI-API-KEY", c.apiKey)
		req.Header.Set("X-BAPI-SIGN", sig)
		req.Header.Set("X-BAPI-TIMESTAMP", ts)
		req.Header.Set("X-BAPI-RECV-WINDOW", strconv.Itoa(c.recvWindow))
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			if try >= c.retry.MaxRetries || !retryableHTTP(0, err) {
				return fmt.Errorf("%w: %s", ErrRetryExhausted, err.Error())
			}
			wait = c.backoff(try)
			continue
		}
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			if try >= c.retry.MaxRetries {
				return ErrRetryExhausted
			}
			wait = c.backoff(try)
			continue
		}

		var parsed apiResponse
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			if try >= c.retry.MaxRetries {
				return fmt.Errorf("%w: HTTP %d body %s", ErrRetryExhausted, resp.StatusCode, clip(respBody))
			}
			wait = c.backoff(try)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			if try >= c.retry.MaxRetries {
				return fmt.Errorf("%w: HTTP %d body %s", ErrRetryExhausted, resp.StatusCode, clip(respBody))
			}
			wait = c.backoff(try)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("bybit HTTP %d body %s", resp.StatusCode, clip(respBody))
		}

		if parsed.RetCode != 0 {
			if isCreate && !retryableRetCode(parsed.RetCode) {
				return &OrderRejectError{Code: parsed.RetCode, Msg: parsed.RetMsg}
			}
			if retryableRetCode(parsed.RetCode) {
				if try >= c.retry.MaxRetries {
					return ErrRetryExhausted
				}
				wait = c.backoff(try)
				continue
			}
			return fmt.Errorf("bybit retCode %d: %s", parsed.RetCode, parsed.RetMsg)
		}

		*out = parsed
		return nil
	}
}

func clip(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 180 {
		s = s[:180]
	}
	return s
}

func sign(secret, timestamp, apiKey string, recvWindow int, body string) string {
	payload := timestamp + apiKey + strconv.Itoa(recvWindow) + body
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func retryableHTTP(status int, err error) bool {
	if status == http.StatusTooManyRequests || status >= 500 {
		return true
	}
	return status == 0 && err != nil
}

func retryableRetCode(code int) bool {
	return code == 10006 || code == 10016
}

func (c *Client) backoff(failure int) time.Duration {
	pow := math.Pow(c.retry.BackoffFactor, float64(failure))
	if math.IsNaN(pow) || math.IsInf(pow, 0) || pow < 0 {
		return c.retry.MaxRetryWait
	}
	wait := time.Duration(float64(c.retry.RetryWait) * pow)
	if wait < 0 || wait > c.retry.MaxRetryWait {
		return c.retry.MaxRetryWait
	}
	return wait
}

type BalanceSnapshot struct {
	Currency        string
	CashBalance     string
	TransactionTime int64
}

func LatestBalances(rows []TransactionLogRow) []BalanceSnapshot {
	latest := make(map[string]TransactionLogRow)
	for _, row := range rows {
		cur, ok := latest[row.Currency]
		if !ok || int64(row.TransactionTime) > int64(cur.TransactionTime) {
			latest[row.Currency] = row
		}
	}
	out := make([]BalanceSnapshot, 0, len(latest))
	for _, row := range latest {
		out = append(out, BalanceSnapshot{
			Currency:        row.Currency,
			CashBalance:     row.CashBalance,
			TransactionTime: int64(row.TransactionTime),
		})
	}
	return out
}
