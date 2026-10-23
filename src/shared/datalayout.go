package shared

import (
	"os"
	"path/filepath"
)

// DataParquetSubdir is where ingest writes symbol candle files under DATA_DIR.
const DataParquetSubdir = "parquet"

// EnsureDataLayout creates directories under dataDir that job containers write to.
// Call on the host (or with the same UID that runs jobs) before root-owned Docker
// processes create them on a bind mount.
func EnsureDataLayout(dataDir string) error {
	if dataDir == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(dataDir, DataParquetSubdir), 0o755)
}
