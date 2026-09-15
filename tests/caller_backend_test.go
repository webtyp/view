package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

type recordCall struct {
	op   string
	args model.Encodable
}

type recordCaller struct {
	calls []recordCall
}

func (c *recordCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	c.calls = append(c.calls, recordCall{op: op, args: args})
	if op == "l" && into != nil {
		if l, ok := into.(*conformance.MockList); ok {
			a := l.Append().(*conformance.MockRecord)
			a.ID, a.Name = "1", "Alice"
		}
	}
	if done != nil {
		done(nil)
	}
}

func (c *recordCaller) Dispatch(op string, args model.Encodable) {
	c.calls = append(c.calls, recordCall{op: op, args: args})
}

var _ router.Caller = (*recordCaller)(nil)

func newMockList() model.ModelSlice { return &conformance.MockList{} }

func TestCallerListerTransport(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerLister(caller,
		view.Ops{List: "l", Save: "s", Update: "u", Delete: "d"},
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
	if len(caller.calls) != 1 || caller.calls[0].op != "s" {
		t.Fatalf("expected 1 call to op %q, got %v", "s", caller.calls)
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
	if len(caller.calls) != 1 || caller.calls[0].op != "u" {
		t.Fatalf("expected 1 call to op %q, got %v", "u", caller.calls)
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
	if len(caller.calls) != 1 || caller.calls[0].op != "d" {
		t.Fatalf("expected 1 call to op %q, got %v", "d", caller.calls)
	}
	pairs = conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "ids", "1") {
		t.Errorf("expected the id in the delete payload, got %v", pairs)
	}
}

func TestCallerListerMissingDeleteOpCarriesNoCapability(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerLister(caller,
		view.Ops{List: "l", Save: "s", Update: "u"},
		newMockList)

	if _, ok := b.(view.Deleter); ok {
		t.Errorf("expected Lister without Delete op to not implement view.Deleter")
	}

	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))
	if _, ok := p.(view.Deleter); ok {
		t.Errorf("expected Presenter to not implement view.Deleter when Delete op is empty")
	}
}
