package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/MaxBernstetter/dagobert/src/backtest"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

func handle(ctx context.Context, ev *v1.ExperimentWorkerBatch) error {
	_ = ctx
	dataDir := ev.GetDataDir()
	if dataDir == "" {
		dataDir = os.Getenv("DATA_DIR")
	}
	if dataDir == "" {
		dataDir = "/tmp"
	}
	_ = os.Setenv("DATA_DIR", dataDir)
	_ = os.Setenv("VENUE_SIMULATE", "1")
	if ev.GetExperimentId() == "" {
		return fmt.Errorf("experimentId required")
	}
	if len(ev.GetInstanceIds()) == 0 {
		return fmt.Errorf("instanceIds required")
	}
	// Instance status and metrics go through backtest.RunExperimentSimulate, which
	// uses the Postgres catalog (pgcatalog) when DATABASE_URL is set. This worker
	// does not write the experiment catalog to DynamoDB.
	return backtest.RunExperimentSimulate(dataDir, ev.GetExperimentId(), ev.GetInstanceIds())
}

func decodeWorkerBatch(raw []byte) (*v1.ExperimentWorkerBatch, error) {
	ev := &v1.ExperimentWorkerBatch{}
	if len(raw) > 0 && raw[0] != '{' && raw[0] != '[' {
		if err := proto.Unmarshal(raw, ev); err == nil {
			return ev, nil
		}
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(raw, ev)); err != nil {
		return nil, err
	}
	return ev, nil
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" || os.Getenv("_LAMBDA_SERVER_PORT") != "" {
		lambda.Start(func(ctx context.Context, raw json.RawMessage) error {
			ev, err := decodeWorkerBatch(raw)
			if err != nil {
				return err
			}
			return handle(ctx, ev)
		})
		return
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ev, err := decodeWorkerBatch(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := handle(context.Background(), ev); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
