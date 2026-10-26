package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
)

func runStudyJob(ctx context.Context, id string, retryFailed bool) {
	switch awsenv.ExperimentRunBackend() {
	case "batched", "sfn":
		runStudyBatched(ctx, id, retryFailed)
	default:
		runStudyJobDockerWhole(ctx, id, retryFailed)
	}
}

func runStudyJobDockerWhole(ctx context.Context, id string, retryFailed bool) {
	code, err := runExperimentDocker(ctx, experimentDockerOpts{
		ExperimentID:     id,
		Phase:       "all",
		RetryFailed: retryFailed,
	})
	finishStudyJob(id, code, err)
}

func finishStudyJob(id string, code int, err error) {
	jobMu.Lock()
	if experimentJob.ID == id {
		experimentJob.Running = false
		experimentJob.Finished = time.Now().UTC().Format(time.RFC3339)
		if experimentJob.Error == "cancelled" {
			// keep cancelled message from POST /experiment/cancel
		} else if err != nil {
			experimentJob.Error = err.Error()
		} else if code != 0 {
			experimentJob.Error = fmt.Sprintf("experiment exit %d", code)
		} else {
			experimentJob.Error = ""
		}
		experimentJobCancel = nil
	}
	jobMu.Unlock()
}

type experimentDockerOpts struct {
	ExperimentID      string
	Phase        string
	InstanceIDs  []string
	RetryFailed  bool
}

func runExperimentDocker(ctx context.Context, opts experimentDockerOpts) (int, error) {
	extra := []string{
		"-e", "EXPERIMENT_PHASE=" + opts.Phase,
	}
	if len(opts.InstanceIDs) > 0 {
		extra = append(extra, "-e", "EXPERIMENT_INSTANCE_IDS="+strings.Join(opts.InstanceIDs, ","))
	}
	// Unique names for parallel simulate batches (Docker --name is global).
	name := experimentContainerName(opts.ExperimentID)
	if opts.Phase == "simulate" && len(opts.InstanceIDs) > 0 {
		suffix := opts.InstanceIDs[0]
		if len(suffix) > 8 {
			suffix = suffix[:8]
		}
		name = experimentContainerName(opts.ExperimentID + "-" + opts.Phase + "-" + suffix)
	} else if opts.Phase != "" && opts.Phase != "all" {
		name = experimentContainerName(opts.ExperimentID + "-" + opts.Phase)
	}
	return runCatalogDocker(ctx, "EXPERIMENT_ID", opts.ExperimentID, "experiment", name, opts.RetryFailed, extra)
}

func runStudyBatched(ctx context.Context, id string, retryFailed bool) {
	var runErr error
	code := 0

	if c, err := runExperimentDocker(ctx, experimentDockerOpts{ExperimentID: id, Phase: "prepare", RetryFailed: retryFailed}); err != nil {
		finishStudyJob(id, c, err)
		return
	} else if c != 0 {
		finishStudyJob(id, c, nil)
		return
	}

	data, err := hostDataDirForJobs()
	if err != nil {
		finishStudyJob(id, 1, err)
		return
	}
	ids, err := readExperimentInstanceCache(data, id)
	if err != nil {
		finishStudyJob(id, 1, err)
		return
	}
	batches := chunkStrings(ids, awsenv.StudyBatchSize())
	if len(batches) == 0 {
		finishStudyJob(id, 1, fmt.Errorf("experiment %s: empty instance cache", id))
		return
	}

	arn := awsenv.ExperimentSFNARN()
	if awsenv.ExperimentRunBackend() == "sfn" && arn != "" {
		if err := runExperimentSFNMap(ctx, id, data, batches); err != nil {
			finishStudyJob(id, 1, err)
			return
		}
	} else {
		maxC := awsenv.StudyMapMaxConcurrency()
		sem := make(chan struct{}, maxC)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, batch := range batches {
			wg.Add(1)
			go func(batch []string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				c, err := runExperimentDocker(ctx, experimentDockerOpts{
					ExperimentID:     id,
					Phase:       "simulate",
					InstanceIDs: batch,
					RetryFailed: retryFailed,
				})
				mu.Lock()
				defer mu.Unlock()
				if err != nil && runErr == nil {
					runErr = err
				}
				if c != 0 && code == 0 {
					code = c
				}
			}(batch)
		}
		wg.Wait()
		if runErr != nil {
			finishStudyJob(id, code, runErr)
			return
		}
		if code != 0 {
			finishStudyJob(id, code, nil)
			return
		}
	}

	c, err := runExperimentDocker(ctx, experimentDockerOpts{ExperimentID: id, Phase: "finalize", RetryFailed: retryFailed})
	finishStudyJob(id, c, err)
}

func readExperimentInstanceCache(dataDir, experimentID string) ([]string, error) {
	path := fmt.Sprintf("%s/experiments/%s/.cache/instances.json", strings.TrimRight(dataDir, "/"), experimentID)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		InstanceIDs []string `json:"instanceIds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc.InstanceIDs, nil
}

func chunkStrings(ids []string, size int) [][]string {
	if len(ids) == 0 {
		return nil
	}
	if size < 1 {
		return [][]string{ids}
	}
	var out [][]string
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[i:end])
	}
	return out
}

func runExperimentSFNMap(ctx context.Context, experimentID, dataDir string, batches [][]string) error {
	arn := awsenv.ExperimentSFNARN()
	cli, err := experimentSFNClient(ctx)
	if err != nil {
		return err
	}
	input := &v1.ExperimentSFNInput{
		ExperimentId: experimentID,
		DataDir: dataDir,
	}
	for _, batch := range batches {
		input.Batches = append(input.Batches, &v1.InstanceBatch{InstanceIds: batch})
	}
	raw, err := protojson.MarshalOptions{UseProtoNames: false}.Marshal(input)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("experiment-%s-%d", experimentID, time.Now().Unix())
	out, err := cli.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(arn),
		Name:            aws.String(name),
		Input:           aws.String(string(raw)),
	})
	if err != nil {
		return err
	}
	execArn := aws.ToString(out.ExecutionArn)
	deadline := time.Now().Add(24 * time.Hour)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("sfn execution timed out: %s", execArn)
		}
		desc, err := cli.DescribeExecution(ctx, &sfn.DescribeExecutionInput{
			ExecutionArn: aws.String(execArn),
		})
		if err != nil {
			return err
		}
		switch desc.Status {
		case types.ExecutionStatusSucceeded:
			return nil
		case types.ExecutionStatusFailed, types.ExecutionStatusTimedOut, types.ExecutionStatusAborted:
			return fmt.Errorf("sfn execution %s: %s", desc.Status, aws.ToString(desc.Cause))
		default:
			time.Sleep(2 * time.Second)
		}
	}
}

func experimentSFNClient(ctx context.Context) (*sfn.Client, error) {
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return nil, err
	}
	return sfn.NewFromConfig(cfg, func(o *sfn.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
		}
	}), nil
}
