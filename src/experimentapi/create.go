package experimentapi

import (
	"context"
	"fmt"
	"log"
	"github.com/MaxBernstetter/dagobert/src/experimentapi/persist"
	"strings"

	"github.com/MaxBernstetter/dagobert/src/experimentapi/experimentdb"
	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type materializePayload struct {
	Mode         v1.ExperimentCreateMode
	Playbook     string
	Axes         []*v1.GridAxis
	Specs        []*v1.ExperimentInstanceSpec
	PlannedTotal int
}

func (s *Server) createStudy(ctx context.Context, req *v1.CreateExperimentRequest) (*v1.Experiment, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	if err := validateParallelism(int(req.GetSettings().GetParallelism()), parallelismMax()); err != nil {
		return nil, err
	}
	merged := mergeSettings(req.Settings)
	fallbackSymbol := strings.ToUpper(merged.Base + merged.Quote)

	var plannedTotal int
	var specs []*v1.ExperimentInstanceSpec
	var payload materializePayload

	mode := req.Mode
	if mode == v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_UNSPECIFIED {
		if req.Grid != nil {
			mode = v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_GRID
		} else {
			mode = v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_MANUAL
		}
	}

	switch mode {
	case v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_GRID:
		if req.Grid == nil {
			return nil, fmt.Errorf("grid required")
		}
		playbook := normalizePlaybook(req.Grid.Playbook)
		if playbook == "" {
			playbook = normalizePlaybook(req.Playbook)
		}
		if playbook == "" {
			playbook = "ma-cross"
		}
		axes := req.Grid.Axes
		if len(axes) == 0 {
			return nil, fmt.Errorf("grid.axes required")
		}
		gridSpecs, err := specsFromGrid(playbook, axes, fallbackSymbol)
		if err != nil {
			return nil, err
		}
		specs = gridSpecs
		plannedTotal = len(specs)
		payload = materializePayload{Mode: v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_GRID, Playbook: playbook, Axes: axes, PlannedTotal: plannedTotal}
	case v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_MANUAL:
		specs = req.Instances
		if len(specs) == 0 {
			return nil, fmt.Errorf("at least one instance required")
		}
		if len(specs) > maxGridCombos {
			return nil, fmt.Errorf("Too many instances (%d); max is %d", len(specs), maxGridCombos)
		}
		playbook := normalizePlaybook(specs[0].Playbook)
		if playbook == "" {
			playbook = normalizePlaybook(req.Playbook)
		}
		if playbook == "" {
			playbook = "ma-cross"
		}
		plannedTotal = len(specs)
		payload = materializePayload{Mode: v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_MANUAL, Playbook: playbook, Specs: specs, PlannedTotal: plannedTotal}
	default:
		return nil, fmt.Errorf("mode must be grid or manual")
	}

	experimentID := uuid.New()
	created := timeNowMs()
	label := "Materializing"
	if plannedTotal <= syncMaterialize {
		label = ""
	}

	row, err := s.q.InsertExperiment(ctx, experimentdb.InsertExperimentParams{
		ID:                    experimentID,
		Name:                  name,
		Description:           req.Description,
		Status:                "materializing",
		Error:                 "",
		Provider:              merged.Provider,
		Base:                  merged.Base,
		Quote:                 merged.Quote,
		FromMs:                merged.FromMs,
		ToMs:                  merged.ToMs,
		InitialUsdc:           merged.InitialUsdc,
		InitialUsdt:           merged.InitialUsdt,
		InitialBase:           merged.InitialBase,
		TargetWeightPct:       merged.TargetWeightPct,
		CashReserveQuote:      merged.CashReserveQuote,
		MinOrderQuote:         merged.MinOrderQuote,
		RedownloadMissing:     merged.RedownloadMissing,
		Algo:                  merged.Algo,
		SpreadBps:             merged.SpreadBps,
		SlippageBps:           merged.SlippageBps,
		FeeBps:                merged.FeeBps,
		Parallelism:           persist.ParallelismToDB(merged.Parallelism),
		CreatedAtMs:           created,
		StartedAtMs:           pgtype.Int8{},
		FinishedAtMs:          pgtype.Int8{},
		ProgressStage:         0,
		ProgressStages:        0,
		ProgressStageLabel:    label,
		ProgressInstanceDone:  0,
		ProgressInstanceTotal: int32(plannedTotal),
	})
	if err != nil {
		return nil, err
	}

	summary := experimentFromRow(row, nil, nil)

	if plannedTotal <= syncMaterialize {
		if err := s.materialize(ctx, experimentID, merged, payload); err != nil {
			return nil, err
		}
		got, err := s.q.GetExperiment(ctx, experimentID)
		if err != nil {
			return nil, err
		}
		summary = experimentFromRow(got, nil, nil)
	} else {
		go s.materializeAsync(experimentID, merged, payload)
	}

	return summary, nil
}

func (s *Server) materializeAsync(experimentID uuid.UUID, merged *v1.ExperimentSettings, payload materializePayload) {
	ctx := context.Background()
	if err := s.materialize(ctx, experimentID, merged, payload); err != nil {
		if ferr := s.q.MarkExperimentFailed(ctx, experimentdb.MarkExperimentFailedParams{
			ID:           experimentID,
			Error:        err.Error(),
			FinishedAtMs: pgtype.Int8{Int64: timeNowMs(), Valid: true},
		}); ferr != nil {
			log.Printf("experiment %s: could not record the failure %q: %v", experimentID, err, ferr)
		}
	}
}

func (s *Server) materialize(ctx context.Context, experimentID uuid.UUID, merged *v1.ExperimentSettings, payload materializePayload) error {
	experiment, err := s.q.GetExperiment(ctx, experimentID)
	if err != nil {
		return err
	}
	if experiment.Status != "materializing" {
		return nil
	}

	fallbackSymbol := strings.ToUpper(merged.Base + merged.Quote)
	var specs []*v1.ExperimentInstanceSpec
	switch payload.Mode {
	case v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_GRID:
		specs, err = specsFromGrid(payload.Playbook, payload.Axes, fallbackSymbol)
	case v1.ExperimentCreateMode_EXPERIMENT_CREATE_MODE_MANUAL:
		specs = payload.Specs
	default:
		return fmt.Errorf("unknown materialize mode")
	}
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	total := len(specs)
	const batchSize = 500
	for i := 0; i < len(specs); i += batchSize {
		end := i + batchSize
		if end > len(specs) {
			end = len(specs)
		}
		chunk := specs[i:end]
		for _, spec := range chunk {
			if err := s.insertOneInstance(ctx, experimentID, merged, spec, seen); err != nil {
				return err
			}
		}
		done := end
		if perr := s.q.UpdateExperimentProgress(ctx, experimentdb.UpdateExperimentProgressParams{
			ID:                    experimentID,
			ProgressStage:         0,
			ProgressStages:        0,
			ProgressStageLabel:    fmt.Sprintf("Materializing %d/%d", done, total),
			ProgressInstanceDone:  int32(done),
			ProgressInstanceTotal: int32(total),
		}); perr != nil {
			log.Printf("experiment %s: materialize progress not saved: %v", experimentID, perr)
		}
	}

	return s.q.MarkExperimentDraft(ctx, experimentdb.MarkExperimentDraftParams{
		ID:                   experimentID,
		ProgressInstanceDone: int32(total),
	})
}

func (s *Server) insertOneInstance(
	ctx context.Context,
	experimentID uuid.UUID,
	merged *v1.ExperimentSettings,
	spec *v1.ExperimentInstanceSpec,
	seen map[string]struct{},
) error {
	playbook := normalizePlaybook(spec.Playbook)
	if playbook == "" {
		playbook = "ma-cross"
	}
	defaultSeries := strings.ToUpper(merged.Base + merged.Quote)
	sym := strings.TrimSpace(spec.Symbol)
	if sym == "" {
		sym = defaultSeries
	}
	sym = strings.ToUpper(sym)
	if sym == "" {
		return fmt.Errorf("missing market")
	}
	raw := strategyMapFromSpec(spec)
	if raw == nil {
		return fmt.Errorf("strategy config required (maCross or signalConsensus)")
	}
	norm, err := normalizeInstanceConfig(playbook, raw)
	if err != nil {
		return err
	}
	uniq := sym + "|" + norm.Hash
	if _, ok := seen[uniq]; ok {
		return fmt.Errorf("duplicate strategy config for %s", sym)
	}
	seen[uniq] = struct{}{}

	instID := uuid.New()
	label := strings.TrimSpace(spec.Label)
	if label == "" {
		label = fmt.Sprintf("%s %s", sym, norm.LabelFrag)
	}
	rel := fmt.Sprintf("experiments/%s/instances/%s", experimentID, instID)
	params := norm.CfgParams
	params.ID = instID
	params.ExperimentID = experimentID
	params.Label = label
	params.Playbook = norm.Playbook
	params.Symbol = sym
	params.ConfigHash = norm.Hash
	params.PlaybookKey = norm.EntryID + "#" + norm.Hash
	params.WorkspaceRelPath = rel
	params.Status = "draft"
	params.Error = ""
	_, err = s.q.InsertInstance(ctx, params)
	return err
}
