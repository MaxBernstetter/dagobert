// Package ingestledger is the ingest ledger in Postgres: which candle hours are stored in the lake
// and which hours the exchange has no candle for. It replaces the old ingest.sqlite.
package ingestledger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/internal/pgpool"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoDatabase means DATABASE_URL is unset, so there is no ledger to read.
var ErrNoDatabase = errors.New("ingestledger: DATABASE_URL is not set")

// Ledger is a connection pool to the control-plane database.
type Ledger struct {
	pool *pgxpool.Pool
}

// Open connects with DATABASE_URL.
func Open(ctx context.Context) (*Ledger, error) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		return nil, ErrNoDatabase
	}
	pool, err := pgpool.Open(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("ingestledger: %w", err)
	}
	return &Ledger{pool: pool}, nil
}

func (l *Ledger) Close() { l.pool.Close() }

func (l *Ledger) opens(ctx context.Context, table, provider, base, quote string, from, to int64) ([]int64, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT open_time FROM `+table+` WHERE provider = $1 AND base = $2 AND quote = $3 AND open_time >= $4 AND open_time < $5 ORDER BY open_time`,
		provider, base, quote, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// Stored lists the stored hour opens in [from, to).
func (l *Ledger) Stored(ctx context.Context, provider, base, quote string, from, to int64) ([]int64, error) {
	return l.opens(ctx, "ingest_stored", provider, base, quote, from, to)
}

// Unavailable lists the confirmed-empty hour opens in [from, to).
func (l *Ledger) Unavailable(ctx context.Context, provider, base, quote string, from, to int64) ([]int64, error) {
	return l.opens(ctx, "ingest_unavailable", provider, base, quote, from, to)
}

// LatestStored is the newest stored hour open for the market, if any.
func (l *Ledger) LatestStored(ctx context.Context, provider, base, quote string) (int64, bool, error) {
	var latest *int64
	err := l.pool.QueryRow(ctx,
		`SELECT MAX(open_time) FROM ingest_stored WHERE provider = $1 AND base = $2 AND quote = $3`,
		provider, base, quote).Scan(&latest)
	if err != nil || latest == nil {
		return 0, false, err
	}
	return *latest, true, nil
}

// MarkStored records hours as stored (idempotent).
func (l *Ledger) MarkStored(ctx context.Context, provider, base, quote string, times []int64) error {
	if len(times) == 0 {
		return nil
	}
	_, err := l.pool.Exec(ctx,
		`INSERT INTO ingest_stored(provider, base, quote, open_time)
		 SELECT $1, $2, $3, unnest($4::bigint[]) ON CONFLICT DO NOTHING`,
		provider, base, quote, times)
	return err
}

// MarkUnavailable records hours as confirmed empty, replacing earlier reasons.
func (l *Ledger) MarkUnavailable(ctx context.Context, provider, base, quote string, times []int64, reason string, confirmedAt int64) error {
	if len(times) == 0 {
		return nil
	}
	if reason == "" {
		reason = "empty"
	}
	_, err := l.pool.Exec(ctx,
		`INSERT INTO ingest_unavailable(provider, base, quote, open_time, reason, confirmed_at)
		 SELECT $1, $2, $3, unnest($4::bigint[]), $5, $6
		 ON CONFLICT (provider, base, quote, open_time) DO UPDATE SET reason = EXCLUDED.reason, confirmed_at = EXCLUDED.confirmed_at`,
		provider, base, quote, times, reason, confirmedAt)
	return err
}

// ClearUnavailable forgets confirmed-empty hours in [from, to), so they are fetched again.
func (l *Ledger) ClearUnavailable(ctx context.Context, provider, base, quote string, from, to int64) error {
	_, err := l.pool.Exec(ctx,
		`DELETE FROM ingest_unavailable WHERE provider = $1 AND base = $2 AND quote = $3 AND open_time >= $4 AND open_time < $5`,
		provider, base, quote, from, to)
	return err
}
