package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/loopback"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

type testModule struct {
	store  map[string]*conformance.MockRecord
	save   func(d *conformance.MockRecord) error
	remove func(ids []string) error
}

func (m *testModule) ModelName() string { return "testmod" }
func (m *testModule) MountOperations(reg router.OperationRegistry) {
	view.MountSave(reg, "save", m.save).Requires(model.Resource("test"), model.Create)
	view.MountDelete(reg, "delete", m.remove).Requires(model.Resource("test"), model.Delete)
}

func TestMountSave_RoundTrip(t *testing.T) {
	mod := &testModule{
		store: make(map[string]*conformance.MockRecord),
	}
	mod.save = func(d *conformance.MockRecord) error {
		if d.ID == "" {
			d.ID = "generated"
		}
		mod.store[d.ID] = d
		return nil
	}
	caller := view.NewRecords().Caller(loopback.ActingAs("t", "u", mod))
	lister := view.NewCallerLister(caller, view.Ops{Module: "testmod", List: "list", Save: "save"}, func() model.ModelSlice { return &conformance.MockList{} })

	var err error
	lister.(view.Saver).Save([]model.Model{&conformance.MockRecord{ID: "1", Name: "one"}}, func(e error) { err = e })

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(mod.store) != 1 {
		t.Fatalf("expected 1 record saved, got %d", len(mod.store))
	}
	if mod.store["1"].Name != "one" {
		t.Fatalf("expected name 'one', got %v", mod.store["1"].Name)
	}
}

func TestMountSave_EmptyIs400(t *testing.T) {
	mod := &testModule{}
	caller := view.NewRecords().Caller(loopback.ActingAs("t", "u", mod))
	var err error
	caller.Call("testmod.save", &saveArgs{}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != `{"status":400,"title":"invalid","detail":"view: no records to save"}` && err.Error() != `{"status":400,"title":"invalid","detail":"view: no ids given"}` {
		t.Fatalf("expected 400 invalid, got %v", err)
	}
}

func TestMountSave_ErrorKeepsItsStatus(t *testing.T) {
	mod := &testModule{
		save: func(d *conformance.MockRecord) error {
			return model.Conflict("dup")
		},
	}
	caller := view.NewRecords().Caller(loopback.ActingAs("t", "u", mod))
	lister := view.NewCallerLister(caller, view.Ops{Module: "testmod", List: "list", Save: "save"}, func() model.ModelSlice { return &conformance.MockList{} })

	var err error
	lister.(view.Saver).Save([]model.Model{&conformance.MockRecord{ID: "1"}}, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != `{"status":409,"title":"conflict","detail":"dup"}` {
		t.Fatalf("expected 409 conflict dup, got %v", err)
	}
}

func TestMountDelete_RoundTrip(t *testing.T) {
	deletedIDs := []string{}
	mod := &testModule{
		remove: func(ids []string) error {
			deletedIDs = append(deletedIDs, ids...)
			return nil
		},
	}
	caller := view.NewRecords().Caller(loopback.ActingAs("t", "u", mod))
	lister := view.NewCallerLister(caller, view.Ops{Module: "testmod", List: "list", Delete: "delete"}, func() model.ModelSlice { return &conformance.MockList{} })

	var err error
	lister.(view.Deleter).Delete([]string{"1", "2"}, func(e error) { err = e })

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(deletedIDs) != 2 || deletedIDs[0] != "1" || deletedIDs[1] != "2" {
		t.Fatalf("expected deleted ids [1, 2], got %v", deletedIDs)
	}
}

func TestMountDelete_EmptyIs400(t *testing.T) {
	mod := &testModule{}
	caller := view.NewRecords().Caller(loopback.ActingAs("t", "u", mod))
	var err error
	caller.Call("testmod.delete", &deleteArgs{}, nil, func(e error) { err = e })

	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != `{"status":400,"title":"invalid","detail":"view: no records to save"}` && err.Error() != `{"status":400,"title":"invalid","detail":"view: no ids given"}` {
		t.Fatalf("expected 400 invalid, got %v", err)
	}
}

type saveArgs struct {
	Records []model.Model
}

func (a *saveArgs) IsNil() bool { return a == nil }

func (a *saveArgs) EncodeFields(w model.FieldWriter) {
	arr := w.Array("records", len(a.Records))
	for _, r := range a.Records {
		arr.Object(r)
	}
}

type deleteArgs struct {
	IDs []string
}

func (a *deleteArgs) IsNil() bool { return a == nil }

func (a *deleteArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array("ids", len(a.IDs))
	for _, id := range a.IDs {
		ids.String(id)
	}
}
