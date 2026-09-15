package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// memStore is a small REAL in-memory backend: a slice of records, not an
// opaque stub. It proves the Lister seam carries a consumer from list
// through every write and back to list.
type memStore struct {
	rows []*conformance.MockRecord
}

func (s *memStore) List(done func([]model.Model, error)) {
	out := make([]model.Model, 0, len(s.rows))
	for _, r := range s.rows {
		out = append(out, r)
	}
	done(out, nil)
}

func (s *memStore) Save(recs []model.Model, done func(error)) {
	for _, m := range recs {
		rec := m.(*conformance.MockRecord)
		replaced := false
		for i, row := range s.rows {
			if row.ID == rec.ID {
				s.rows[i] = rec
				replaced = true
			}
		}
		if !replaced {
			s.rows = append(s.rows, rec)
		}
	}
	done(nil)
}

func (s *memStore) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	patch := rec.(*conformance.MockRecord)
	for _, row := range s.rows {
		inScope := false
		for _, id := range ids {
			if row.ID == id {
				inScope = true
			}
		}
		if !inScope {
			continue
		}
		for _, f := range fields {
			if f == "name" {
				row.Name = patch.Name
			}
		}
	}
	done(nil)
}

func (s *memStore) Delete(ids []string, done func(error)) {
	kept := s.rows[:0]
	for _, row := range s.rows {
		drop := false
		for _, id := range ids {
			if row.ID == id {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, row)
		}
	}
	s.rows = kept
	done(nil)
}

func TestListerListSaveUpdateDelete(t *testing.T) {
	store := &memStore{rows: []*conformance.MockRecord{
		{ID: "1", Name: "Alice"},
	}}
	p := view.New(store, &conformance.MockRecord{}, view.WithTitle("t"))

	// Reload → Items projected.
	var rerr error
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("Reload failed: %v", rerr)
	}
	if items := p.Items(); len(items) != 1 || items[0].ID != "1" {
		t.Fatalf("unexpected items: %v", items)
	}

	// Select.
	if m := p.Select("1"); m == nil {
		t.Fatalf("expected Select(1) to return the record")
	}

	// Save adds a record.
	s := p.(view.Saver)
	var serr error
	s.Save([]model.Model{&conformance.MockRecord{ID: "2", Name: "Bob"}}, func(e error) { serr = e })
	if serr != nil {
		t.Fatalf("Save failed: %v", serr)
	}

	// Reload so the index knows the new record.
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("Reload after Save failed: %v", rerr)
	}
	if items := p.Items(); len(items) != 2 {
		t.Fatalf("unexpected items after Save: %v", items)
	}

	// Update writes only the named columns.
	u := p.(view.Updater)
	var uerr error
	u.Update([]string{"1"}, &conformance.MockRecord{Name: "Alicia"}, []string{"name"}, func(e error) { uerr = e })
	if uerr != nil {
		t.Fatalf("Update failed: %v", uerr)
	}

	// Delete removes.
	d := p.(view.Deleter)
	var derr error
	d.Delete([]string{"2"}, func(e error) { derr = e })
	if derr != nil {
		t.Fatalf("Delete failed: %v", derr)
	}

	// Reload again reflects all of it.
	p.Reload(func(e error) { rerr = e })
	if rerr != nil {
		t.Fatalf("second Reload failed: %v", rerr)
	}
	items := p.Items()
	if len(items) != 1 || items[0].ID != "1" || items[0].Label != "Alicia" {
		t.Fatalf("unexpected items after writes: %v", items)
	}
}
