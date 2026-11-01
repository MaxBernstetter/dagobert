package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/MaxBernstetter/dagobert/src/internal/secrets"
)

const (
	threeHours = 3 * time.Hour
	hourMS     = int64(time.Hour / time.Millisecond)
	machine    = "candle-close"
	activity   = "run-job"
)

var (
	registeredActivity string
	jobMu              sync.Mutex
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		os.Exit(runOnce())
	}
	os.Exit(serve())
}

func client(ctx context.Context) (*sfn.Client, error) {
	return clientWithTimeout(ctx, 0)
}

func clientWithTimeout(ctx context.Context, timeout time.Duration) (*sfn.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(env("AWS_REGION", "us-east-1")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			env("AWS_ACCESS_KEY_ID", "test"),
			env("AWS_SECRET_ACCESS_KEY", "test"),
			"",
		)),
	)
	if err != nil {
		return nil, err
	}
	endpoint := env("SFN_ENDPOINT", "http://floci:4566")
	return sfn.NewFromConfig(cfg, func(o *sfn.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		if timeout > 0 {
			o.HTTPClient = &http.Client{Timeout: timeout}
		}
	}), nil
}

func serve() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	api, err := client(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: %s\n", err)
		return 1
	}
	arn, err := register(ctx, api)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: register: %s\n", err)
		return 1
	}
	fmt.Printf("orchestrator: state machine %s\n", arn)
	clearStaleBackfill()
	dataDir, err := hostDataDirForJobs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: data dir: %s\n", err)
		return 1
	}
	if err := ensureJobDataLayout(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: data layout: %s\n", err)
		return 1
	}

	go clock(ctx, api, arn)
	go serveBackfill(ctx)
	if err := poll(ctx, api); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "orchestrator: %s\n", err)
		return 1
	}
	return 0
}

func runOnce() int {
	ctx := context.Background()
	api, err := client(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: %s\n", err)
		return 1
	}
	closeMS, err := strconv.ParseInt(env("CANDLE_CLOSE_MS", "1700006400000"), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: CANDLE_CLOSE_MS\n")
		return 1
	}
	arn, err := findMachine(ctx, api)
	if err != nil || arn == "" {
		fmt.Fprintf(os.Stderr, "orchestrator: state machine is not registered\n")
		return 1
	}
	if err := start(ctx, api, arn, closeMS); err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: start: %s\n", err)
		return 1
	}
	return 0
}

func register(ctx context.Context, api *sfn.Client) (string, error) {
	arn, err := ensureActivity(ctx, api)
	if err != nil {
		return "", err
	}
	registeredActivity = arn
	def := strings.ReplaceAll(definition, "ACTIVITY_ARN", registeredActivity)
	existing, err := findMachine(ctx, api)
	if err != nil {
		return "", err
	}
	if existing == "" {
		out, err := api.CreateStateMachine(ctx, &sfn.CreateStateMachineInput{
			Name:       aws.String(machine),
			RoleArn:    aws.String("arn:aws:iam::000000000000:role/orchestrator"),
			Definition: aws.String(def),
		})
		if err != nil {
			return "", err
		}
		return aws.ToString(out.StateMachineArn), nil
	}
	_, err = api.UpdateStateMachine(ctx, &sfn.UpdateStateMachineInput{
		StateMachineArn: aws.String(existing),
		Definition:      aws.String(def),
	})
	return existing, err
}

func ensureActivity(ctx context.Context, api *sfn.Client) (string, error) {
	var token *string
	for {
		page, err := api.ListActivities(ctx, &sfn.ListActivitiesInput{NextToken: token})
		if err != nil {
			return "", err
		}
		for _, item := range page.Activities {
			if aws.ToString(item.Name) == activity {
				return aws.ToString(item.ActivityArn), nil
			}
		}
		if page.NextToken == nil || aws.ToString(page.NextToken) == "" {
			break
		}
		token = page.NextToken
	}
	act, err := api.CreateActivity(ctx, &sfn.CreateActivityInput{Name: aws.String(activity)})
	if err != nil {
		return "", err
	}
	return aws.ToString(act.ActivityArn), nil
}

