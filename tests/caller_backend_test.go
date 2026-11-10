package tests

import (
	"strings"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

type recordCall struct {
	op   router.Operation
	args model.Encodable
}

type recordCaller struct {
	calls []recordCall
}

func (c *recordCaller) Call(op router.Operation, args model.Encodable, into model.Decodable, done func(err error)) {
	c.calls = append(c.calls, recordCall{op: op, args: args})
	opRead := router.Operation{Module: "m", Entity: (&conformance.MockRecord{}).ModelName(), Action: model.Read}
	if op == opRead && into != nil {
		if l, ok := into.(*conformance.MockList); ok {
			a := l.Append().(*conformance.MockRecord)
			a.ID, a.Name = "1", "Alice"
		}
	}
	if done != nil {
		done(nil)
	}
}

func (c *recordCaller) Dispatch(op router.Operation, args model.Encodable) {
	c.calls = append(c.calls, recordCall{op: op, args: args})
}

var _ router.Caller = (*recordCaller)(nil)

func newMockList() model.TableSlice { return &conformance.MockList{} }

func TestCallerListerTransport(t *testing.T) {
	caller := &recordCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	b := view.NewCallerLister(recordsCaller,
		view.Ops{Module: "m"},
		newMockList)
	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))

	var rerr error
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("Reload failed: %v", rerr)
	}
	if items := p.Items(); len(items) != 1 || items[0].Label != "Alice" {
		t.Fatalf("unexpected items: %v", items)
	}

	caller.calls = nil

	s := p.(view.Saver)
	r1 := &conformance.MockRecord{ID: "10", Name: "Ten"}
	r2 := &conformance.MockRecord{ID: "11", Name: "Eleven"}
	var serr error
	s.Save([]model.Model{r1, r2}, func(e error) { serr = e })
	if serr != nil {
		t.Fatalf("Save failed: %v", serr)
	}
	expectedCreate := router.Operation{Module: "m", Entity: (&conformance.MockRecord{}).ModelName(), Action: model.Create}
	if len(caller.calls) != 1 || caller.calls[0].op != expectedCreate {
		t.Fatalf("expected 1 call to op %v, got %v", expectedCreate, caller.calls)
	}
	pairs := conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "name", "Ten") || !conformance.Has(pairs, "name", "Eleven") {
		t.Errorf("expected both records' fields in the save payload, got %v", pairs)
	}

	caller.calls = nil

	u := p.(view.Updater)
	var uerr error
	u.Update([]string{"1", "2"}, &conformance.MockRecord{Name: "Patched"}, []string{"name"}, func(e error) { uerr = e })
	if uerr != nil {
		t.Fatalf("Update failed: %v", uerr)
	}
	expectedUpdate := router.Operation{Module: "m", Entity: (&conformance.MockRecord{}).ModelName(), Action: model.Update}
	if len(caller.calls) != 1 || caller.calls[0].op != expectedUpdate {
		t.Fatalf("expected 1 call to op %v, got %v", expectedUpdate, caller.calls)
	}
	pairs = conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "ids", "1") || !conformance.Has(pairs, "ids", "2") || !conformance.Has(pairs, "fields", "name") || !conformance.Has(pairs, "name", "Patched") {
		t.Errorf("expected ids, fields and record fields in the update payload, got %v", pairs)
	}

	caller.calls = nil

	// Delete needs the ids in the index: reload first.
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("second Reload failed: %v", rerr)
	}
	caller.calls = nil
	d := p.(view.Deleter)
	var derr error
	d.Delete([]string{"1"}, func(e error) { derr = e })
	if derr != nil {
		t.Fatalf("Delete failed: %v", derr)
	}
	expectedDelete := router.Operation{Module: "m", Entity: (&conformance.MockRecord{}).ModelName(), Action: model.Delete}
	if len(caller.calls) != 1 || caller.calls[0].op != expectedDelete {
		t.Fatalf("expected 1 call to op %v, got %v", expectedDelete, caller.calls)
	}
	pairs = conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "ids", "1") {
		t.Errorf("expected the id in the delete payload, got %v", pairs)
	}
}

