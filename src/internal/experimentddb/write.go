package experimentddb

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type experimentMetaItem struct {
	PK                    string `dynamodbav:"pk"`
	SK                    string `dynamodbav:"sk"`
	GSI1PK                string `dynamodbav:"gsi1pk"`
	GSI1SK                string `dynamodbav:"gsi1sk"`
	EntityType            string `dynamodbav:"entityType"`
	ExperimentID          string `dynamodbav:"experimentId"`
	Name                  string `dynamodbav:"name"`
	Description           string `dynamodbav:"description"`
	Status                string `dynamodbav:"status"`
	SettingsJSON          string `dynamodbav:"settingsJson"`
	CreatedAt             int64  `dynamodbav:"createdAt"`
	StartedAt             *int64 `dynamodbav:"startedAt,omitempty"`
	FinishedAt            *int64 `dynamodbav:"finishedAt,omitempty"`
	Error                 string `dynamodbav:"error"`
	ProgressStage         int    `dynamodbav:"progressStage"`
	ProgressStages        int    `dynamodbav:"progressStages"`
	ProgressStageLabel    string `dynamodbav:"progressStageLabel"`
	ProgressInstanceDone  int    `dynamodbav:"progressInstanceDone"`
	ProgressInstanceTotal int    `dynamodbav:"progressInstanceTotal"`
	GridJSON              string `dynamodbav:"gridJson,omitempty"`
}

type instanceDDBItem struct {
	PK                      string   `dynamodbav:"pk"`
	SK                      string   `dynamodbav:"sk"`
	EntityType              string   `dynamodbav:"entityType"`
	ExperimentID            string   `dynamodbav:"experimentId"`
	InstanceID              string   `dynamodbav:"instanceId"`
	Label                   string   `dynamodbav:"label"`
	Playbook                string   `dynamodbav:"playbook"`
	Symbol                  string   `dynamodbav:"symbol"`
	StrategyConfigJSON      string   `dynamodbav:"strategyConfigJson"`
	ConfigHash              string   `dynamodbav:"configHash"`
	PlaybookKey             string   `dynamodbav:"playbookKey"`
	WorkspaceRelPath        string   `dynamodbav:"workspaceRelPath"`
	Status                  string   `dynamodbav:"status"`
	Error                   string   `dynamodbav:"error"`
	StartedAt               *int64   `dynamodbav:"startedAt,omitempty"`
	FinishedAt              *int64   `dynamodbav:"finishedAt,omitempty"`
	MetricDurationMs        *int64   `dynamodbav:"metricDurationMs,omitempty"`
	MetricCagr              *float64 `dynamodbav:"metricCagr,omitempty"`
	MetricMaxDrawdown       *float64 `dynamodbav:"metricMaxDrawdown,omitempty"`
	MetricSharpe            *float64 `dynamodbav:"metricSharpe,omitempty"`
	MetricSortino           *float64 `dynamodbav:"metricSortino,omitempty"`
	MetricMonthlyReturnAvg  *float64 `dynamodbav:"metricMonthlyReturnAvg,omitempty"`
	MetricMonthlyReturnMed  *float64 `dynamodbav:"metricMonthlyReturnMed,omitempty"`
	MetricYearlyReturnAvg   *float64 `dynamodbav:"metricYearlyReturnAvg,omitempty"`
	MetricYearlyReturnMed   *float64 `dynamodbav:"metricYearlyReturnMed,omitempty"`
	MetricTradesPerMonthAvg *float64 `dynamodbav:"metricTradesPerMonthAvg,omitempty"`
	MetricTradesPerMonthMed *float64 `dynamodbav:"metricTradesPerMonthMed,omitempty"`
	MetricWins              *int32   `dynamodbav:"metricWins,omitempty"`
	MetricLosses            *int32   `dynamodbav:"metricLosses,omitempty"`
	MetricBreakeven         *int32   `dynamodbav:"metricBreakeven,omitempty"`
	MetricWinRate           *float64 `dynamodbav:"metricWinRate,omitempty"`
	MetricMonthlyWinRateAvg *float64 `dynamodbav:"metricMonthlyWinRateAvg,omitempty"`
	MetricMonthlyWinRateMed *float64 `dynamodbav:"metricMonthlyWinRateMed,omitempty"`
	MetricYearlyWinRateAvg  *float64 `dynamodbav:"metricYearlyWinRateAvg,omitempty"`
	MetricYearlyWinRateMed  *float64 `dynamodbav:"metricYearlyWinRateMed,omitempty"`
}

