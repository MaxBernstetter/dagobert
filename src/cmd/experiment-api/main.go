package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MaxBernstetter/dagobert/src/experimentapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

const listenAddr = ":8091"

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL required")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "experiment-api: db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	experimentapi.Mount(mux, experimentapi.NewServer(pool, experimentapi.Hooks{}))

	srv := &http.Server{Addr: listenAddr, Handler: mux}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
	}()
	fmt.Printf("experiment-api: listening on %s\n", listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "experiment-api: %v\n", err)
		os.Exit(1)
	}
}
