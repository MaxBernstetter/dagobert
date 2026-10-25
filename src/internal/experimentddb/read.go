package experimentddb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func itemToMeta(item experimentMetaItem) ExperimentMeta {
	return ExperimentMeta{
		ExperimentID:          item.ExperimentID,
		Name:                  item.Name,
		Description:           item.Description,
		Status:                item.Status,
		SettingsJSON:          item.SettingsJSON,
		CreatedAt:             item.CreatedAt,
		StartedAt:             item.StartedAt,
		FinishedAt:            item.FinishedAt,
		Error:                 item.Error,
		ProgressStage:         item.ProgressStage,
		ProgressStages:        item.ProgressStages,
		ProgressStageLabel:    item.ProgressStageLabel,
		ProgressInstanceDone:  item.ProgressInstanceDone,
		ProgressInstanceTotal: item.ProgressInstanceTotal,
		GridJSON:              item.GridJSON,
	}
}

func itemToInstance(item instanceDDBItem) InstanceItem {
	return InstanceItem{
		ExperimentID:            item.ExperimentID,
		InstanceID:              item.InstanceID,
		Label:                   item.Label,
		Playbook:                item.Playbook,
		Symbol:                  item.Symbol,
		StrategyConfigJSON:      item.StrategyConfigJSON,
		ConfigHash:              item.ConfigHash,
		PlaybookKey:             item.PlaybookKey,
		WorkspaceRelPath:        item.WorkspaceRelPath,
		Status:                  item.Status,
		Error:                   item.Error,
		StartedAt:               item.StartedAt,
		FinishedAt:              item.FinishedAt,
		MetricDurationMs:        item.MetricDurationMs,
		MetricCagr:              item.MetricCagr,
		MetricMaxDrawdown:       item.MetricMaxDrawdown,
		MetricSharpe:            item.MetricSharpe,
		MetricSortino:           item.MetricSortino,
		MetricMonthlyReturnAvg:  item.MetricMonthlyReturnAvg,
		MetricMonthlyReturnMed:  item.MetricMonthlyReturnMed,
		MetricYearlyReturnAvg:   item.MetricYearlyReturnAvg,
		MetricYearlyReturnMed:   item.MetricYearlyReturnMed,
		MetricTradesPerMonthAvg: item.MetricTradesPerMonthAvg,
		MetricTradesPerMonthMed: item.MetricTradesPerMonthMed,
		MetricWins:              item.MetricWins,
		MetricLosses:            item.MetricLosses,
		MetricBreakeven:         item.MetricBreakeven,
		MetricWinRate:           item.MetricWinRate,
		MetricMonthlyWinRateAvg: item.MetricMonthlyWinRateAvg,
		MetricMonthlyWinRateMed: item.MetricMonthlyWinRateMed,
		MetricYearlyWinRateAvg:  item.MetricYearlyWinRateAvg,
		MetricYearlyWinRateMed:  item.MetricYearlyWinRateMed,
	}
}

// GetExperimentMeta loads experiment META by id.
func GetExperimentMeta(ctx context.Context, experimentID string) (*ExperimentMeta, error) {
	if !Enabled() {
		return nil, fmt.Errorf("experimentddb disabled")
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cli.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table()),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			"sk": &types.AttributeValueMemberS{Value: experimentMetaSK()},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, fmt.Errorf("experiment not found")
	}
	var item experimentMetaItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, err
	}
	meta := itemToMeta(item)
	return &meta, nil
}

// ListExperimentMetas returns all experiment summaries (newest first via gsi1).
func ListExperimentMetas(ctx context.Context) ([]ExperimentMeta, error) {
	if !Enabled() {
		return nil, fmt.Errorf("experimentddb disabled")
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return nil, err
	}
	var out []ExperimentMeta
	var startKey map[string]types.AttributeValue
	for {
		resp, err := cli.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(table()),
			IndexName:              aws.String("gsi1"),
			KeyConditionExpression: aws.String("gsi1pk = :pk"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk": &types.AttributeValueMemberS{Value: gsi1ExperimentPK},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}
		for _, raw := range resp.Items {
			var item experimentMetaItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			out = append(out, itemToMeta(item))
		}
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey
	}
	return out, nil
}

// ListInstances returns all instances for a experiment.
func ListInstances(ctx context.Context, experimentID string) ([]InstanceItem, error) {
	if !Enabled() {
		return nil, fmt.Errorf("experimentddb disabled")
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return nil, err
	}
	var out []InstanceItem
	var startKey map[string]types.AttributeValue
	for {
		resp, err := cli.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(table()),
			KeyConditionExpression: aws.String("pk = :pk AND begins_with(sk, :inst)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":   &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
				":inst": &types.AttributeValueMemberS{Value: "INST#"},
			},
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, err
		}
		for _, raw := range resp.Items {
			var item instanceDDBItem
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			out = append(out, itemToInstance(item))
		}
		if len(resp.LastEvaluatedKey) == 0 {
			break
		}
		startKey = resp.LastEvaluatedKey
	}
	return out, nil
}

// GetInstance loads one instance by experiment and instance id.
func GetInstance(ctx context.Context, experimentID, instanceID string) (*InstanceItem, error) {
	if !Enabled() {
		return nil, fmt.Errorf("experimentddb disabled")
	}
	cli, err := ddbClient(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cli.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table()),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: experimentPK(experimentID)},
			"sk": &types.AttributeValueMemberS{Value: instanceSK(instanceID)},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Item) == 0 {
		return nil, fmt.Errorf("experiment instance not found")
	}
	var item instanceDDBItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return nil, err
	}
	inst := itemToInstance(item)
	return &inst, nil
}

// CountFailedInstances returns instances with status failed for a experiment.
func CountFailedInstances(ctx context.Context, experimentID string) (int, error) {
	instances, err := ListInstances(ctx, experimentID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, inst := range instances {
		if inst.Status == "failed" {
			n++
		}
	}
	return n, nil
}