func metaToItem(m ExperimentMeta) experimentMetaItem {
	return experimentMetaItem{
		PK:                    experimentPK(m.ExperimentID),
		SK:                    experimentMetaSK(),
		GSI1PK:                gsi1ExperimentPK,
		GSI1SK:                gsi1SK(m.CreatedAt, m.ExperimentID),
		EntityType:            entityStudy,
		ExperimentID:          m.ExperimentID,
		Name:                  m.Name,
		Description:           m.Description,
		Status:                m.Status,
		SettingsJSON:          m.SettingsJSON,
		CreatedAt:             m.CreatedAt,
		StartedAt:             m.StartedAt,
		FinishedAt:            m.FinishedAt,
		Error:                 m.Error,
		ProgressStage:         m.ProgressStage,
		ProgressStages:        m.ProgressStages,
		ProgressStageLabel:    m.ProgressStageLabel,
		ProgressInstanceDone:  m.ProgressInstanceDone,
		ProgressInstanceTotal: m.ProgressInstanceTotal,
		GridJSON:              m.GridJSON,
	}
}

func instanceToItem(inst InstanceItem) instanceDDBItem {
	return instanceDDBItem{
		PK:                      experimentPK(inst.ExperimentID),
		SK:                      instanceSK(inst.InstanceID),
		EntityType:              entityInstance,
		ExperimentID:            inst.ExperimentID,
		InstanceID:              inst.InstanceID,
		Label:                   inst.Label,
		Playbook:                inst.Playbook,
		Symbol:                  inst.Symbol,
		StrategyConfigJSON:      inst.StrategyConfigJSON,
		ConfigHash:              inst.ConfigHash,
		PlaybookKey:             inst.PlaybookKey,
		WorkspaceRelPath:        inst.WorkspaceRelPath,
		Status:                  inst.Status,
		Error:                   inst.Error,
		StartedAt:               inst.StartedAt,
		FinishedAt:              inst.FinishedAt,
		MetricDurationMs:        inst.MetricDurationMs,
		MetricCagr:              inst.MetricCagr,
		MetricMaxDrawdown:       inst.MetricMaxDrawdown,
		MetricSharpe:            inst.MetricSharpe,
		MetricSortino:           inst.MetricSortino,
		MetricMonthlyReturnAvg:  inst.MetricMonthlyReturnAvg,
		MetricMonthlyReturnMed:  inst.MetricMonthlyReturnMed,
		MetricYearlyReturnAvg:   inst.MetricYearlyReturnAvg,
		MetricYearlyReturnMed:   inst.MetricYearlyReturnMed,
		MetricTradesPerMonthAvg: inst.MetricTradesPerMonthAvg,
		MetricTradesPerMonthMed: inst.MetricTradesPerMonthMed,
		MetricWins:              inst.MetricWins,
		MetricLosses:            inst.MetricLosses,
		MetricBreakeven:         inst.MetricBreakeven,
		MetricWinRate:           inst.MetricWinRate,
		MetricMonthlyWinRateAvg: inst.MetricMonthlyWinRateAvg,
		MetricMonthlyWinRateMed: inst.MetricMonthlyWinRateMed,
		MetricYearlyWinRateAvg:  inst.MetricYearlyWinRateAvg,
		MetricYearlyWinRateMed:  inst.MetricYearlyWinRateMed,
	}
}

// PutExperimentMeta writes or replaces the experiment META item.
func PutExperimentMeta(ctx context.Context, m ExperimentMeta) error {
	if !Enabled() {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	av, err := attributevalue.MarshalMap(metaToItem(m))
	if err != nil {
		return err
	}
	_, err = cli.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table()),
		Item:      av,
	})
	return err
}

// PutInstance writes or replaces one instance item.
func PutInstance(ctx context.Context, inst InstanceItem) error {
	if !Enabled() {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	av, err := attributevalue.MarshalMap(instanceToItem(inst))
	if err != nil {
		return err
	}
	_, err = cli.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table()),
		Item:      av,
	})
	return err
}

