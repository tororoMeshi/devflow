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
	return testSnapshot(flow.Flow{Purpose: "Test flow purpose", ID: id, Title: fmt.Sprintf("%s title", id), Steps: []flow.Step{{ID: "first", Title: "first", Outcome: "test objective"}, {ID: "second", Title: "second", Outcome: "test objective"}, {ID: "current", Title: "current", Outcome: "test objective"}}})
}
