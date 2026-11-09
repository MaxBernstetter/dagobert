// Package pgtest gives tests a scratch Postgres database with the current schema.
package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultAdminURL is the Aurora cluster that `task up` creates on Floci.
const DefaultAdminURL = "postgres://trading:trading@127.0.0.1:7001/postgres?sslmode=require"

// ScratchDB creates a fresh database on the Postgres named by POSTGRES_TEST_URL (default: Aurora on
// Floci) with db/sqlc/schema.sql applied, and returns its URL. The database is required: when
// Postgres is unreachable the test fails. It is dropped at cleanup.
func ScratchDB(t *testing.T) string {
	t.Helper()
	admin := os.Getenv("POSTGRES_TEST_URL")
	if admin == "" {
		admin = DefaultAdminURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ap, err := pgxpool.New(ctx, admin)
	if err == nil {
		err = ap.Ping(ctx)
	}
	if err != nil {
		t.Fatalf("Postgres required (POSTGRES_TEST_URL=%s): %v", admin, err)
	}
	t.Cleanup(ap.Close)
	name := "sys_test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if _, err := ap.Exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() { _, _ = ap.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	_, self, _, _ := runtime.Caller(0)
	schema, err := os.ReadFile(filepath.Join(filepath.Dir(self), "../../../db/sqlc/schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := pgxpool.New(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.Exec(context.Background(), string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return u.String()
}

// Opens returns the open_time column of table for one market, ordered.
func Opens(t *testing.T, dbURL, table, base, quote string) []int64 {
	t.Helper()
	p, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	rows, err := p.Query(context.Background(),
		fmt.Sprintf(`SELECT open_time FROM %s WHERE provider = 'binance' AND base = $1 AND quote = $2 ORDER BY open_time`, table), base, quote)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}
