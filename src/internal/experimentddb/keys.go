package experimentddb

import "fmt"

const (
	entityStudy    = "experiment"
	entityInstance = "instance"
	gsi1ExperimentPK    = "STUDY"
	maxCreatedInv  = 9999999999999
)

func experimentPK(experimentID string) string {
	return "EXPERIMENT#" + experimentID
}

func experimentMetaSK() string {
	return "META"
}

func instanceSK(instanceID string) string {
	return "INST#" + instanceID
}

// GSI1SK sorts newest experiments first: inverted createdAt (13 digits) + experiment id.
func gsi1SK(createdAtMs int64, experimentID string) string {
	inv := maxCreatedInv - createdAtMs
	if inv < 0 {
		inv = 0
	}
	return fmt.Sprintf("%013d#%s", inv, experimentID)
}
