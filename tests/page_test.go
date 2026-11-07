package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// A remote list sends the search, the last row shown and the page size to its list operation, as the module's Page
// declares: patients open empty, search 20 at a time, and scroll on from the last row.
func TestCallerLister_SendsSearchAndPage(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerLister(view.NewRecords().Caller(caller),
		view.Ops{Module: "m", Page: view.Page{First: 0, Size: 20, More: true}}, newMockList)
	p := view.New(b, &conformance.MockRecord{})

	p.Reload(nil)
	if len(caller.calls) != 0 {
		t.Fatalf("Page.First 0: opening must not call the list operation, got %v", caller.calls)
	}
	p.Search("maria", nil)
	expectedOp := "m." + crud.Op(&conformance.MockRecord{}, model.Read)
	if len(caller.calls) != 1 || caller.calls[0].op != expectedOp {
		t.Fatalf("Search: want one call to %s, got %v", expectedOp, caller.calls)
	}
	pairs := conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "search", "maria") || !conformance.Has(pairs, "limit", "20") || !conformance.Has(pairs, "after", "") {
		t.Errorf("Search payload: want search=maria limit=20 after=\"\", got %v", pairs)
	}
}