func findMachine(ctx context.Context, api *sfn.Client) (string, error) {
	var token *string
	for {
		page, err := api.ListStateMachines(ctx, &sfn.ListStateMachinesInput{NextToken: token})
		if err != nil {
			return "", err
		}
		for _, m := range page.StateMachines {
			if aws.ToString(m.Name) == machine {
				return aws.ToString(m.StateMachineArn), nil
			}
		}
		if page.NextToken == nil || aws.ToString(page.NextToken) == "" {
			return "", nil
		}
		token = page.NextToken
	}
}

func clock(ctx context.Context, api *sfn.Client, arn string) {
	for {
		next := nextBoundary(time.Now().UTC())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if err := start(ctx, api, arn, next.UnixMilli()); err != nil {
				fmt.Fprintf(os.Stderr, "orchestrator: schedule: %s\n", err)
			}
		}
	}
}

func nextBoundary(now time.Time) time.Time {
	now = now.UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	slot := now.Sub(day) / threeHours
	boundary := day.Add(time.Duration(slot) * threeHours)
	if now.Equal(boundary) {
		return boundary
	}
	return boundary.Add(threeHours)
}

func start(ctx context.Context, api *sfn.Client, arn string, closeMS int64) error {
	body, err := json.Marshal(map[string]any{
		"candleClose": closeMS,
		"symbol":      env("SYMBOL", "BTCUSDT"),
	})
	if err != nil {
		return err
	}
	name := fmt.Sprintf("close-%d-%d", closeMS, time.Now().UnixNano())
	_, err = api.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(arn),
		Name:            aws.String(name),
		Input:           aws.String(string(body)),
	})
	if err == nil {
		fmt.Printf("orchestrator: started %s\n", name)
	}
	return err
}

type taskInput struct {
	Stage       string `json:"stage"`
	CandleClose int64  `json:"candleClose"`
	Symbol      string `json:"symbol"`
	Provider    string `json:"provider,omitempty"`
	From        int64  `json:"from,omitempty"`
}

func runJob(ctx context.Context, in taskInput) (int, error) {
	jobMu.Lock()
	defer jobMu.Unlock()
	return runJobLocked(ctx, in)
}

