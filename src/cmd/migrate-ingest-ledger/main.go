// migrate-ingest-ledger copies the old DATA_DIR/ingest.sqlite into the Postgres ingest ledger
// (DATABASE_URL). Idempotent: rows already present are kept.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/MaxBernstetter/dagobert/src/internal/ingestledger"
)

type market struct{ provider, base, quote string }

func main() {
	dataDir := "./data"
	if len(os.Args) > 1 {
		dataDir = os.Args[1]
	}
	abs, err := filepath.Abs(filepath.Join(dataDir, "ingest.sqlite"))
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stat(abs); err != nil {
		log.Fatalf("nothing to migrate: %v", err)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	l, err := ingestledger.Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	stored := map[market][]int64{}
	rows, err := db.Query(`SELECT provider, base, quote, open_time FROM stored`)
	if err != nil {
		log.Fatal(err)
	}
	n := 0
	for rows.Next() {
		var m market
		var t int64
		if err := rows.Scan(&m.provider, &m.base, &m.quote, &t); err != nil {
			log.Fatal(err)
		}
		stored[m] = append(stored[m], t)
		n++
	}
	rows.Close()
	for m, times := range stored {
		for len(times) > 0 {
			k := min(len(times), 50_000)
			if err := l.MarkStored(ctx, m.provider, m.base, m.quote, times[:k]); err != nil {
				log.Fatal(err)
			}
			times = times[k:]
		}
	}

	m2 := 0
	urows, err := db.Query(`SELECT provider, base, quote, open_time, reason, confirmed_at FROM unavailable`)
	if err != nil {
		log.Fatal(err)
	}
	for urows.Next() {
		var m market
		var t, at int64
		var reason string
		if err := urows.Scan(&m.provider, &m.base, &m.quote, &t, &reason, &at); err != nil {
			log.Fatal(err)
		}
		if err := l.MarkUnavailable(ctx, m.provider, m.base, m.quote, []int64{t}, reason, at); err != nil {
			log.Fatal(err)
		}
		m2++
	}
	urows.Close()
	fmt.Printf("migrated %d stored and %d unavailable hours from %s\n", n, m2, abs)
}
