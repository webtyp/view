package tests

import (
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
)

// memStore is a small REAL in-memory backend: a slice of records, not an
// opaque stub. It proves the Backend seam carries a consumer from list
// through every write and back to list.
type memStore struct {
	rows []*conformance.MockRecord
}

func (s *memStore) List() ([]model.Model, error) {
	out := make([]model.Model, 0, len(s.rows))
	for _, r := range s.rows {
		out = append(out, r)
	}
	return out, nil
}

func (s *memStore) Save(recs []model.Model) error {
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
	return nil
}

func (s *memStore) Update(ids []string, rec model.Model, fields []string) error {
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
	return nil
}

func (s *memStore) Delete(ids []string) error {
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
	return nil
}

func TestBackendListSaveUpdateDelete(t *testing.T) {
	store := &memStore{rows: []*conformance.MockRecord{
		{ID: "1", Name: "Alice"},
	}}
	p := view.New(store, &conformance.MockRecord{}, view.WithTitle("t"))

	// Reload → Items projected.
	if err := p.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
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
	if err := s.Save(&conformance.MockRecord{ID: "2", Name: "Bob"}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reload so the index knows the new record.
	if err := p.Reload(); err != nil {
		t.Fatalf("Reload after Save failed: %v", err)
	}
	if items := p.Items(); len(items) != 2 {
		t.Fatalf("unexpected items after Save: %v", items)
	}

	// Update writes only the named columns.
	u := p.(view.Updater)
	if err := u.Update([]string{"1"}, &conformance.MockRecord{Name: "Alicia"}, []string{"name"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Delete removes.
	d := p.(view.Deleter)
	if err := d.Delete("2"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Reload again reflects all of it.
	if err := p.Reload(); err != nil {
		t.Fatalf("second Reload failed: %v", err)
	}
	items := p.Items()
	if len(items) != 1 || items[0].ID != "1" || items[0].Label != "Alicia" {
		t.Fatalf("unexpected items after writes: %v", items)
	}
}
