package command

import (
	"fmt"
	"github.com/8noki8/devflow/internal/flow"
)

func testSnapshot(id string) flow.FlowSnapshot {
	return testSnapshotForStep(id, "first")
}

func testSnapshotForStep(id, stepID string) flow.FlowSnapshot {
	steps := []flow.Step{{ID: stepID, Title: stepID, Instruction: "test instruction"}}
	snapshot, err := flow.BuildSnapshot(flow.Flow{ID: id, Title: fmt.Sprintf("%s title", id), Steps: steps}, flow.FlowSource{})
	if err != nil {
		panic(err)
	}
	return snapshot
}
