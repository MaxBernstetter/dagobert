// migrate-candles rewrites legacy string candle files under DATA_DIR/parquet in
// the fixed-scale decimal schema. It is idempotent: files already migrated are
// skipped. Each file is verified by a decimal round trip before it replaces the
// original (temp file + rename).
package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/internal/candlefile"
	"github.com/MaxBernstetter/dagobert/src/internal/lakeobject"
	"github.com/MaxBernstetter/dagobert/src/shared"
)

func main() {
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if len(os.Args) > 1 {
		dataDir = os.Args[1]
	}
	if dataDir == "" {
		fmt.Fprintln(os.Stderr, "usage: migrate-candles [DATA_DIR] (or set DATA_DIR)")
		os.Exit(2)
	}
	root := filepath.Join(dataDir, shared.DataParquetSubdir)
	var migrated, skipped, failed, rows int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != shared.CandlesParquetFile {
			return nil
		}
		did, n, err := candlefile.MigrateFile(path)
		switch {
		case err != nil:
			failed++
			fmt.Printf("FAILED   %s: %v\n", path, err)
		case did:
			migrated++
			rows += n
			_ = lakeobject.PutLocalFile(context.Background(), dataDir, path)
			fmt.Printf("migrated %s (%d rows)\n", path, n)
		default:
			skipped++
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk %s: %v\n", root, err)
		os.Exit(1)
	}
	fmt.Printf("migrated=%d skipped=%d failed=%d rows=%d\n", migrated, skipped, failed, rows)
	if failed > 0 {
		os.Exit(1)
	}
}
