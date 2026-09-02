package view_test

import (
	"testing"

	"github.com/tinywasm/input"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

type dummyCallerCall struct {
	op   string
	args model.Encodable
}

type dummyCaller struct {
	calls []dummyCallerCall
}

func (c *dummyCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	c.calls = append(c.calls, dummyCallerCall{op: op, args: args})
	if op == "list_op" && into != nil {
		if l, ok := into.(*dummyList); ok {
			a := l.Append().(*dummyRecord)
			a.id, a.name = "1", "One"
			b := l.Append().(*dummyRecord)
			b.id, b.name = "2", "Two"
			c := l.Append().(*dummyRecord)
			c.id, c.name = "3", "Three"
		}
	}
	if done != nil {
		done(nil)
	}
}

func (c *dummyCaller) Dispatch(op string, args model.Encodable) {
	c.calls = append(c.calls, dummyCallerCall{op: op, args: args})
}

var _ router.Caller = (*dummyCaller)(nil)

type dummyRecord struct {
	id   string
	name string
}

func (r *dummyRecord) ModelName() string { return "dummyRecord" }
func (r *dummyRecord) IsNil() bool        { return r == nil }
func (r *dummyRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: input.Text()},
		{Name: "name", Type: input.Text()},
	}
}
func (r *dummyRecord) Pointers() []any { return []any{&r.id, &r.name} }
func (r *dummyRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", r.id)
	w.String("name", r.name)
}
func (r *dummyRecord) DecodeFields(fr model.FieldReader) {}
func (r *dummyRecord) Item() view.Item {
	return view.Item{ID: r.id, Label: r.name}
}

type dummyList struct {
	items []*dummyRecord
}

func (l *dummyList) IsNil() bool            { return l == nil }
func (l *dummyList) Schema() []model.Field  { return nil }
func (l *dummyList) Pointers() []any        { return nil }
func (l *dummyList) DecodeFields(r model.FieldReader) {}
func (l *dummyList) Len() int               { return len(l.items) }
func (l *dummyList) At(i int) model.Fielder { return l.items[i] }
func (l *dummyList) Append() model.Fielder {
	it := &dummyRecord{}
	l.items = append(l.items, it)
	return it
}

func setupView(caller *dummyCaller) view.Presenter {
	return view.New(
		caller,
		&dummyRecord{},
		"list_op",
		func() model.ModelSlice {
			return &dummyList{}
		},
		view.WithSaveOp("save_op"),
		view.WithUpdateOp("update_op"),
		view.WithDeleteOp("delete_op"),
	)
}

func TestSaveRejectsAnEmptyBatch(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	s := p.(view.Saver)

	err := s.Save()
	if err == nil || err.Error() != "view: Save requires at least one record" {
		t.Errorf("expected empty batch error, got %v", err)
	}
}

func TestSaveShipsEveryRecordInOneCall(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	s := p.(view.Saver)

	r1 := &dummyRecord{id: "a", name: "A"}
	r2 := &dummyRecord{id: "b", name: "B"}
	r3 := &dummyRecord{id: "c", name: "C"}

	if err := s.Save(r1, r2, r3); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 caller call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != "save_op" {
		t.Errorf("expected op 'save_op', got %q", caller.calls[0].op)
	}
}

func TestUpdateRejectsAnEmptyIDList(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	u := p.(view.Updater)

	rec := &dummyRecord{name: "Patched"}
	err := u.Update(nil, rec, []string{"name"})
	if err == nil || err.Error() != "view: Update requires at least one id" {
		t.Errorf("expected empty id list error, got %v", err)
	}
}

func TestUpdateRejectsAnEmptyFieldList(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	u := p.(view.Updater)

	rec := &dummyRecord{name: "Patched"}
	err := u.Update([]string{"1"}, rec, nil)
	if err == nil || err.Error() != "view: Update requires at least one field" {
		t.Errorf("expected empty field list error, got %v", err)
	}
}

func TestUpdateShipsIDsFieldsAndRecord(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	u := p.(view.Updater)

	rec := &dummyRecord{name: "Patched"}
	if err := u.Update([]string{"1", "2"}, rec, []string{"name"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != "update_op" {
		t.Errorf("expected op 'update_op', got %q", caller.calls[0].op)
	}
}

func TestDeleteRejectsAnEmptyIDList(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	d := p.(view.Deleter)

	err := d.Delete()
	if err == nil || err.Error() != "view: Delete requires at least one id" {
		t.Errorf("expected empty id list error, got %v", err)
	}
}

func TestDeleteValidatesEveryIDBeforeShipping(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	if err := p.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Reset calls after Reload so we only count Delete calls.
	caller.calls = nil

	d := p.(view.Deleter)
	err := d.Delete("1", "unknown", "2")
	if err == nil || err.Error() != `view: Delete: unknown id "unknown"` {
		t.Errorf("expected unknown id error, got %v", err)
	}

	if len(caller.calls) != 0 {
		t.Errorf("expected 0 calls to caller when validation fails, got %d", len(caller.calls))
	}
}

func TestDeleteShipsEveryIDInOneCall(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	if err := p.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Reset calls after Reload so we only count Delete calls.
	caller.calls = nil

	d := p.(view.Deleter)
	if err := d.Delete("1", "2", "3"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != "delete_op" {
		t.Errorf("expected op 'delete_op', got %q", caller.calls[0].op)
	}
}
