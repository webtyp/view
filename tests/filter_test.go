package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// ownerLister answers the rows whose Name equals the filter's Name: the server side of a Search rule.
type ownerLister struct {
	rows    []view.Record
	records *view.Records
}

func (l *ownerLister) Records() *view.Records {
	if l.records == nil {
		l.records = view.NewRecords()
	}
	return l.records
}

func (l *ownerLister) Page() view.Page { return view.Page{First: view.All} }

func (l *ownerLister) List(args crud.ReadArgs, done func([]view.Record, error)) {
	want := ""
	if f, ok := args.Filter.(*conformance.MockRecord); ok {
		want = f.Name
	}
	var out []view.Record
	for _, r := range l.rows {
		if r.(*conformance.MockRecord).Name == want {
			out = append(out, r)
		}
	}
	done(out, nil)
}

func ids(items []model.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

// The doctor picks a patient and sees only that patient's records; picking another shows only the other's — never
// the first patient's rows as if they had just been saved.
func TestWithFilter_ListsOnlyTheCurrentFilter(t *testing.T) {
	l := &ownerLister{rows: []view.Record{
		&conformance.MockRecord{ID: "a1", Name: "ana"},
		&conformance.MockRecord{ID: "a2", Name: "ana"},
		&conformance.MockRecord{ID: "l1", Name: "luis"},
	}}
	patient := "ana"
	p := view.New(l, &conformance.MockRecord{}, view.WithFilter(func() model.Model { return &conformance.MockRecord{Name: patient} }))

	p.Reload(nil)
	if got := ids(p.Items()); len(got) != 2 || got[0] != "a1" || got[1] != "a2" {
		t.Fatalf("ana: items = %v, want [a1 a2]", got)
	}
	patient = "luis"
	p.Reload(nil)
	if got := ids(p.Items()); len(got) != 1 || got[0] != "l1" {
		t.Fatalf("luis: items = %v, want [l1] (ana's rows must not come back as fresh)", got)
	}
	l.Records().Put(&conformance.MockRecord{ID: "l2", Name: "luis"}) // saved while luis is shown
	if got := ids(p.Items()); len(got) != 2 || got[0] != "l2" {
		t.Fatalf("after save: items = %v, want l2 first", got)
	}
}
