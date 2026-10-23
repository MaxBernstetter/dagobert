package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/framework"
	"github.com/MaxBernstetter/trading-copilot/src/calculation-signal/internal/playbooks"
)

// Invocation: calculation-signal <symbol> <candle_close_ms>
// Environment: DATA_DIR, PLAYBOOK (slug or legacy path, e.g. ma-cross or ma-cross.yaml).
func main() {
	exitCode, detail := run()
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(composeExit(exitCode))
}

func composeExit(code int) int {
	if code == 2 && os.Getenv("COMPOSE_OK_ON_ABORT") == "1" {
		return 0
	}
	return code
}

var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)

func run() (exitCode int, detail int) {
	symbol, candleClose, ok := parseArgs(os.Args[1:])
	if !ok {
		return 1, 10
	}

	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	playbookEnv := strings.TrimSpace(os.Getenv("PLAYBOOK"))
	if dataDir == "" || playbookEnv == "" {
		return 1, 10
	}

	detail, err := playbooks.Run(dataDir, playbookEnv, symbol, candleClose)
	if err != nil {
		if errors.Is(err, framework.ErrDuplicateAction) {
			return 2, detail
		}
		if detail == 20 {
			return 2, detail
		}
		if detail == 40 {
			return 1, detail
		}
		return 1, detail
	}
	return 0, detail
}

func parseArgs(args []string) (symbol string, candleClose int64, ok bool) {
	if len(args) != 2 {
		return "", 0, false
	}
	symbol = strings.TrimSpace(args[0])
	if !symbolPattern.MatchString(symbol) {
		return "", 0, false
	}
	ms, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
	if err != nil || ms <= 0 {
		return "", 0, false
	}
	agg := framework.AggregationDurationMS()
	if ms-agg < 0 {
		return "", 0, false
	}
	return symbol, ms, true
}