func poll(ctx context.Context, api *sfn.Client) error {
	for ctx.Err() == nil {
		out, err := api.GetActivityTask(ctx, &sfn.GetActivityTaskInput{
			ActivityArn: aws.String(registeredActivity),
			WorkerName:  aws.String("orchestrator"),
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintf(os.Stderr, "orchestrator: poll: %s\n", err)
			time.Sleep(2 * time.Second)
			continue
		}
		token := aws.ToString(out.TaskToken)
		if token == "" {
			continue
		}
		var in taskInput
		if err := json.Unmarshal([]byte(aws.ToString(out.Input)), &in); err != nil {
			fail(ctx, api, token, err.Error())
			continue
		}
		code, runErr := runJob(ctx, in)
		if runErr != nil || (code != 0 && code != 2) {
			msg := fmt.Sprintf("exit %d", code)
			if runErr != nil {
				msg = runErr.Error()
			}
			fail(ctx, api, token, msg)
			continue
		}
		payload, err := json.Marshal(map[string]any{
			"candleClose": in.CandleClose,
			"symbol":      in.Symbol,
			"stage":       in.Stage,
			"exit":        code,
		})
		if err != nil {
			fail(ctx, api, token, err.Error())
			continue
		}
		_, err = api.SendTaskSuccess(ctx, &sfn.SendTaskSuccessInput{
			TaskToken: aws.String(token),
			Output:    aws.String(string(payload)),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "orchestrator: success: %s\n", err)
		}
	}
	return nil
}

func fail(ctx context.Context, api *sfn.Client, token, cause string) {
	_, err := api.SendTaskFailure(ctx, &sfn.SendTaskFailureInput{
		TaskToken: aws.String(token),
		Error:     aws.String("JobFailed"),
		Cause:     aws.String(cause),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "orchestrator: failure: %s\n", err)
	}
}

func runJobLocked(ctx context.Context, in taskInput) (int, error) {
	if in.CandleClose <= 0 || in.Symbol == "" {
		return 1, fmt.Errorf("missing candle or symbol")
	}
	barOpen := in.CandleClose - int64(threeHours/time.Millisecond)
	data, err := hostDataDirForJobs()
	if err != nil {
		return 1, err
	}
	if err := ensureJobDataLayout(data); err != nil {
		return 1, err
	}
	args := []string{"run", "--rm"}
	args = append(args, dockerJobUserArgs()...)
	args = append(args, "-e", "DATA_DIR=/data", "-v", data+":/data")
	args = append(args, dockerDataPlaneEnvArgs()...)
	switch in.Stage {
	case "ingest":
		from := in.From
		provider := strings.TrimSpace(in.Provider)
		if provider == "" {
			provider = "binance"
		}
		base, quote := splitSeries(in.Symbol)
		if from <= 0 {
			from = ingestFrom(provider, base, quote, barOpen)
		}
		args = append(args,
			"-e", "BINANCE_BASE_URL="+jobBaseURL(),
			"dagobert-ingest",
			"-from", strconv.FormatInt(from, 10),
			"-to", strconv.FormatInt(in.CandleClose, 10),
			"-base", base,
			"-quote", quote,
			"-provider", provider,
		)
	case "process", "calculation-technical-indicator":
		args = append(args, "-e", "SOURCE=live", "dagobert-calculation-technical-indicator", in.Symbol, strconv.FormatInt(barOpen, 10))
	case "indicators":
		args = append(args,
			"-e", "SOURCE=backfill",
			"dagobert-calculation-technical-indicator",
			in.Symbol,
			strconv.FormatInt(in.From, 10),
			strconv.FormatInt(in.CandleClose, 10),
		)
	case "strategy", "calculation-signal":
		args = append(args,
			"-e", "PLAYBOOK=ma-cross",
			"dagobert-calculation-signal",
			in.Symbol, strconv.FormatInt(in.CandleClose, 10),
		)
	case "calculation-market-action":
		envFile, err := secretsEnvFile()
		if err != nil {
			return 1, err
		}
		defer os.Remove(envFile)
		args = append(args,
			"--env-file", envFile,
			"-e", "PLAYBOOK=ma-cross.yaml",
			"dagobert-calculation-market-action",
			in.Symbol, strconv.FormatInt(in.CandleClose, 10),
		)
	case "execution":
		envFile, err := secretsEnvFile()
		if err != nil {
			return 1, err
		}
		defer os.Remove(envFile)
		args = append(args,
			"--env-file", envFile,
			"-e", "PLAYBOOK=ma-cross.yaml",
			"dagobert-execution",
			in.Symbol, strconv.FormatInt(in.CandleClose, 10),
		)
	default:
		return 1, fmt.Errorf("unknown stage %s", in.Stage)
	}
	fmt.Printf("orchestrator: %s %s %d\n", in.Stage, in.Symbol, in.CandleClose)
	writeLive(in, true, 0)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	code := 0
	if err == nil {
		writeLive(in, false, 0)
		return 0, nil
	}
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
		writeLive(in, false, code)
		return code, nil
	}
	writeLive(in, false, 1)
	return 1, err
}

type liveFile struct {
	Stage        string `json:"stage"`
	Symbol       string `json:"symbol"`
	CandleClose  int64  `json:"candleClose"`
	Since        string `json:"since"`
	LastStage    string `json:"lastStage"`
	LastExit     int    `json:"lastExit"`
	LastFinished string `json:"lastFinished"`
}

// ingestFrom starts at the hour after the newest stored open when that hour
// is older than the current bar, so one ingest fills the gap and the current bar.
func splitSeries(sym string) (base, quote string) {
	sym = strings.ToUpper(strings.TrimSpace(sym))
	quotes := []string{"USDT", "USDC", "BUSD", "FDUSD", "TUSD", "BTC", "ETH", "BNB", "EUR", "TRY", "BRL", "GBP", "AUD", "USD"}
	for _, q := range quotes {
		if strings.HasSuffix(sym, q) && len(sym) > len(q) {
			return sym[:len(sym)-len(q)], q
		}
	}
	if sym == "" {
		return "BTC", "USDT"
	}
	return sym, "USDT"
}

