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

type recordTable struct {
	name string
	rows []model.Model
	subs []func()
}

// NewRecords returns an empty copy for one page.
func NewRecords() *Records { return &Records{} }

// Caller wraps c so every lister built over it (NewCallerLister) shares these Records.
func (r *Records) Caller(c router.Caller) router.Caller { return &recordsCaller{Caller: c, records: r} }

// Put inserts or replaces rows by model name and id (model.RowOf(row).ID), then
// tells each touched model's subscribers once.
func (r *Records) Put(rows ...model.Model) {
	var touched []*recordTable
	for _, row := range rows {
		t := r.table(row.ModelName())
		id := model.RowOf(row).ID
		replaced := false
		for i, cur := range t.rows {
			if model.RowOf(cur).ID == id {
				t.rows[i] = row
				replaced = true
				break
			}
		}
		if !replaced {
			t.rows = append(t.rows, row)
		}
		touched = appendTable(touched, t)
	}
	notify(touched)
}

// Remove drops the rows of modelName with the given ids and tells its subscribers.
func (r *Records) Remove(modelName string, ids ...string) {
	t := r.table(modelName)
	kept := t.rows[:0]
	for _, cur := range t.rows {
		if !containsID(ids, model.RowOf(cur).ID) {
			kept = append(kept, cur)
		}
	}
	t.rows = kept
	notify([]*recordTable{t})
}

// Get returns the record of modelName with id.
func (r *Records) Get(modelName, id string) (model.Model, bool) {
	for _, cur := range r.table(modelName).rows {
		if model.RowOf(cur).ID == id {
			return cur, true
		}
	}
	return nil, false
}

// All returns every record of modelName, in the order they were first put.
func (r *Records) All(modelName string) []model.Model { return r.table(modelName).rows }

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
