package main

import (
	"fmt"
	"os"

	"github.com/MaxBernstetter/dagobert/src/calculation-market-action/internal/job"
)

func main() {
	exitCode, detail := job.Run(os.Args[1:])
	fmt.Printf("DETAIL=%d\n", detail)
	os.Exit(composeExit(exitCode))
}

func composeExit(code int) int {
	if code == 2 && os.Getenv("COMPOSE_OK_ON_ABORT") == "1" {
		return 0
	}
	return code
}