func TestCallerListerMissingDeleteOpCarriesNoCapability(t *testing.T) {
	caller := &recordCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	newList := func() model.TableSlice {
		l := &conformance.MockList{}
		l.SetAllow(model.Read | model.Create | model.Update)
		return l
	}
	b := view.NewCallerLister(recordsCaller,
		view.Ops{Module: "m"},
		newList)

	if _, ok := b.(view.Deleter); ok {
		t.Errorf("expected Lister without Delete op to not implement view.Deleter")
	}

	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))
	if _, ok := p.(view.Deleter); ok {
		t.Errorf("expected Presenter to not implement view.Deleter when Delete op is empty")
	}
}

func TestCallerLister_CapabilitiesFromModelAllow(t *testing.T) {
	caller := &recordCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	newList := func() model.TableSlice {
		l := &conformance.MockList{}
		l.SetAllow(model.Read | model.Create)
		return l
	}
	b := view.NewCallerLister(recordsCaller, view.Ops{Module: "m"}, newList)

	if _, ok := b.(view.Saver); !ok {
		t.Errorf("expected Lister with Read|Create to implement view.Saver")
	}
	if _, ok := b.(view.Updater); ok {
		t.Errorf("expected Lister with Read|Create to NOT implement view.Updater")
	}
	if _, ok := b.(view.Deleter); ok {
		t.Errorf("expected Lister with Read|Create to NOT implement view.Deleter")
	}
}

func TestCallerLister_NoReadPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when model does not allow Read, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "does not allow Read") {
			t.Fatalf("expected panic message containing 'does not allow Read', got %v", r)
		}
	}()

	caller := &recordCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	newList := func() model.TableSlice {
		l := &conformance.MockList{}
		l.SetAllow(model.Create)
		return l
	}
	view.NewCallerLister(recordsCaller, view.Ops{Module: "m"}, newList)
}

func TestCallerLister_OperationsAreTypedPerAction(t *testing.T) {
	caller := &recordCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	b := view.NewCallerLister(recordsCaller, view.Ops{Module: "m"}, newMockList)
	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))

	entity := (&conformance.MockRecord{}).ModelName()

	// 1. Read
	p.Reload(nil)
	wantRead := router.Operation{Module: "m", Entity: entity, Action: model.Read}
	if len(caller.calls) == 0 || caller.calls[0].op != wantRead {
		t.Errorf("read: want %v, got %v", wantRead, caller.calls)
	}

	// 2. Create
	caller.calls = nil
	s := p.(view.Saver)
	s.Save([]model.Model{&conformance.MockRecord{Name: "New"}}, func(error) {})
	wantCreate := router.Operation{Module: "m", Entity: entity, Action: model.Create}
	if len(caller.calls) == 0 || caller.calls[0].op != wantCreate {
		t.Errorf("create: want %v, got %v", wantCreate, caller.calls)
	}

	// 3. Update
	caller.calls = nil
	u := p.(view.Updater)
	u.Update([]string{"1"}, &conformance.MockRecord{Name: "Upd"}, []string{"name"}, func(error) {})
	wantUpdate := router.Operation{Module: "m", Entity: entity, Action: model.Update}
	if len(caller.calls) == 0 || caller.calls[0].op != wantUpdate {
		t.Errorf("update: want %v, got %v", wantUpdate, caller.calls)
	}

	// 4. Delete
	caller.calls = nil
	d := p.(view.Deleter)
	d.Delete([]string{"1"}, func(error) {})
	wantDelete := router.Operation{Module: "m", Entity: entity, Action: model.Delete}
	if len(caller.calls) == 0 || caller.calls[0].op != wantDelete {
		t.Errorf("delete: want %v, got %v", wantDelete, caller.calls)
	}
}

