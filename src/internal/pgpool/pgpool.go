// Package pgpool opens the Postgres pools for Aurora Serverless v2 with auto-pause (database.tf: min 0 ACU,
// seconds_until_auto_pause 300). Two rules keep a scale-to-zero database quiet and reachable:
//   - idle connections are closed after MaxConnIdleTime, so the cluster can pause once the workload stops;
//   - the first connection waits for the cluster to resume (about 15 to 30 s from pause).
package pgpool

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// maxConnIdle is well below the 300 s auto-pause window, so idle pools never hold the cluster awake.
	maxConnIdle = 60 * time.Second
	// resumeWait covers the 15 to 30 s resume from zero ACU, inside the 29 s API gateway timeout's first call.
	resumeWait = 25 * time.Second
	retryEvery = 2 * time.Second
)

// Open parses url, builds the pool and returns it once the database answers a ping. It fails after resumeWait.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConnIdleTime = maxConnIdle
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(resumeWait)
	for {
		err = pool.Ping(ctx)
		if err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			pool.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(retryEvery):
		}
	}
}
