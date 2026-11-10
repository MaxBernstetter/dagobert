package pgcatalog

import (
	"context"
	"errors"
	"testing"

	"github.com/MaxBernstetter/dagobert/src/shared/pgtest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A cancelled experiment must not be resurrected by a phase that was already in flight, but a re-run
// (the API sets status running before the phases start) must still write.
func TestPhaseWritesDoNotResurrectCancelledExperiment(t *testing.T) {
	dbURL := pgtest.ScratchDB(t)
	t.Setenv("DATABASE_URL", dbURL)
	ctx := context.Background()

	c, err := Open(ctx)
	if err != nil || c == nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer c.Close()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO experiment (id, name, status, created_at_ms, progress_stage_label)
		VALUES ($1, 'x', 'cancelled', 1, 'Cancelled')`, id); err != nil {
		t.Fatal(err)
	}
	status := func() (string, string) {
		var s, label string
		if err := pool.QueryRow(ctx, `SELECT status, progress_stage_label FROM experiment WHERE id = $1`, id).Scan(&s, &label); err != nil {
			t.Fatal(err)
		}
		return s, label
	}
	sid := id.String()

	if err := c.MarkExperimentRunning(ctx, sid); !errors.Is(err, ErrExperimentCancelled) {
		t.Fatalf("MarkExperimentRunning on cancelled: got %v, want ErrExperimentCancelled", err)
	}
	if err := c.MarkExperimentRunningKeepCompleted(ctx, sid, 1); !errors.Is(err, ErrExperimentCancelled) {
		t.Fatalf("MarkExperimentRunningKeepCompleted on cancelled: got %v", err)
	}
	if err := c.SetExperimentProgress(ctx, sid, 2, "Indicators", 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkExperimentFailed(ctx, sid, "boom"); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkExperimentCompleted(ctx, sid); err != nil {
		t.Fatal(err)
	}
	if s, label := status(); s != "cancelled" || label != "Cancelled" {
		t.Fatalf("after phase writes: status=%q label=%q, want cancelled/Cancelled", s, label)
	}

	// Re-run: the API marks the row running first, then the phase starts normally.
	if _, err := pool.Exec(ctx, `UPDATE experiment SET status = 'running' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkExperimentRunning(ctx, sid); err != nil {
		t.Fatalf("MarkExperimentRunning on running: %v", err)
	}
	if s, _ := status(); s != "running" {
		t.Fatalf("re-run status=%q, want running", s)
	}
}
