//go:build !wasm

package tests

import (
	stdjson "encoding/json"
	"testing"

	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// What a screen sends must be JSON a server can decode: a save from a generic
// screen arrived as {"records":[{…}} (no closing bracket) and every module
// answered 400 — mjosefa-cms could not save a single device.
func TestCallerListerSendsValidJSON(t *testing.T) {
	caller := &recordCaller{}
	b := view.NewCallerLister(view.NewRecords().Caller(caller),
		view.Ops{Module: "m", List: "l", Save: "s", Update: "u", Delete: "d"}, newMockList)
	p := view.New(b, &conformance.MockRecord{}, view.WithTitle("t"))

	p.(view.Saver).Save([]model.Model{&conformance.MockRecord{ID: "10", Name: "Ten"}}, func(error) {})
	p.(view.Updater).Update([]string{"1"}, &conformance.MockRecord{Name: "Patched"}, []string{"name"}, func(error) {})

	p.Reload(func(error) {}) // delete needs the ids in the index
	p.(view.Deleter).Delete([]string{"1"}, func(error) {})

	var sent []recordCall
	for _, c := range caller.calls {
		if c.op != "m.l" {
			sent = append(sent, c)
		}
	}
	if len(sent) != 3 {
		t.Fatalf("calls = %v, want a save, an update and a delete", sent)
	}
	for _, c := range sent {
		var body []byte
		if err := json.Encode(c.args, &body); err != nil {
			t.Fatalf("%s: encode: %v", c.op, err)
		}
		if !stdjson.Valid(body) {
			t.Errorf("%s sends invalid JSON: %s", c.op, body)
		}
	}
}
