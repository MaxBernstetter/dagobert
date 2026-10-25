package binance

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

const (
	klineWeight = 2
	pageLimit   = 1000 // API max candles per request (= max 1h hours per page)
)

// Options configures the Binance public klines adapter (connection only).
type Options struct {
	BaseURL string
}

// Adapter fetches Binance spot 1h klines (one HTTP page per FetchPage).
type Adapter struct {
	endpoint string
	host     string
	http     *http.Client
}

// New builds a Binance adapter from Options.
func New(opts Options) (*Adapter, error) {
	base := strings.TrimSpace(opts.BaseURL)
	if base == "" {
		return nil, fmt.Errorf("BINANCE_BASE_URL")
	}
	endpoint, err := klineEndpoint(base)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	a := &Adapter{
		endpoint: endpoint,
		host:     u.Host,
	}
	a.http = &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Host != a.host {
				return errors.New("refusing redirect")
			}
			return nil
		},
	}
	return a, nil
}

func (a *Adapter) ID() string { return shared.DataSourceBinance.String() }

func (a *Adapter) DataSource() shared.DataSource { return shared.DataSourceBinance }

func (a *Adapter) EmptyReason() string { return "binance_empty" }

func (a *Adapter) MaxPageHours() int { return pageLimit }

func (a *Adapter) RequestWeight() int { return klineWeight }

func (a *Adapter) Retryable(status int, err error) bool {
	if status == http.StatusTooManyRequests || status == http.StatusTeapot || status >= 500 {
		return true
	}
	if status == 0 && err != nil {
		return true
	}
	if status == http.StatusOK && err != nil {
		return true
	}
	return false
}

func klineEndpoint(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("BINANCE_BASE_URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("BINANCE_BASE_URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v3/klines"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

var _ provider.Adapter = (*Adapter)(nil)
