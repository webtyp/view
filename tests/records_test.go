package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

func TestRecords_PutReplacesByID(t *testing.T) {
	r := view.NewRecords()
	r.Put(&conformance.MockRecord{ID: "a", Name: "A"})
	r.Put(&conformance.MockRecord{ID: "a", Name: "B"})

	all := r.All("MockRecord")
	if len(all) != 1 {
		t.Fatalf("expected 1 row, got %d", len(all))
	}
	if all[0].(*conformance.MockRecord).Name != "B" {
		t.Errorf("expected name to be B, got %s", all[0].(*conformance.MockRecord).Name)
	}
}

func TestRecords_OnChangeOncePerPut(t *testing.T) {
	r := view.NewRecords()
	calls := 0
	r.OnChange("MockRecord", func() {
		calls++
	})

	r.Put(
		&conformance.MockRecord{ID: "a", Name: "A"},
		&conformance.MockRecord{ID: "b", Name: "B"},
	)

	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRecords_Remove(t *testing.T) {
	r := view.NewRecords()
	r.Put(
		&conformance.MockRecord{ID: "a", Name: "A"},
		&conformance.MockRecord{ID: "b", Name: "B"},
	)

	calls := 0
	r.OnChange("MockRecord", func() {
		calls++
	})

	r.Remove("MockRecord", "a")

	all := r.All("MockRecord")
	if len(all) != 1 {
		t.Fatalf("expected 1 row left, got %d", len(all))
	}
	if all[0].(*conformance.MockRecord).ID != "b" {
		t.Errorf("expected remaining row to be b, got %s", all[0].(*conformance.MockRecord).ID)
	}

	if calls != 1 {
		t.Errorf("expected 1 call to OnChange, got %d", calls)
	}
}

func TestPresenters_ShareRecords(t *testing.T) {
	records := view.NewRecords()

	l1 := &memStore{
		records: records,
		rows: []*conformance.MockRecord{
			{ID: "1", Name: "Original"},
		},
	}

	l2 := &memStore{
		records: records,
		rows: []*conformance.MockRecord{
			{ID: "1", Name: "Original"},
		},
	}

	p1 := view.New(l1, &conformance.MockRecord{})
	p2 := view.New(l2, &conformance.MockRecord{})

	p1.Reload(nil)
	p2.Reload(nil)

	calls := 0
	p2.(interface{ OnChange(func()) }).OnChange(func() {
		calls++
	})

	s := p1.(view.Saver)
	s.Save([]model.Model{&conformance.MockRecord{ID: "1", Name: "Updated"}}, nil)

	items := p2.Items()
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Label != "Updated" {
		t.Errorf("expected p2 item label to be Updated, got %s", items[0].Label)
	}

	if calls != 1 {
		t.Errorf("expected p2 OnChange to be called once, got %d", calls)
	}
}

func TestPresenter_SavedRowFirst(t *testing.T) {
	records := view.NewRecords()
	l := &memStore{
		records: records,
		rows: []*conformance.MockRecord{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B"},
		},
	}
	p := view.New(l, &conformance.MockRecord{})

	p.Reload(nil)

	s := p.(view.Saver)
	s.Save([]model.Model{&conformance.MockRecord{ID: "c", Name: "C"}}, nil)

	items := p.Items()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != "c" {
		t.Errorf("expected first item to be c, got %s", items[0].ID)
	}
	if items[1].ID != "a" || items[2].ID != "b" {
		t.Errorf("expected order c, a, b, got %v", items)
	}
}

func TestNewCallerLister_RequiresRecordsCaller(t *testing.T) {
	plainCaller := &testCaller{}
	ops := view.Ops{Module: "m", List: "l"}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic")
		} else if r != "view: NewCallerLister: caller must come from view.Records.Caller — build it once per page: view.NewRecords().Caller(c)" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()

	view.NewCallerLister(plainCaller, ops, func() model.ModelSlice { return &conformance.MockList{} })
}

type singleAnswerCaller struct {
	answer model.Model
}

func (c *singleAnswerCaller) Call(op string, req model.Encodable, res model.Decodable, done func(error)) {
	if res != nil {
		if rec, ok := res.(*conformance.MockRecord); ok {
			rec.ID = c.answer.(*conformance.MockRecord).ID
			rec.Name = c.answer.(*conformance.MockRecord).Name
		}
	}
	done(nil)
}
func (c *singleAnswerCaller) Dispatch(op string, args model.Encodable) {}

func TestCallerLister_SavePutsServerAnswer(t *testing.T) {
	records := view.NewRecords()
	caller := records.Caller(&singleAnswerCaller{
		answer: &conformance.MockRecord{ID: "x", Name: "Server name"},
	})
	ops := view.Ops{Module: "MockRecord", List: "l", Save: "s"}

	l := view.NewCallerLister(caller, ops, func() model.ModelSlice { return &conformance.MockList{} })

	s := l.(view.Saver)
	s.Save([]model.Model{&conformance.MockRecord{Name: "Client name"}}, func(error) {})

	rec, ok := records.Get("MockRecord", "x")
	if !ok {
		t.Fatalf("expected record with id x to exist")
	}
	if rec.(*conformance.MockRecord).Name != "Server name" {
		t.Errorf("expected Server name, got %s", rec.(*conformance.MockRecord).Name)
	}
}
