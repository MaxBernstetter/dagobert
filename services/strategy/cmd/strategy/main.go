package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/action"
	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/kpi"
	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/playbook"
	"github.com/MaxBernstetter/trading-copilot/services/strategy/internal/retry"
)

// Invocation: strategy <symbol> <candle_close_ms>
// Environment: DATA_DIR, PLAYBOOK (path to YAML); optional SCHEMA_PATH overrides embedded JSON Schema.
const threeHoursMs int64 = 3 * 60 * 60 * 1000

var symbolPattern = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)

func main() {
	exitCode, detail := run()
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(exitCode)
}

func run() (exitCode int, detail int) {
	symbol, candleClose, ok := parseArgs(os.Args[1:])
	if !ok {
		return 1, 10
	}

	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	playbookPath := strings.TrimSpace(os.Getenv("PLAYBOOK"))
	if dataDir == "" || playbookPath == "" {
		return 1, 10
	}

	compiler, err := playbook.LoadSchemaCompiler()
	if err != nil {
		return 1, 40
	}

	doc, err := playbook.LoadAndValidate(playbookPath, compiler)
	if err != nil {
		if errors.Is(err, playbook.ErrSchemaValidation) || errors.Is(err, playbook.ErrSchemaLoad) {
			return 1, 40
		}
		return 1, 30
	}

	barOpen := playbook.BarOpen(candleClose)
	var results []playbook.KPIResult
	err = retry.Do(func() error {
		var readErr error
		results, readErr = kpi.Read(dataDir, symbol, barOpen)
		return readErr
	})
	if errors.Is(err, kpi.ErrNotFound) {
		return 2, 20
	}
	if err != nil {
		return 1, 30
	}

	dec, err := playbook.Evaluate(doc, results)
	if err != nil {
		return 1, 30
	}

	err = retry.Do(func() error {
		return action.Write(dataDir, playbookPath, symbol, candleClose, dec)
	})
	if errors.Is(err, action.ErrDuplicate) {
		return 2, 1
	}
	if err != nil {
		return 1, 30
	}

	return 0, 0
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
	if ms-threeHoursMs < 0 {
		return "", 0, false
	}
	return symbol, ms, true
}
