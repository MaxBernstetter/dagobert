package experimentddb

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/MaxBernstetter/dagobert/src/internal/awsenv"
)

// ExperimentMeta is the DynamoDB experiment META item (business fields + keys).
type ExperimentMeta struct {
	ExperimentID          string
	Name                  string
	Description           string
	Status                string
	SettingsJSON          string
	CreatedAt             int64
	StartedAt             *int64
	FinishedAt            *int64
	Error                 string
	ProgressStage         int
	ProgressStages        int
	ProgressStageLabel    string
	ProgressInstanceDone  int
	ProgressInstanceTotal int
	GridJSON              string
}

// InstanceItem is one experiment instance row in DynamoDB.
type InstanceItem struct {
	ExperimentID            string
	InstanceID              string
	Label                   string
	Playbook                string
	Symbol                  string
	StrategyConfigJSON      string
	ConfigHash              string
	PlaybookKey             string
	WorkspaceRelPath        string
	Status                  string
	Error                   string
	StartedAt               *int64
	FinishedAt              *int64
	MetricDurationMs        *int64
	MetricCagr              *float64
	MetricMaxDrawdown       *float64
	MetricSharpe            *float64
	MetricSortino           *float64
	MetricMonthlyReturnAvg  *float64
	MetricMonthlyReturnMed  *float64
	MetricYearlyReturnAvg   *float64
	MetricYearlyReturnMed   *float64
	MetricTradesPerMonthAvg *float64
	MetricTradesPerMonthMed *float64
	MetricWins              *int32
	MetricLosses            *int32
	MetricBreakeven         *int32
	MetricWinRate           *float64
	MetricMonthlyWinRateAvg *float64
	MetricMonthlyWinRateMed *float64
	MetricYearlyWinRateAvg  *float64
	MetricYearlyWinRateMed  *float64
}

// Enabled is true when catalog backend is dual|ddb and EXPERIMENTS_TABLE is set.
func Enabled() bool {
	switch awsenv.ExperimentCatalogBackend() {
	case "dual", "ddb":
		return awsenv.StudiesTable() != ""
	default:
		return false
	}
}

// WritePrimary is true when catalog writes should target DynamoDB only (caller may still mirror SQLite).
func WritePrimary() bool {
	return awsenv.ExperimentCatalogBackend() == "ddb" && awsenv.StudiesTable() != ""
}

// ReadPrimary is true when catalog reads should come from DynamoDB.
func ReadPrimary() bool {
	return awsenv.ExperimentCatalogBackend() == "ddb" && awsenv.StudiesTable() != ""
}

var (
	clientMu sync.Mutex
	client   *dynamodb.Client
)

func table() string {
	return awsenv.StudiesTable()
}

func ddbClient(ctx context.Context) (*dynamodb.Client, error) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if client != nil {
		return client, nil
	}
	cfg, err := awsenv.Load(ctx)
	if err != nil {
		return nil, err
	}
	client = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if ep := awsenv.ResolveEndpoint(); ep != nil {
			o.BaseEndpoint = ep
		}
	})
	return client, nil
}
