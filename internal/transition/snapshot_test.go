package transition

import (
	"fmt"
	"github.com/8noki8/devflow/internal/flow"
)

func testSnapshot(value flow.Flow) flow.FlowSnapshot {
	snapshot, err := flow.BuildSnapshot(value, flow.FlowSource{})
	if err != nil {
		panic(err)
	}
	return snapshot
}

func stateSnapshot(id string) flow.FlowSnapshot {
	return testSnapshot(flow.Flow{ID: id, Title: fmt.Sprintf("%s title", id), Steps: []flow.Step{{ID: "first", Title: "first", Instruction: "test instruction"}, {ID: "second", Title: "second", Instruction: "test instruction"}, {ID: "current", Title: "current", Instruction: "test instruction"}}})
}
