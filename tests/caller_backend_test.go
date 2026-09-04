package tests

import (
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
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

func TestCallerBackendTransport(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerBackend(caller,
		view.Ops{List: "l", Save: "s", Update: "u", Delete: "d"},
		newMockList)
	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))

	if err := p.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if items := p.Items(); len(items) != 1 || items[0].Label != "Alice" {
		t.Fatalf("unexpected items: %v", items)
	}

	caller.calls = nil

	s := p.(view.Saver)
	r1 := &conformance.MockRecord{ID: "10", Name: "Ten"}
	r2 := &conformance.MockRecord{ID: "11", Name: "Eleven"}
	if err := s.Save(r1, r2); err != nil {
		t.Fatalf("Save failed: %v", err)
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
	if err := u.Update([]string{"1", "2"}, &conformance.MockRecord{Name: "Patched"}, []string{"name"}); err != nil {
		t.Fatalf("Update failed: %v", err)
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
	if err := p.Reload(); err != nil {
		t.Fatalf("second Reload failed: %v", err)
	}
	caller.calls = nil
	d := p.(view.Deleter)
	if err := d.Delete("1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].op != "d" {
		t.Fatalf("expected 1 call to op %q, got %v", "d", caller.calls)
	}
	pairs = conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "ids", "1") {
		t.Errorf("expected the id in the delete payload, got %v", pairs)
	}
}

func TestCallerBackendMissingDeleteOpCarriesNoCapability(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerBackend(caller,
		view.Ops{List: "l", Save: "s", Update: "u"},
		newMockList)

	if _, ok := b.(view.Deleter); ok {
		t.Errorf("expected Backend without Delete op to not implement view.Deleter")
	}

	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))
	if _, ok := p.(view.Deleter); ok {
		t.Errorf("expected Presenter to not implement view.Deleter when Delete op is empty")
	}
}
