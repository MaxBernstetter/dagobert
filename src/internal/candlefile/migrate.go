package candlefile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/internal/parquetmeta"
)

// MigrateFile rewrites a legacy (string) candle file in the current schema.
// The new file is written next to the original, read back and compared value
// by value as decimal strings, and only then renamed over the original. A file
// that already uses the current schema is left untouched (migrated is false).
func MigrateFile(path string) (migrated bool, rows int, err error) {
	src, err := os.Open(path)
	if err != nil {
		return false, 0, err
	}
	info, err := src.Stat()
	if err != nil {
		src.Close()
		return false, 0, err
	}
	pf, err := parquet.OpenFile(src, info.Size())
	if err != nil {
		src.Close()
		return false, 0, fmt.Errorf("open: %w", err)
	}
	if !IsLegacy(pf) {
		src.Close()
		return false, 0, nil
	}
	old, err := Read(pf)
	meta, _ := parquetmeta.FromFile(pf)
	src.Close()
	if err != nil {
		return false, 0, err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return false, 0, err
	}
	tmpName := tmp.Name()
	defer func() {
		if !migrated {
			os.Remove(tmpName)
		}
	}()
	if err := Write(tmp, old, meta); err != nil {
		tmp.Close()
		return false, 0, err
	}
	if err := tmp.Close(); err != nil {
		return false, 0, err
	}
	if err := verify(tmpName, old); err != nil {
		return false, 0, fmt.Errorf("verify: %w", err)
	}
	if err := os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		return false, 0, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return false, 0, err
	}
	return true, len(old), nil
}

// verify reads the new file back and compares every value with want as a decimal.
func verify(path string, want []Row) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return err
	}
	got, err := Read(pf)
	if err != nil {
		return err
	}
	if len(got) != len(want) {
		return fmt.Errorf("row count %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.OpenTime != w.OpenTime || g.CloseTime != w.CloseTime ||
			NormalizeDecimal(g.Open) != NormalizeDecimal(w.Open) ||
			NormalizeDecimal(g.High) != NormalizeDecimal(w.High) ||
			NormalizeDecimal(g.Low) != NormalizeDecimal(w.Low) ||
			NormalizeDecimal(g.Close) != NormalizeDecimal(w.Close) ||
			NormalizeDecimal(g.Volume) != NormalizeDecimal(w.Volume) {
			return fmt.Errorf("row %d (open_time %d) differs after rewrite", i, w.OpenTime)
		}
	}
	return nil
}
