package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const backfillAddr = ":8090"

var backfillSymbols = []string{"BTCUSDT", "ETHUSDT", "USDCUSDT"}

type backfillRequest struct {
	Kind    string   `json:"kind"`
	From    int64    `json:"from"`
	To      int64    `json:"to"`
	Symbols []string `json:"symbols"`
}

type backfillStatus struct {
	Kind     string `json:"kind"`
	From     int64  `json:"from"`
	To       int64  `json:"to"`
	Symbol   string `json:"symbol"`
	Running  bool   `json:"running"`
	Error    string `json:"error"`
	Finished string `json:"finished"`
}

func serveBackfill(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /backfill", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, readBackfill())
	})
	mux.HandleFunc("POST /backfill", func(w http.ResponseWriter, r *http.Request) {
		var req backfillRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Kind != "ingest" && req.Kind != "indicators" {
			http.Error(w, "kind must be ingest or indicators", http.StatusBadRequest)
			return
		}
		if req.From <= 0 || req.To <= req.From {
			http.Error(w, "from and to must be millisecond timestamps with from < to", http.StatusBadRequest)
			return
		}
		now := time.Now().UnixMilli()
		if req.To > now {
			req.To = now
		}
		if req.Kind == "ingest" {
			req.To -= req.To % hourMS
		}
		if req.From >= req.To {
			http.Error(w, "range ends inside the current open hour", http.StatusBadRequest)
			return
		}
		symbols, err := backfillTargets(req.Symbols)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		req.Symbols = symbols
		jobMu.Lock()
		cur := readBackfill()
		if cur.Running {
			jobMu.Unlock()
			http.Error(w, "a backfill is already running", http.StatusConflict)
			return
		}
		status := backfillStatus{Kind: req.Kind, From: req.From, To: req.To, Running: true}
		writeBackfill(status)
		jobMu.Unlock()
		go runBackfill(context.Background(), req)
		writeJSON(w, http.StatusAccepted, status)
	})
	srv := &http.Server{Addr: backfillAddr, Handler: mux}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shut)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "orchestrator: backfill: %s\n", err)
	}
}

func clearStaleBackfill() {
	cur := readBackfill()
	if !cur.Running {
		return
	}
	symbol := cur.Symbol
	cur.Running = false
	cur.Symbol = ""
	if symbol == "" {
		cur.Error = "orchestrator restarted during backfill"
	} else {
		cur.Error = "orchestrator restarted during " + symbol
	}
	cur.Finished = time.Now().UTC().Format(time.RFC3339)
	writeBackfill(cur)
	writeLive(taskInput{Stage: cur.Kind}, false, 1)
}

func backfillTargets(symbols []string) ([]string, error) {
	if len(symbols) == 0 {
		return []string{"BTCUSDT", "ETHUSDT"}, nil
	}
	allowed := map[string]struct{}{"BTCUSDT": {}, "ETHUSDT": {}, "USDCUSDT": {}}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		if _, ok := allowed[symbol]; !ok {
			return nil, fmt.Errorf("unknown position %s", symbol)
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out, nil
}

func runBackfill(ctx context.Context, req backfillRequest) {
	jobMu.Lock()
	defer jobMu.Unlock()
	status := backfillStatus{Kind: req.Kind, From: req.From, To: req.To, Running: true}
	var failed string
	for _, symbol := range req.Symbols {
		status.Symbol = symbol
		writeBackfill(status)
		stage := req.Kind
		if stage == "indicators" {
			stage = "indicators"
		}
		code, err := runJobLocked(ctx, taskInput{
			Stage:       stage,
			Symbol:      symbol,
			From:        req.From,
			CandleClose: req.To,
		})
		if err != nil || (code != 0 && code != 2) {
			if err != nil {
				failed = err.Error()
			} else {
				failed = fmt.Sprintf("%s %s exit %d", req.Kind, symbol, code)
			}
			break
		}
	}
	status.Symbol = ""
	status.Running = false
	status.Error = failed
	status.Finished = time.Now().UTC().Format(time.RFC3339)
	writeBackfill(status)
	writeLive(taskInput{Stage: req.Kind}, false, 0)
}

func backfillPath() string {
	return filepath.Join(env("LIVE_DIR", "/data"), "backfill.json")
}

func readBackfill() backfillStatus {
	raw, err := os.ReadFile(backfillPath())
	if err != nil {
		return backfillStatus{}
	}
	var status backfillStatus
	if json.Unmarshal(raw, &status) != nil {
		return backfillStatus{}
	}
	return status
}

func writeBackfill(status backfillStatus) {
	body, err := json.Marshal(status)
	if err != nil {
		return
	}
	_ = os.WriteFile(backfillPath(), body, 0o644)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
