package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/MaxBernstetter/dagobert/src/internal/experimentddb"
)

// Go experiment jobs keep SQLite as the source of truth for KPIs and instance timing.
// When experimentddb.Enabled(), catalog mutations are mirrored to DynamoDB (dual mode).
// When experimentddb.ReadPrimary(), ListExperiments/GetExperiment read from DynamoDB; instance-by-id
// still uses SQLite because DDB has no instance-id GSI.

const ddbSyncTimeout = 5 * time.Second

func ddbSyncCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ddbSyncTimeout)
}

func logDDBSyncErr(op string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "experimentddb sync %s: %v\n", op, err)
	}
}

func experimentMetaFromRecord(rec ExperimentRecord) (experimentddb.ExperimentMeta, error) {
	settingsJSON, err := json.Marshal(rec.Settings)
	if err != nil {
		return experimentddb.ExperimentMeta{}, err
	}
	return experimentddb.ExperimentMeta{
		ExperimentID:               rec.ID,
		Name:                  rec.Name,
		Description:           rec.Description,
		Status:                rec.Status,
		SettingsJSON:          string(settingsJSON),
		CreatedAt:             rec.CreatedAt,
		StartedAt:             rec.StartedAt,
		FinishedAt:            rec.FinishedAt,
		Error:                 rec.Error,
		ProgressStage:         rec.ProgressStage,
		ProgressStages:        rec.ProgressStages,
		ProgressStageLabel:    rec.ProgressStageLabel,
		ProgressInstanceDone:  rec.ProgressInstanceDone,
		ProgressInstanceTotal: rec.ProgressInstanceTotal,
	}, nil
}

func instanceFromExperiment(inst ExperimentInstance) experimentddb.InstanceItem {
	return experimentddb.InstanceItem{
		ExperimentID:            inst.ExperimentID,
		InstanceID:         inst.ID,
		Label:              inst.Label,
		Playbook:           inst.Playbook,
		Symbol:             inst.Symbol,
		StrategyConfigJSON: string(inst.StrategyConfig),
		ConfigHash:         inst.ConfigHash,
		PlaybookKey:        inst.PlaybookKey,
		WorkspaceRelPath:   inst.WorkspaceRelPath,
		Status:             inst.Status,
		Error:              inst.Error,
		StartedAt:          inst.StartedAt,
		FinishedAt:         inst.FinishedAt,
	}
}

func syncExperimentMeta(rec ExperimentRecord) {
	if !experimentddb.Enabled() {
		return
	}
	meta, err := experimentMetaFromRecord(rec)
	if err != nil {
		logDDBSyncErr("meta marshal", err)
		return
	}
	ctx, cancel := ddbSyncCtx()
	defer cancel()
	logDDBSyncErr("PutExperimentMeta", experimentddb.PutExperimentMeta(ctx, meta))
}

func syncExperimentInstances(instances []ExperimentInstance) {
	if !experimentddb.Enabled() {
		return
	}
	items := make([]experimentddb.InstanceItem, len(instances))
	for i, inst := range instances {
		items[i] = instanceFromExperiment(inst)
	}
	ctx, cancel := ddbSyncCtx()
	defer cancel()
	logDDBSyncErr("PutInstances", experimentddb.PutInstances(ctx, items))
}

func (c *Catalog) syncExperimentByID(experimentID string) {
	if !experimentddb.Enabled() {
		return
	}
	rec, err := c.getExperimentSQLite(experimentID)
	if err != nil {
		logDDBSyncErr("load experiment", err)
		return
	}
	syncExperimentMeta(rec)
	syncExperimentInstances(rec.Instances)
}

func (c *Catalog) syncInstanceByID(instanceID string) {
	if !experimentddb.Enabled() {
		return
	}
	inst, err := c.getExperimentInstanceSQLite(instanceID)
	if err != nil {
		logDDBSyncErr("load instance", err)
		return
	}
	ctx, cancel := ddbSyncCtx()
	defer cancel()
	logDDBSyncErr("PutInstance", experimentddb.PutInstance(ctx, instanceFromExperiment(inst)))
}

func recordFromDDB(meta experimentddb.ExperimentMeta, instances []experimentddb.InstanceItem) (ExperimentRecord, error) {
	var settings ExperimentSettings
	if err := json.Unmarshal([]byte(meta.SettingsJSON), &settings); err != nil {
		return ExperimentRecord{}, err
	}
	rec := ExperimentRecord{
		ID:                    meta.ExperimentID,
		Name:                  meta.Name,
		Description:           meta.Description,
		Status:                meta.Status,
		Settings:              settings,
		CreatedAt:             meta.CreatedAt,
		StartedAt:             meta.StartedAt,
		FinishedAt:            meta.FinishedAt,
		Error:                 meta.Error,
		ProgressStage:         meta.ProgressStage,
		ProgressStages:        meta.ProgressStages,
		ProgressStageLabel:    meta.ProgressStageLabel,
		ProgressInstanceDone:  meta.ProgressInstanceDone,
		ProgressInstanceTotal: meta.ProgressInstanceTotal,
	}
	rec.Instances = make([]ExperimentInstance, len(instances))
	for i, inst := range instances {
		rec.Instances[i] = ExperimentInstance{
			ID:               inst.InstanceID,
			ExperimentID:          inst.ExperimentID,
			Label:            inst.Label,
			Playbook:         inst.Playbook,
			Symbol:           inst.Symbol,
			StrategyConfig:   json.RawMessage(inst.StrategyConfigJSON),
			ConfigHash:       inst.ConfigHash,
			PlaybookKey:      inst.PlaybookKey,
			WorkspaceRelPath: inst.WorkspaceRelPath,
			Status:           inst.Status,
			Error:            inst.Error,
			StartedAt:        inst.StartedAt,
			FinishedAt:       inst.FinishedAt,
		}
	}
	return rec, nil
}
