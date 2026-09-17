package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router/mock"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

func TestModulePerspective(t *testing.T) {
	caller := &mock.Caller{
		CannedResult: []byte(`[{"id":"m1","name":"Module 1"},{"id":"m2","name":"Module 2"}]`),
	}
	record := &conformance.MockRecord{}

	b := view.NewCallerLister(caller,
		view.Ops{Module: "m", List: "list_items", Save: "save_item", Delete: "delete_item"},
		func() model.ModelSlice {
			return &conformance.MockList{}
		})
	p := view.New(b, record, view.WithTitle("t"))

	// 1. Reload -> List
	var rerr error
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("reload failed: %v", rerr)
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
	var serr error
	s.Save([]model.Model{record}, func(e error) { serr = e })
	if serr != nil {
		t.Fatalf("save failed: %v", serr)
	}

	var foundSaveCall bool
	for _, call := range caller.Calls {
		if call.Op == "m.save_item" {
			foundSaveCall = true
		}
	}
	if !foundSaveCall {
		t.Errorf("expected save call to m.save_item")
	}

	// 4. Delete -> DeleteOp with ID
	d, ok := p.(view.Deleter)
	if !ok {
		t.Fatalf("expected presenter to implement view.Deleter")
	}
	var derr error
	d.Delete([]string{"m1"}, func(e error) { derr = e })
	if derr != nil {
		t.Fatalf("delete failed: %v", derr)
	}

	var foundDeleteCall bool
	for _, call := range caller.Calls {
		if call.Op == "m.delete_item" {
			foundDeleteCall = true
		}
	}
	if !foundDeleteCall {
		t.Errorf("expected delete call to m.delete_item")
	}

	// 5. Deselect() -> Selected() is cleared
	p.Deselect()
	if p.Selected() != "" {
		t.Errorf("expected selected to be cleared")
	}
}
