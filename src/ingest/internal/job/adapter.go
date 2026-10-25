package job

import (
	"fmt"

	"github.com/MaxBernstetter/dagobert/src/ingest/provider"
	"github.com/MaxBernstetter/dagobert/src/ingest/provider/binance"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

func openAdapter(cfg Config) (provider.Adapter, error) {
	switch cfg.Provider {
	case shared.DataSourceBinance.String():
		return binance.New(binance.Options{BaseURL: cfg.BaseURL})
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