// PutInstances batch-writes instances (25 per request).
func PutInstances(ctx context.Context, instances []InstanceItem) error {
	if !Enabled() || len(instances) == 0 {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	const batchSize = 25
	for i := 0; i < len(instances); i += batchSize {
		end := i + batchSize
		if end > len(instances) {
			end = len(instances)
		}
		writes := make([]types.WriteRequest, 0, end-i)
		for _, inst := range instances[i:end] {
			av, err := attributevalue.MarshalMap(instanceToItem(inst))
			if err != nil {
				return err
			}
			writes = append(writes, types.WriteRequest{
				PutRequest: &types.PutRequest{Item: av},
			})
		}
		_, err = cli.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				table(): writes,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateExperimentProgress patches progress fields on experiment META.
func UpdateExperimentProgress(ctx context.Context, experimentID string, stage, stages, instanceDone, instanceTotal int, label string) error {
	if !Enabled() {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	_, err = cli.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table()),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			"sk": &types.AttributeValueMemberS{Value: experimentMetaSK()},
		},
		UpdateExpression: aws.String(
			"SET progressStage = :stage, progressStages = :stages, progressStageLabel = :label, " +
				"progressInstanceDone = :done, progressInstanceTotal = :total",
		),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":stage":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", stage)},
			":stages": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", stages)},
			":label":  &types.AttributeValueMemberS{Value: label},
			":done":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", instanceDone)},
			":total":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", instanceTotal)},
		},
	})
	return err
}

// UpdateExperimentStatus patches status and optional timestamps/error on META.
func UpdateExperimentStatus(ctx context.Context, experimentID, status, errMsg string, started, finished *int64) error {
	if !Enabled() {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	expr := "SET #st = :status, error = :err"
	names := map[string]string{"#st": "status"}
	values := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: status},
		":err":    &types.AttributeValueMemberS{Value: errMsg},
	}
	if started != nil {
		expr += ", startedAt = :started"
		values[":started"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", *started)}
	}
	if finished != nil {
		expr += ", finishedAt = :finished"
		values[":finished"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", *finished)}
	}
	_, err = cli.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table()),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			"sk": &types.AttributeValueMemberS{Value: experimentMetaSK()},
		},
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}

// UpdateInstanceStatus patches instance status fields.
func UpdateInstanceStatus(ctx context.Context, experimentID, instanceID, status, errMsg string, started, finished *int64) error {
	if !Enabled() {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	expr := "SET #st = :status, error = :err"
	names := map[string]string{"#st": "status"}
	values := map[string]types.AttributeValue{
		":status": &types.AttributeValueMemberS{Value: status},
		":err":    &types.AttributeValueMemberS{Value: errMsg},
	}
	if started != nil {
		expr += ", startedAt = :started"
		values[":started"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", *started)}
	}
	if finished != nil {
		expr += ", finishedAt = :finished"
		values[":finished"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", *finished)}
	}
	_, err = cli.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table()),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			"sk": &types.AttributeValueMemberS{Value: instanceSK(instanceID)},
		},
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}

// UpdateInstanceMetrics patches denormalized metric columns on an instance.
func UpdateInstanceMetrics(ctx context.Context, experimentID, instanceID string, metrics map[string]float64) error {
	if !Enabled() || len(metrics) == 0 {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	parts := make([]string, 0, len(metrics))
	values := map[string]types.AttributeValue{}
	for k, v := range metrics {
		ph := ":" + strings.ReplaceAll(k, ".", "_")
		parts = append(parts, k+" = "+ph)
		values[ph] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%g", v)}
	}
	_, err = cli.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table()),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			"sk": &types.AttributeValueMemberS{Value: instanceSK(instanceID)},
		},
		UpdateExpression:          aws.String("SET " + strings.Join(parts, ", ")),
		ExpressionAttributeValues: values,
	})
	return err
}

// DeleteExperiment removes META and all instance items for a experiment.
func DeleteExperiment(ctx context.Context, experimentID string) error {
	if !Enabled() {
		return nil
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return err
	}
	var keys []map[string]types.AttributeValue
	var startKey map[string]types.AttributeValue
	for {
		out, err := cli.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(table()),
			KeyConditionExpression: aws.String("pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return err
		}
		for _, item := range out.Items {
			keys = append(keys, map[string]types.AttributeValue{
				"pk": item["pk"],
				"sk": item["sk"],
			})
		}
		if len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}
	const batchSize = 25
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		writes := make([]types.WriteRequest, 0, end-i)
		for _, key := range keys[i:end] {
			writes = append(writes, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{Key: key},
			})
		}
		_, err = cli.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				table(): writes,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
