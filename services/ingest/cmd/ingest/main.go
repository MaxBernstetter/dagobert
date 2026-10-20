package main

import (
	"fmt"
	"os"

	"github.com/MaxBernstetter/trading-copilot/services/ingest/internal/job"
)

func main() {
	code, detail := job.Run(os.Args[1:], os.Getenv)
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(code)
}
