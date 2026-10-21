package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/MaxBernstetter/trading-copilot/src/process/internal/job"
)

func main() {
	now := time.Now().UnixMilli()
	nowInvalid := false
	if raw, ok := os.LookupEnv("NOW_MS"); ok {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			nowInvalid = true
		} else {
			now = parsed
		}
	}

	detail, code := job.Run(os.Args[1:], os.Getenv("DATA_DIR"), now, nowInvalid)
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(composeExit(code))
}

func composeExit(code int) int {
	if code == 2 && os.Getenv("COMPOSE_OK_ON_ABORT") == "1" {
		return 0
	}
	return code
}
