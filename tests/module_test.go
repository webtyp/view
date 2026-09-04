package tests

import (
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
)

func TestModulePerspective(t *testing.T) {
	caller := &mock.Caller{
		CannedResult: []byte(`[{"id":"m1","name":"Module 1"},{"id":"m2","name":"Module 2"}]`),
	}
	record := &conformance.MockRecord{}

	b := view.NewCallerBackend(caller,
		view.Ops{List: "list_items", Save: "save_item", Delete: "delete_item"},
		func() model.ModelSlice {
			return &conformance.MockList{}
		})
	p := view.New(b, record, view.WithTitle("t"))

	// 1. Reload -> List
	if err := p.Reload(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	items := p.Items()
	if len(items) != 2 || items[0].ID != "m1" || items[1].ID != "m2" {
		t.Errorf("unexpected items: %v", items)
	}

	// 2. Select / Selected
	if p.Selected() != "" {
		t.Errorf("expected initially empty selection")
	}

	m := p.Select("m2")
	if m == nil {
		t.Errorf("expected selected model to be returned")
	} else {
		mr := m.(*conformance.MockRecord)
		if mr.ID != "m2" || mr.Name != "Module 2" {
			t.Errorf("unexpected model fields: %v", mr)
		}
	}
	if p.Selected() != "m2" {
		t.Errorf("expected Selected() to be 'm2', got %q", p.Selected())
	}

	// 3. Save -> SaveOp with payload
	s, ok := p.(view.Saver)
	if !ok {
		t.Fatalf("expected presenter to implement view.Saver")
	}
	if err := s.Save(record); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	var foundSaveCall bool
	for _, call := range caller.Calls {
		if call.Op == "save_item" {
			foundSaveCall = true
		}
	}
	if !foundSaveCall {
		t.Errorf("expected save call to save_item")
	}

	// 4. Delete -> DeleteOp with ID
	d, ok := p.(view.Deleter)
	if !ok {
		t.Fatalf("expected presenter to implement view.Deleter")
	}
	if err := d.Delete("m1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	var foundDeleteCall bool
	for _, call := range caller.Calls {
		if call.Op == "delete_item" {
			foundDeleteCall = true
		}
	}
	if !foundDeleteCall {
		t.Errorf("expected delete call to delete_item")
	}

	// 5. Deselect() -> Selected() is cleared
	p.Deselect()
	if p.Selected() != "" {
		t.Errorf("expected selected to be cleared")
	}
}
