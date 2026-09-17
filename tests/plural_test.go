package tests

import (
	"testing"

	"webtyp.com/input"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
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
	if op == "m.list_op" && into != nil {
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
func (r *dummyRecord) IsNil() bool       { return r == nil }
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

func (l *dummyList) IsNil() bool                      { return l == nil }
func (l *dummyList) DecodeFields(r model.FieldReader) {}
func (l *dummyList) Len() int                         { return len(l.items) }
func (l *dummyList) At(i int) model.Fielder           { return l.items[i] }
func (l *dummyList) Append() model.Fielder {
	it := &dummyRecord{}
	l.items = append(l.items, it)
	return it
}

func setupView(caller *dummyCaller) view.Presenter {
	b := view.NewCallerLister(caller,
		view.Ops{Module: "m", List: "list_op", Save: "save_op", Update: "update_op", Delete: "delete_op"},
		func() model.ModelSlice { return &dummyList{} })
	return view.New(b, &dummyRecord{}, view.WithTitle("t"))
}

func TestSaveRejectsAnEmptyBatch(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	s := p.(view.Saver)

	var err error
	s.Save(nil, func(e error) { err = e })
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

	var err error
	s.Save([]model.Model{r1, r2, r3}, func(e error) { err = e })
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 caller call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != "m.save_op" {
		t.Errorf("expected op 'm.save_op', got %q", caller.calls[0].op)
	}
	pairs := conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "name", "A") || !conformance.Has(pairs, "name", "B") || !conformance.Has(pairs, "name", "C") {
		t.Errorf("expected all three records' fields in the wire payload, got %v", pairs)
	}
}

func TestUpdateRejectsAnEmptyIDList(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	u := p.(view.Updater)

	rec := &dummyRecord{name: "Patched"}
	var err error
	u.Update(nil, rec, []string{"name"}, func(e error) { err = e })
	if err == nil || err.Error() != "view: Update requires at least one id" {
		t.Errorf("expected empty id list error, got %v", err)
	}
}

func TestUpdateRejectsAnEmptyFieldList(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	u := p.(view.Updater)

	rec := &dummyRecord{name: "Patched"}
	var err error
	u.Update([]string{"1"}, rec, nil, func(e error) { err = e })
	if err == nil || err.Error() != "view: Update requires at least one field" {
		t.Errorf("expected empty field list error, got %v", err)
	}
}

func TestUpdateShipsIDsFieldsAndRecord(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	u := p.(view.Updater)

	rec := &dummyRecord{name: "Patched"}
	var err error
	u.Update([]string{"1", "2"}, rec, []string{"name"}, func(e error) { err = e })
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != "m.update_op" {
		t.Errorf("expected op 'm.update_op', got %q", caller.calls[0].op)
	}
	pairs := conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "ids", "1") || !conformance.Has(pairs, "ids", "2") || !conformance.Has(pairs, "fields", "name") || !conformance.Has(pairs, "name", "Patched") {
		t.Errorf("expected ids, fields and record fields in the wire payload, got %v", pairs)
	}
}

func TestDeleteRejectsAnEmptyIDList(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	d := p.(view.Deleter)

	var err error
	d.Delete(nil, func(e error) { err = e })
	if err == nil || err.Error() != "view: Delete requires at least one id" {
		t.Errorf("expected empty id list error, got %v", err)
	}
}

func TestDeleteValidatesEveryIDBeforeShipping(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	var rerr error
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("Reload failed: %v", rerr)
	}

	// Reset calls after Reload so we only count Delete calls.
	caller.calls = nil

	d := p.(view.Deleter)
	var err error
	d.Delete([]string{"1", "unknown", "2"}, func(e error) { err = e })
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
	var rerr error
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("Reload failed: %v", rerr)
	}

	// Reset calls after Reload so we only count Delete calls.
	caller.calls = nil

	d := p.(view.Deleter)
	var err error
	d.Delete([]string{"1", "2", "3"}, func(e error) { err = e })
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if caller.calls[0].op != "m.delete_op" {
		t.Errorf("expected op 'm.delete_op', got %q", caller.calls[0].op)
	}
	pairs := conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "ids", "1") || !conformance.Has(pairs, "ids", "2") || !conformance.Has(pairs, "ids", "3") {
		t.Errorf("expected all three ids in the wire payload, got %v", pairs)
	}
}

// deferredLister captures done and returns without invoking it: the shape of a
// real transport, whose result arrives in a later turn of the event loop.
type deferredLister struct {
	rows []model.Model
	done func([]model.Model, error)
}

func (l *deferredLister) List(done func([]model.Model, error)) { l.done = done }

// TestListDoneRunsLater is the package-level twin of the conformance clause
// no_blocking_in_list: Reload must project only when List's done finally runs,
// and Items() must be empty before that.
func TestListDoneRunsLater(t *testing.T) {
	dl := &deferredLister{
		rows: []model.Model{&dummyRecord{id: "1", name: "One"}},
	}
	p := view.New(dl, &dummyRecord{})

	p.Reload(func(error) {})
	if items := p.Items(); len(items) != 0 {
		t.Fatalf("expected no items before done runs, got %v", items)
	}

	released := make(chan struct{})
	go func() {
		dl.done(dl.rows, nil)
		close(released)
	}()
	<-released

	items := p.Items()
	if len(items) != 1 || items[0].ID != "1" || items[0].Label != "One" {
		t.Fatalf("expected item 1/One after deferred done, got %v", items)
	}
}

// TestValidationErrorsTravelThroughDone pins the one-channel-of-error rule:
// every programming error save/update/delete can produce is delivered through
// done — the methods return nothing, so there is exactly one way to report a
// failure. Messages are compared word for word: tests depend on them.
func TestValidationErrorsTravelThroughDone(t *testing.T) {
	caller := &dummyCaller{}
	p := setupView(caller)
	s := p.(view.Saver)
	u := p.(view.Updater)
	d := p.(view.Deleter)

	cases := []struct {
		name string
		run  func(done func(error))
		want string
	}{
		{"save_empty_batch", func(done func(error)) { s.Save(nil, done) }, "view: Save requires at least one record"},
		{"save_nil_record", func(done func(error)) { s.Save([]model.Model{nil}, done) }, "view: Save payload is nil"},
		{"update_no_ids", func(done func(error)) { u.Update(nil, &dummyRecord{}, []string{"name"}, done) }, "view: Update requires at least one id"},
		{"update_no_fields", func(done func(error)) { u.Update([]string{"1"}, &dummyRecord{}, nil, done) }, "view: Update requires at least one field"},
		{"update_nil_record", func(done func(error)) { u.Update([]string{"1"}, nil, []string{"name"}, done) }, "view: Update record is nil"},
		{"delete_no_ids", func(done func(error)) { d.Delete(nil, done) }, "view: Delete requires at least one id"},
		{"delete_unknown_id", func(done func(error)) { d.Delete([]string{"unknown"}, done) }, `view: Delete: unknown id "unknown"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got error
			tc.run(func(err error) { got = err })
			if got == nil || got.Error() != tc.want {
				t.Errorf("expected error %q via done, got %v", tc.want, got)
			}
		})
	}
}