func ingestFrom(provider, base, quote string, barOpen int64) int64 {
	latest, ok := latestStored(provider, base, quote)
	if !ok {
		return barOpen
	}
	next := latest + hourMS
	if next < barOpen {
		fmt.Printf("orchestrator: %s %s/%s behind, ingest from %d through the current bar\n", provider, base, quote, next)
		return next
	}
	return barOpen
}

func latestStored(provider, base, quote string) (int64, bool) {
	path := filepath.Join(env("LIVE_DIR", "/data"), "ingest.sqlite")
	if _, err := os.Stat(path); err != nil {
		return 0, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "_pragma=busy_timeout(5000)"}
	db, err := sql.Open("sqlite", u.String()+"&mode=ro")
	if err != nil {
		return 0, false
	}
	defer db.Close()
	var latest sql.NullInt64
	err = db.QueryRow(
		`SELECT MAX(open_time) FROM stored WHERE provider = ? AND base = ? AND quote = ?`,
		provider, base, quote,
	).Scan(&latest)
	if err != nil || !latest.Valid {
		return 0, false
	}
	return latest.Int64, true
}

func writeLive(in taskInput, running bool, exitCode int) {
	dir := env("LIVE_DIR", "/data")
	path := dir + "/live.json"
	cur := liveFile{}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &cur)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if running {
		cur.Stage = in.Stage
		cur.Symbol = in.Symbol
		cur.CandleClose = in.CandleClose
		cur.Since = now
	} else {
		cur.Stage = ""
		cur.LastStage = in.Stage
		cur.LastExit = exitCode
		cur.LastFinished = now
	}
	body, err := json.Marshal(cur)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, body, 0o644)
}

// secretsEnvFile resolves Bybit credentials (SSM → .secrets → process env) into a docker --env-file.
func secretsEnvFile() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	envMap, err := secrets.LoadBybit(ctx, env("SECRETS_FILE", "/secrets/.secrets"))
	if err != nil {
		return "", err
	}
	return secrets.WriteEnvFile(envMap)
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

const definition = `{
  "StartAt": "Ingest",
  "States": {
    "Ingest": {
      "Type": "Task",
      "Resource": "ACTIVITY_ARN",
      "Parameters": {"stage": "ingest", "candleClose.$": "$.candleClose", "symbol.$": "$.symbol"},
      "Retry": [{"ErrorEquals": ["JobFailed"], "IntervalSeconds": 2, "MaxAttempts": 2, "BackoffRate": 2}],
      "Next": "TechnicalIndicator"
    },
    "TechnicalIndicator": {
      "Type": "Task",
      "Resource": "ACTIVITY_ARN",
      "Parameters": {"stage": "calculation-technical-indicator", "candleClose.$": "$.candleClose", "symbol.$": "$.symbol"},
      "Retry": [{"ErrorEquals": ["JobFailed"], "IntervalSeconds": 2, "MaxAttempts": 2, "BackoffRate": 2}],
      "Next": "CalculationSignal"
    },
    "CalculationSignal": {
      "Type": "Task",
      "Resource": "ACTIVITY_ARN",
      "Parameters": {"stage": "calculation-signal", "candleClose.$": "$.candleClose", "symbol.$": "$.symbol"},
      "Retry": [{"ErrorEquals": ["JobFailed"], "IntervalSeconds": 2, "MaxAttempts": 2, "BackoffRate": 2}],
      "Next": "MarketAction"
    },
    "MarketAction": {
      "Type": "Task",
      "Resource": "ACTIVITY_ARN",
      "Parameters": {"stage": "calculation-market-action", "candleClose.$": "$.candleClose", "symbol.$": "$.symbol"},
      "Retry": [{"ErrorEquals": ["JobFailed"], "IntervalSeconds": 2, "MaxAttempts": 2, "BackoffRate": 2}],
      "Next": "Execution"
    },
    "Execution": {
      "Type": "Task",
      "Resource": "ACTIVITY_ARN",
      "Parameters": {"stage": "execution", "candleClose.$": "$.candleClose", "symbol.$": "$.symbol"},
      "Retry": [{"ErrorEquals": ["JobFailed"], "IntervalSeconds": 2, "MaxAttempts": 2, "BackoffRate": 2}],
      "End": true
    }
  }
}`
