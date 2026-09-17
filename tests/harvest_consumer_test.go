package tests

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

// harvestConsumerModule is a router.OperationModule exactly as a domain
// library writes one — the real shape mcp.HarvestOps consumes server-side.
type harvestConsumerModule struct{}

func (harvestConsumerModule) ModelName() string { return "widget" }
func (harvestConsumerModule) MountOperations(r router.OperationRegistry) {
	r.Operation("list", func(ctx router.Context) {}).Public()
}

var _ router.OperationModule = harvestConsumerModule{}

// TestQualifiedNameMatchesHarvestOps is the consumer-shape proof the
// op-namespacing change is built for: the SAME two inputs — a module's
// ModelName() and the bare op name it registers — must produce the
// identical wire string on both sides of the seam. Neither side is handed
// the other's output; if this test ever needed a hand-written qualified
// literal to pass, the seam would still be broken.
func TestQualifiedNameMatchesHarvestOps(t *testing.T) {
	mod := harvestConsumerModule{}

	// Server side: the real mcp.HarvestOps, the one a composition root uses.
	provider := mcp.HarvestOps(mod)
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	serverName := tools[0].Name

	// Client side: view.Ops built from the SAME ModelName() and the SAME
	// bare op name — no qualified literal typed anywhere in this test.
	caller := &recordCaller{}
	view.NewCallerLister(caller,
		view.Ops{Module: mod.ModelName(), List: "list"},
		newMockList,
	).List(func([]model.Model, error) {})

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != serverName {
		t.Errorf("client called %q, server harvested %q — the two sides disagree", caller.calls[0].op, serverName)
	}
}
