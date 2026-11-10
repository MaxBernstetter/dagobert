package job

import (
	"context"

	"github.com/MaxBernstetter/dagobert/src/internal/ingestledger"
)

// store is the ingest ledger (Postgres, see ingestledger): stored hours and confirmed-empty hours.
type store struct {
	l *ingestledger.Ledger
}

func openStore() (*store, error) {
	l, err := ingestledger.Open(context.Background())
	if err != nil {
		return nil, err
	}
	return &store{l: l}, nil
}

func (s *store) close() { s.l.Close() }

func (s *store) list(provider, base, quote string, from, to int64) ([]int64, error) {
	return s.l.Stored(context.Background(), provider, base, quote, from, to)
}

func (s *store) insert(provider, base, quote string, times []int64) error {
	return s.l.MarkStored(context.Background(), provider, base, quote, times)
}

func (s *store) listUnavailable(provider, base, quote string, from, to int64) ([]int64, error) {
	return s.l.Unavailable(context.Background(), provider, base, quote, from, to)
}

func (s *store) insertUnavailable(provider, base, quote string, times []int64, reason string, confirmedAt int64) error {
	return s.l.MarkUnavailable(context.Background(), provider, base, quote, times, reason, confirmedAt)
}

func (s *store) clearUnavailable(provider, base, quote string, from, to int64) error {
	return s.l.ClearUnavailable(context.Background(), provider, base, quote, from, to)
}
