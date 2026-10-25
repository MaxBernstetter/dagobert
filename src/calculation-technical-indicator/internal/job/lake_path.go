package job

import (
	"context"
	"os"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

func rawParquetPaths(dataDir, symbol string) []string {
	provider := strings.TrimSpace(os.Getenv("INGEST_PROVIDER"))
	if provider == "" {
		provider = shared.DefaultDataSource().String()
	}
	base, quote, ok := shared.SplitPipelineSymbol(symbol)
	if !ok {
		return nil
	}
	paths, err := lakeobject.ListCandleParquetAbsPaths(context.Background(), dataDir, provider, base, quote)
	if err != nil || len(paths) == 0 {
		return nil
	}
	return paths
}
