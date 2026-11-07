package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// harvestConsumerModule is a crud.Module exactly as a domain
// library writes one — the real shape mcp.HarvestOps consumes server-side.
type harvestConsumerModule struct{}

func (harvestConsumerModule) ModelName() string { return "conformance_mock_record" }
func (harvestConsumerModule) MountOperations(r crud.Registry) {
	r.Read(&conformance.MockRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		return &conformance.MockList{}, nil
	})
}

var _ crud.Module = harvestConsumerModule{}

func TestQualifiedNameMatchesHarvestOps(t *testing.T) {
	mod := harvestConsumerModule{}

	// Server side: the real mcp.HarvestOps, the one a composition root uses.
	expectedTool := mcp.ToolNameOf("conformance_mock_record", crud.Op(&conformance.MockRecord{}, model.Read))
	provider, err := mcp.HarvestOps([]mcp.ToolName{expectedTool}, mod)
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	serverName := string(tools[0].Name)

	// Client side: view.Ops built from the SAME ModelName()
	caller := &recordCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	view.NewCallerLister(recordsCaller,
		view.Ops{Module: mod.ModelName()},
		newMockList,
	).List(crud.ReadArgs{}, func([]view.Record, error) {})

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != serverName {
		t.Errorf("client called %q, server harvested %q — the two sides disagree", caller.calls[0].op, serverName)
	}
}
