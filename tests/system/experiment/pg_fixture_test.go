package experiment_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxBernstetter/dagobert/src/backtest/store"
	macross "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/ma-cross"
	signalconsensus "github.com/MaxBernstetter/dagobert/src/calculation-signal/playbooks/signal-consensus"
	signalrunner "github.com/MaxBernstetter/dagobert/src/calculation-signal/runner"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
)

// aggregationMS is the 3-hour candle the playbook configs are validated against (the default of
// signalstore.AggregationDurationMS, which tests cannot import).
const aggregationMS = 3 * 3600_000

// pgScratchDB creates a fresh database on the Postgres named by POSTGRES_TEST_URL (default: the
// compose Postgres) with the current schema applied. The database is required: when Postgres is
// unreachable the test fails. It is dropped at cleanup.
func pgScratchDB(t *testing.T) string {
	t.Helper()
	admin := os.Getenv("POSTGRES_TEST_URL")
	if admin == "" {
		admin = "postgres://trading:trading@127.0.0.1:9432/postgres"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ap, err := pgxpool.New(ctx, admin)
	if err == nil {
		err = ap.Ping(ctx)
	}
	if err != nil {
		t.Fatalf("Postgres required for the experiment tests (POSTGRES_TEST_URL=%s): %v", admin, err)
	}
	t.Cleanup(ap.Close)
	name := "exp_test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := ap.Exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() { _, _ = ap.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })
	cfg := ap.Config().ConnConfig.Copy()
	cfg.Database = name
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, name)
	schema, err := os.ReadFile("../../../db/sqlc/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	p, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Exec(context.Background(), string(schema)); err != nil {
		t.Fatal(err)
	}
	return url
}

// fixtureSpec is one instance as a test describes it: label, playbook, market and the raw strategy
// config (sizing and zero cooldowns included, which the create API cannot express).
type fixtureSpec struct {
	Label    string
	Playbook string
	Config   json.RawMessage
}

// pgInst is one instance of an experiment in the scratch database, with the fields the tests read.
type pgInst struct {
	ID               string
	Label            string
	Playbook         string
	Symbol           string
	PlaybookKey      string
	ConfigHash       string
	WorkspaceRelPath string
	ShardKey         string
	Status           string
	Error            string
	StartedAt        *int64
	FinishedAt       *int64
}

// pgRun is an experiment in the scratch database after creation or a run.
type pgRun struct {
	ID                    string
	Status                string
	Error                 string
	ProgressInstanceDone  int
	ProgressInstanceTotal int
	Instances             []pgInst
}

func toPGInst(i experimentdb.ExperimentInstance) pgInst {
	out := pgInst{
		ID: i.ID.String(), Label: i.Label, Playbook: i.Playbook, Symbol: i.Symbol, PlaybookKey: i.PlaybookKey, ConfigHash: i.ConfigHash,
		WorkspaceRelPath: i.WorkspaceRelPath, ShardKey: i.ShardKey, Status: i.Status, Error: i.Error,
	}
	if i.StartedAtMs.Valid {
		v := i.StartedAtMs.Int64
		out.StartedAt = &v
	}
	if i.FinishedAtMs.Valid {
		v := i.FinishedAtMs.Int64
		out.FinishedAt = &v
	}
	return out
}

