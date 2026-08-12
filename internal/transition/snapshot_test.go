package transition

import (
	"fmt"
	"github.com/tororoMeshi/devflow/internal/flow"
)

func testSnapshot(value flow.Flow) flow.FlowSnapshot {
	snapshot, err := flow.BuildSnapshot(value, flow.FlowSource{})
	if err != nil {
		panic(err)
	}
	return snapshot
}

func stateSnapshot(id string) flow.FlowSnapshot {
	return testSnapshot(flow.Flow{ID: id, Title: fmt.Sprintf("%s title", id), Steps: []flow.Step{{ID: "first", Title: "first", Objective: "test objective"}, {ID: "second", Title: "second", Objective: "test objective"}, {ID: "current", Title: "current", Objective: "test objective"}}})
}
