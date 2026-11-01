package pgcatalog

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
)

// ProviderRecord is the stored state of one ingestion provider: activation, setup status and progress,
// and its non-secret config. Config nil means never saved; it is stored as SQL NULL.
type ProviderRecord struct {
	ID            string
	Activated     bool
	Status        string
	Error         string
	ProgressLabel *string
	ProgressDone  *int
	ProgressTotal *int
	Config        map[string]string
	UpdatedAtMs   int64
}

// ProviderPair is one market of a provider snapshot.
type ProviderPair struct {
	Base  string
	Quote string
}

// GetProvider returns the stored state of a provider; ok is false when it was never saved.
func (c *Catalog) GetProvider(ctx context.Context, id string) (ProviderRecord, bool, error) {
	row, err := c.q.GetProviderState(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProviderRecord{}, false, nil
	}
	if err != nil {
		return ProviderRecord{}, false, err
	}
	rec := ProviderRecord{
		ID:          row.ID,
		Activated:   row.Activated,
		Status:      row.Status,
		Error:       row.Error,
		UpdatedAtMs: row.UpdatedAtMs,
	}
	if row.ProgressLabel.Valid {
		label := row.ProgressLabel.String
		rec.ProgressLabel = &label
	}
	if row.ProgressDone.Valid {
		done := int(row.ProgressDone.Int32)
		rec.ProgressDone = &done
	}
	if row.ProgressTotal.Valid {
		total := int(row.ProgressTotal.Int32)
		rec.ProgressTotal = &total
	}
	if row.Config != nil {
		if err := json.Unmarshal(row.Config, &rec.Config); err != nil {
			return ProviderRecord{}, false, err
		}
		if rec.Config == nil {
			rec.Config = map[string]string{}
		}
	}
	return rec, true, nil
}

func providerParams(rec ProviderRecord) (experimentdb.UpsertProviderStateParams, error) {
	p := experimentdb.UpsertProviderStateParams{
		ID:          rec.ID,
		Activated:   rec.Activated,
		Status:      rec.Status,
		Error:       rec.Error,
		UpdatedAtMs: rec.UpdatedAtMs,
	}
	if rec.ProgressLabel != nil {
		p.ProgressLabel = pgtype.Text{String: *rec.ProgressLabel, Valid: true}
	}
	if rec.ProgressDone != nil {
		p.ProgressDone = pgtype.Int4{Int32: int32(*rec.ProgressDone), Valid: true}
	}
	if rec.ProgressTotal != nil {
		p.ProgressTotal = pgtype.Int4{Int32: int32(*rec.ProgressTotal), Valid: true}
	}
	if rec.Config != nil {
		body, err := json.Marshal(rec.Config)
		if err != nil {
			return p, err
		}
		p.Config = body
	}
	return p, nil
}

// SaveProvider writes the provider state.
func (c *Catalog) SaveProvider(ctx context.Context, rec ProviderRecord) error {
	p, err := providerParams(rec)
	if err != nil {
		return err
	}
	return c.q.UpsertProviderState(ctx, p)
}

// ProviderPairs returns the stored snapshot of a provider in its upstream order.
func (c *Catalog) ProviderPairs(ctx context.Context, id string) ([]ProviderPair, error) {
	rows, err := c.q.ListProviderPairs(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderPair, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProviderPair{Base: r.Base, Quote: r.Quote})
	}
	return out, nil
}

// SaveProviderSetup writes a successful setup in one transaction: the snapshot replaces the previous one
// and the state is saved with it, so a reader never sees ready without its pairs.
func (c *Catalog) SaveProviderSetup(ctx context.Context, rec ProviderRecord, pairs []ProviderPair) error {
	p, err := providerParams(rec)
	if err != nil {
		return err
	}
	tx, err := c.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := c.q.WithTx(tx)
	if err := q.UpsertProviderState(ctx, p); err != nil {
		return err
	}
	if err := q.DeleteProviderPairs(ctx, rec.ID); err != nil {
		return err
	}
	for i, pair := range pairs {
		if err := q.InsertProviderPair(ctx, experimentdb.InsertProviderPairParams{
			ProviderID: rec.ID, Ordinal: int32(i), Base: pair.Base, Quote: pair.Quote,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