// createExperimentPG inserts a draft experiment and its instances. The config hash, playbook key
// and canonical config come from the same playbook calls the pipeline uses, so the instances run
// exactly as an experiment created through the API. An invalid config is an error.
func createExperimentPG(url, name string, settings store.ExperimentSettings, specs []fixtureSpec) (pgRun, error) {
	if strings.TrimSpace(name) == "" || len(specs) == 0 {
		return pgRun{}, fmt.Errorf("name and at least one instance required")
	}
	settings.Base = strings.ToUpper(strings.TrimSpace(settings.Base))
	settings.Quote = strings.ToUpper(strings.TrimSpace(settings.Quote))
	if strings.TrimSpace(settings.Provider) == "" {
		settings.Provider = "binance"
	}
	if settings.Algo == "" {
		settings.Algo = "v1"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return pgRun{}, err
	}
	defer pool.Close()
	q := experimentdb.New(pool)

	expID := uuid.New()
	s := settings
	if _, err := q.InsertExperiment(ctx, experimentdb.InsertExperimentParams{
		ID: expID, Name: name, Status: store.StatusDraft, Provider: s.Provider, Base: s.Base, Quote: s.Quote,
		FromMs: s.FromMs, ToMs: s.ToMs, InitialUsdc: s.InitialUSDC, InitialUsdt: s.InitialUSDT, InitialBase: s.InitialBase,
		TargetWeightPct: s.TargetWeightPct, CashReserveQuote: s.CashReserveQuote, MinOrderQuote: s.MinOrderQuote,
		RedownloadMissing: s.RedownloadMissing, Algo: s.Algo, SpreadBps: s.SpreadBps, SlippageBps: s.SlippageBps,
		FeeBps: s.FeeBps, CreatedAtMs: time.Now().UnixMilli(), ProgressInstanceTotal: int32(len(specs)),
	}); err != nil {
		return pgRun{}, err
	}

	seen := map[string]bool{}
	out := pgRun{ID: expID.String(), Status: store.StatusDraft, ProgressInstanceTotal: len(specs)}
	for i, spec := range specs {
		playbook := strings.TrimSpace(spec.Playbook)
		if playbook == "" {
			playbook = "ma-cross"
		}
		ref, err := signalrunner.ResolvePlaybookRef(playbook)
		if err != nil {
			return pgRun{}, err
		}
		configJSON, hash, defaultLabel, err := canonicalConfig(ref.Slug, spec.Config)
		if err != nil {
			return pgRun{}, fmt.Errorf("instance %d: %w", i+1, err)
		}
		sym := settings.PipelineSymbol()
		if seen[hash] {
			return pgRun{}, fmt.Errorf("instance %d: duplicate strategy config (hash %s)", i+1, hash[:8])
		}
		seen[hash] = true
		instID := uuid.New()
		label := strings.TrimSpace(spec.Label)
		if label == "" {
			label = sym + " " + defaultLabel
		}
		inst, err := q.InsertInstance(ctx, experimentdb.InsertInstanceParams{
			ID: instID, ExperimentID: expID, Label: label, Playbook: ref.Slug, Symbol: sym,
			ConfigHash: hash, PlaybookKey: signalrunner.InstancePlaybookKey(ref.ID, hash),
			WorkspaceRelPath: fmt.Sprintf("experiments/%s/instances/%s", expID, instID), Status: store.StatusDraft,
			StrategyConfigJson: pgtype.Text{String: string(configJSON), Valid: true},
		})
		if err != nil {
			return pgRun{}, err
		}
		out.Instances = append(out.Instances, toPGInst(inst))
	}
	return out, nil
}

// canonicalConfig validates a raw strategy config for the playbook and returns its canonical JSON,
// hash and default label, the same as the experiment engine.
func canonicalConfig(slug string, raw json.RawMessage) ([]byte, string, string, error) {
	switch slug {
	case "ma-cross":
		cfg, err := macross.ParseConfig(raw)
		if err != nil {
			return nil, "", "", err
		}
		if err := cfg.Validate(aggregationMS); err != nil {
			return nil, "", "", err
		}
		body, err := cfg.CanonicalJSON()
		if err != nil {
			return nil, "", "", err
		}
		hash, err := cfg.Hash()
		return body, hash, fmt.Sprintf("%dh / %dh", cfg.FastHr, cfg.SlowHr), err
	case "signal-consensus":
		cfg, err := signalconsensus.ParseConfig(raw)
		if err != nil {
			return nil, "", "", err
		}
		if err := cfg.Validate(aggregationMS); err != nil {
			return nil, "", "", err
		}
		body, err := cfg.CanonicalJSON()
		if err != nil {
			return nil, "", "", err
		}
		hash, err := cfg.Hash()
		return body, hash, fmt.Sprintf("rsi %.0f/%.0f atr×%.1f", cfg.RsiBull, cfg.RsiBear, cfg.AtrStopMult), err
	}
	return nil, "", "", fmt.Errorf("experiment instances do not support playbook %q", slug)
}

// loadExperimentPG reads the experiment and its instances back from the scratch database.
func loadExperimentPG(t *testing.T, url, id string) pgRun {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	q := experimentdb.New(pool)
	eid := uuid.MustParse(id)
	exp, err := q.GetExperiment(context.Background(), eid)
	if err != nil {
		t.Fatal(err)
	}
	insts, err := q.ListInstancesByExperiment(context.Background(), eid)
	if err != nil {
		t.Fatal(err)
	}
	out := pgRun{
		ID: id, Status: exp.Status, Error: exp.Error,
		ProgressInstanceDone: int(exp.ProgressInstanceDone), ProgressInstanceTotal: int(exp.ProgressInstanceTotal),
	}
	for _, i := range insts {
		out.Instances = append(out.Instances, toPGInst(i))
	}
	return out
}
