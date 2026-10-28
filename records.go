package view

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

// Records is the one in-memory copy of every record the screens of a page have
// loaded, keyed by model name and id. Every presenter built over the same Records
// reads it, so what is saved through one screen shows at once in every other place
// that displays it (a list row, a select). Build ONE per page (appkit does it):
//
//	records := view.NewRecords()
//	caller := records.Caller(rpc.NewCaller(""))
type Records struct {
	tables []*recordTable // a slice, not a map: this file compiles to WASM
}

type recordEntry struct {
	item model.Item // computed once, in Put
	rec  Record
}

type recordTable struct {
	name    string
	entries []recordEntry
	subs    []func()
}

// NewRecords returns an empty copy for one page.
func NewRecords() *Records { return &Records{} }

// Caller wraps c so every lister built over it (NewCallerLister) shares these Records.
func (r *Records) Caller(c router.Caller) router.Caller { return &recordsCaller{Caller: c, records: r} }

// Put inserts or replaces rows by model name and Item().ID, computing each item once.
func (r *Records) Put(rows ...Record) {
	var touched []*recordTable
	for _, row := range rows {
		t := r.table(row.ModelName())
		e := recordEntry{item: row.Item(), rec: row}
		replaced := false
		for i := range t.entries {
			if t.entries[i].item.ID == e.item.ID {
				t.entries[i] = e
				replaced = true
				break
			}
		}
		if !replaced {
			t.entries = append(t.entries, e)
		}
		touched = appendTable(touched, t)
	}
	notify(touched)
}

// Remove drops the rows of modelName with the given ids and tells its subscribers.
func (r *Records) Remove(modelName string, ids ...string) {
	t := r.table(modelName)
	kept := t.entries[:0]
	for _, cur := range t.entries {
		if !containsID(ids, cur.item.ID) {
			kept = append(kept, cur)
		}
	}
	t.entries = kept
	notify([]*recordTable{t})
}

// Get returns the record of modelName with id.
func (r *Records) Get(modelName, id string) (Record, bool) {
	for _, cur := range r.table(modelName).entries {
		if cur.item.ID == id {
			return cur.rec, true
		}
	}
	return nil, false
}

// All returns every record of modelName, in the order they were first put.
func (r *Records) All(modelName string) []Record {
	entries := r.table(modelName).entries
	recs := make([]Record, len(entries))
	for i, e := range entries {
		recs[i] = e.rec
	}
	return recs
}

// items returns the cached entries of modelName for the presenter.
func (r *Records) items(modelName string) []recordEntry {
	return r.table(modelName).entries
}

// OnChange registers fn to run after any Put or Remove that touches modelName.
func (r *Records) OnChange(modelName string, fn func()) {
	t := r.table(modelName)
	t.subs = append(t.subs, fn)
}

func (r *Records) table(name string) *recordTable {
	for _, t := range r.tables {
		if t.name == name {
			return t
		}
	}
	t := &recordTable{name: name}
	r.tables = append(r.tables, t)
	return t
}

func appendTable(ts []*recordTable, t *recordTable) []*recordTable {
	for _, cur := range ts {
		if cur == t {
			return ts
		}
	}
	return append(ts, t)
}

func notify(ts []*recordTable) {
	for _, t := range ts {
		for _, fn := range t.subs {
			fn()
		}
	}
}

func containsID(ids []string, id string) bool {
	for _, cur := range ids {
		if cur == id {
			return true
		}
	}
	return false
}

// recordsCaller is the router.Caller Records.Caller returns; NewCallerLister reads
// the Records from it.
type recordsCaller struct {
	router.Caller
	records *Records
}
