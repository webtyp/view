package tests

import (
	"testing"

	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// A remote list sends the search, the last row shown and the page size to its list operation, as the module's Page
// declares: patients open empty, search 20 at a time, and scroll on from the last row.
func TestCallerLister_SendsSearchAndPage(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerLister(view.NewRecords().Caller(caller),
		view.Ops{Module: "m", List: "l", Page: view.Page{First: 0, Size: 20, More: true}}, newMockList)
	p := view.New(b, &conformance.MockRecord{})

	p.Reload(nil)
	if len(caller.calls) != 0 {
		t.Fatalf("Page.First 0: opening must not call the list operation, got %v", caller.calls)
	}
	p.Search("maria", nil)
	if len(caller.calls) != 1 || caller.calls[0].op != "m.l" {
		t.Fatalf("Search: want one call to m.l, got %v", caller.calls)
	}
	pairs := conformance.Payload(caller.calls[0].args)
	if !conformance.Has(pairs, "search", "maria") || !conformance.Has(pairs, "limit", "20") || !conformance.Has(pairs, "after", "") {
		t.Errorf("Search payload: want search=maria limit=20 after=\"\", got %v", pairs)
	}
}

// The server never returns more than MaxPage rows, whatever the browser asks.
func TestCap(t *testing.T) {
	for _, tt := range []struct{ in, want int64 }{{0, view.MaxPage}, {-1, view.MaxPage}, {20, 20}, {view.MaxPage + 1, view.MaxPage}} {
		if got := view.Cap(tt.in); got != tt.want {
			t.Errorf("Cap(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// ListArgs travels the wire both ways unchanged.
func TestListArgs_RoundTrip(t *testing.T) {
	in := view.ListArgs{Search: "ana", After: "17", Limit: 20}
	pairs := conformance.Payload(&in)
	if !conformance.Has(pairs, "search", "ana") || !conformance.Has(pairs, "after", "17") || !conformance.Has(pairs, "limit", "20") {
		t.Errorf("ListArgs payload: got %v", pairs)
	}
}
