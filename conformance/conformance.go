package conformance

import (
	"testing"

	"webtyp.com/input"
	"webtyp.com/model"
	"webtyp.com/view"
)

// FakeLister is a typed view.Lister test double recording typed calls. It
// holds every capability (List/Save/Update/Delete) so a presenter built over
// it carries every capability; capability-absence clauses below use the
// purpose-built minimal doubles instead.
type FakeLister struct {
	Rows []model.Model // what List returns

	Calls         int
	SavedRecords  []model.Model
	UpdatedIDs    []string
	UpdatedFields []string
	UpdatedRecord model.Model
	DeletedIDs    []string
	Err           error // returned by every operation, for error-path clauses
}

func (b *FakeLister) List() ([]model.Model, error) {
	b.Calls++
	if b.Err != nil {
		return nil, b.Err
	}
	out := make([]model.Model, len(b.Rows))
	copy(out, b.Rows)
	return out, nil
}

func (b *FakeLister) Save(recs ...model.Model) error {
	if b.Err != nil {
		return b.Err
	}
	b.SavedRecords = append(b.SavedRecords, recs...)
	return nil
}

func (b *FakeLister) Update(ids []string, rec model.Model, fields []string) error {
	if b.Err != nil {
		return b.Err
	}
	b.UpdatedIDs = append(b.UpdatedIDs, ids...)
	b.UpdatedFields = append(b.UpdatedFields, fields...)
	b.UpdatedRecord = rec
	return nil
}

func (b *FakeLister) Delete(ids ...string) error {
	if b.Err != nil {
		return b.Err
	}
	b.DeletedIDs = append(b.DeletedIDs, ids...)
	return nil
}

var (
	_ view.Lister  = (*FakeLister)(nil)
	_ view.Saver   = (*FakeLister)(nil)
	_ view.Updater = (*FakeLister)(nil)
	_ view.Deleter = (*FakeLister)(nil)
)

// listOnlyLister implements List and nothing else: the double for the
// negative capability clauses below.
type listOnlyLister struct {
	rows []model.Model
}

func (b *listOnlyLister) List() ([]model.Model, error) {
	out := make([]model.Model, len(b.rows))
	copy(out, b.rows)
	return out, nil
}

// listSaveLister implements List+Save only: the double proving the mirror
// rule (a backend with Save yields a Presenter that IS a Saver and is NOT an
// Updater/Deleter).
type listSaveLister struct {
	rows  []model.Model
	saved []model.Model
}

func (b *listSaveLister) List() ([]model.Model, error) {
	out := make([]model.Model, len(b.rows))
	copy(out, b.rows)
	return out, nil
}

func (b *listSaveLister) Save(recs ...model.Model) error {
	b.saved = append(b.saved, recs...)
	return nil
}

// Factory builds the renderer under test around the presenter and returns a Driver.
type Factory struct {
	New func(t *testing.T, p view.Presenter) Driver
}

// Driver simulates user UI interaction over the renderer.
type Driver struct {
	Mount    func()                   // triggers initialization: renderer loads list
	Labels   func() []string          // what the list shows right now
	Select   func(id string)          // simulates picking a row with that id
	SetField func(name, value string) // sets a form field
	Save     func()                   // simulates the save action
	Delete   func()                   // simulates the delete action

	// New simulates the "+" (create new) action.
	New func()
	// Edit simulates picking ⋮ → Editar for the given id (select + unlock).
	Edit func(id string)
	// Cancel simulates the "↺" (undo) action — abandoning a new-record draft
	// or a selected-but-unedited row, back to "+".
	Cancel func()
	// FocusedFieldID returns the id of the field the renderer most recently
	// asked to focus (empty if none). Real focus movement is a WASM-only
	// DOM side effect; this exposes the INTENT so the clauses below can
	// assert it without a live DOM/browser.
	FocusedFieldID func() string
}

// MockRecord is a simulation record for conformance suite.
type MockRecord struct {
	ID   string
	Name string
}

// ModelName implements model.ModuleNaming.
func (m *MockRecord) ModelName() string { return "MockRecord" }

// IsNil implements model.Model.
func (m *MockRecord) IsNil() bool { return m == nil }

// Schema implements model.Fielder.
func (m *MockRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: input.Text()},
		{Name: "name", Type: input.Text()},
	}
}

// Pointers implements model.Fielder.
func (m *MockRecord) Pointers() []any {
	return []any{&m.ID, &m.Name}
}

// EncodeFields implements model.Encodable.
func (m *MockRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
}

// DecodeFields implements model.Decodable.
func (m *MockRecord) DecodeFields(r model.FieldReader) {
	if val, ok := r.String("id"); ok {
		m.ID = val
	}
	if val, ok := r.String("name"); ok {
		m.Name = val
	}
}

// Item implements view.Itemizer.
func (m *MockRecord) Item() view.Item {
	return view.Item{
		ID:          m.ID,
		Label:       m.Name,
		Description: "Desc of " + m.Name,
	}
}

// MockList is a list holding simulation records.
type MockList struct {
	items []*MockRecord
}

// IsNil implements model.Model.
func (m *MockList) IsNil() bool { return m == nil }

// DecodeFields implements model.Decodable.
func (m *MockList) DecodeFields(r model.FieldReader) {}

// Schema implements model.Fielder.
func (m *MockList) Schema() []model.Field { return nil }

// Pointers implements model.Fielder.
func (m *MockList) Pointers() []any { return nil }

// Len implements model.FielderSlice.
func (m *MockList) Len() int { return len(m.items) }

// At implements model.FielderSlice.
func (m *MockList) At(i int) model.Fielder {
	return m.items[i]
}

// Append implements model.FielderSlice.
func (m *MockList) Append() model.Fielder {
	it := &MockRecord{}
	m.items = append(m.items, it)
	return it
}

// Run executes the full set of conformance clauses.
func Run(t *testing.T, f Factory) {
	t.Run("mount_triggers_list_load", func(t *testing.T) {
		fb := &FakeLister{}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()

		if fb.Calls == 0 {
			t.Errorf("expected Mount to trigger a List call, got none")
		}
	})

	t.Run("list_renders_item_labels", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
				&MockRecord{ID: "2", Name: "Bob"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()

		labels := driver.Labels()
		if len(labels) != 2 || labels[0] != "Alice" || labels[1] != "Bob" {
			t.Errorf("expected labels %v, got %v", []string{"Alice", "Bob"}, labels)
		}
	})

	t.Run("select_fills_form", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
				&MockRecord{ID: "2", Name: "Bob"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("2")
		// Editing "name" (not just Select+Save) is what makes this a genuine
		// save: an unedited Save must NOT ship (see unchanged_save_does_not_ship
		// below). The unchanged "id" field surviving into the payload is what
		// actually proves Select loaded record 2's real values into the form —
		// if Select had left it blank/zero, this field would ship empty instead
		// of "2".
		driver.SetField("name", "Bob Updated")
		driver.Save()

		if len(fb.SavedRecords) == 0 {
			t.Fatalf("expected a save call with MockRecord payload")
		}
		savedRecord, ok := fb.SavedRecords[0].(*MockRecord)
		if !ok {
			t.Fatalf("expected saved record to be a *MockRecord, got %T", fb.SavedRecords[0])
		}
		if savedRecord.ID != "2" || savedRecord.Name != "Bob Updated" {
			t.Errorf("expected saved record to be ID '2' (loaded by Select) Name 'Bob Updated' (edited), got ID %q Name %q", savedRecord.ID, savedRecord.Name)
		}
	})

	t.Run("save_ships_form_values", func(t *testing.T) {
		fb := &FakeLister{}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.SetField("name", "X")
		driver.Save()

		if len(fb.SavedRecords) == 0 {
			t.Fatalf("expected a save call with MockRecord payload")
		}
		savedRecord, ok := fb.SavedRecords[0].(*MockRecord)
		if !ok {
			t.Fatalf("expected saved record to be a *MockRecord, got %T", fb.SavedRecords[0])
		}
		if savedRecord.Name != "X" {
			t.Errorf("expected saved record to have Name 'X', got %q", savedRecord.Name)
		}
	})

	// unchanged_save_does_not_ship / revert_edit_is_not_dirty are the CRUD
	// "dirty-check" contract (CRUD_DIRTY_SAVE_MASTER_PLAN.md): any renderer
	// that claims CRUD conformance must gate persistence on "did something
	// actually change since the record was loaded", not "was a field
	// committed" — a blur with no edit is not a save. Every renderer answers
	// this the same way structurally: webtyp/form.Form.IsDirty compares
	// live signals against a baseline snapshotted on load; view/mock.Renderer
	// compares its form map against a baseline snapshotted on Select/Deselect.
	t.Run("unchanged_save_does_not_ship", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		driver.Save() // no SetField at all — nothing changed since the load

		if len(fb.SavedRecords) != 0 {
			t.Fatalf("expected no save call when nothing changed since Select, got one")
		}
	})

	t.Run("revert_edit_is_not_dirty", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		driver.SetField("name", "Temporary")
		driver.SetField("name", "Alice") // back to the value Select loaded
		driver.Save()

		if len(fb.SavedRecords) != 0 {
			t.Fatalf("expected no save call after editing then reverting to the loaded value, got one")
		}
	})

	t.Run("delete_ships_selected_record", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
				&MockRecord{ID: "2", Name: "Bob"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("2")
		driver.Delete()

		if len(fb.DeletedIDs) == 0 {
			t.Errorf("expected a delete call with IDs payload")
		} else if len(fb.DeletedIDs) != 1 || fb.DeletedIDs[0] != "2" {
			t.Errorf("expected deleted ID to be '2', got %v", fb.DeletedIDs)
		}
	})

	t.Run("no_save_capability_without_saver", func(t *testing.T) {
		b := &listOnlyLister{}
		record := &MockRecord{}
		p := view.New(b, record)

		if _, ok := p.(view.Saver); ok {
			t.Errorf("expected presenter to not implement view.Saver when backend does not implement view.Saver")
		}
	})

	t.Run("deselect_clears_selection", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		if p.Selected() != "1" {
			t.Errorf("expected selection to be '1', got %q", p.Selected())
		}
		p.Deselect()
		if p.Selected() != "" {
			t.Errorf("expected selection to be cleared, got %q", p.Selected())
		}
	})

	t.Run("select_unknown_id_returns_nil", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		if p.Selected() != "1" {
			t.Errorf("expected selection to be '1', got %q", p.Selected())
		}
		res := p.Select("unknown")
		if res != nil {
			t.Errorf("expected Select(unknown) to return nil, got %v", res)
		}
		if p.Selected() != "1" {
			t.Errorf("expected selection to remain '1', got %q", p.Selected())
		}
	})

	t.Run("filter_matches_label_and_description", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"}, // description will be "Desc of Alice"
				&MockRecord{ID: "2", Name: "Bob"},   // description will be "Desc of Bob"
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()

		// Case-insensitive match on label:
		res := p.Filter("alice")
		if len(res) != 1 || res[0].ID != "1" {
			t.Errorf("expected 1 result matching 'alice', got %v", res)
		}

		// Case-insensitive match on description:
		res = p.Filter("bOb")
		if len(res) != 1 || res[0].ID != "2" {
			t.Errorf("expected 1 result matching 'bOb', got %v", res)
		}

		// Empty term returns all:
		res = p.Filter("")
		if len(res) != 2 {
			t.Errorf("expected all 2 results on empty term, got %v", res)
		}
	})

	t.Run("delete_unknown_id_errors", func(t *testing.T) {
		fb := &FakeLister{}
		record := &MockRecord{}
		p := view.New(fb, record)

		d, ok := p.(view.Deleter)
		if !ok {
			t.Fatalf("expected presenter to implement view.Deleter")
		}

		err := d.Delete("unknown")
		if err == nil {
			t.Errorf("expected error deleting unknown id, got nil")
		}

		if len(fb.DeletedIDs) != 0 {
			t.Errorf("unexpected delete reaching the backend on unknown ID")
		}
	})

	// "+" and ⋮ Editar must move focus to the form's first field so the user
	// can start typing immediately — a standard behavior every renderer must
	// implement identically, not a crudview-specific nicety.
	t.Run("new_focuses_first_field", func(t *testing.T) {
		fb := &FakeLister{}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.New()

		if got := driver.FocusedFieldID(); got == "" {
			t.Error("expected the \"+\" action to focus the form's first field, got no focus request")
		}
	})

	t.Run("edit_focuses_first_field", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.Edit("1")

		if got := driver.FocusedFieldID(); got == "" {
			t.Error("expected Editar to focus the form's first field, got no focus request")
		}
	})

	// Cancelling a new-record draft must leave NOTHING focused — a stray
	// focused field after "↺" is a leftover from the draft that should have
	// been fully abandoned. Standard behavior, not crudview-specific.
	t.Run("cancel_clears_focus", func(t *testing.T) {
		fb := &FakeLister{}
		record := &MockRecord{}
		p := view.New(fb, record)

		driver := f.New(t, p)
		driver.Mount()
		driver.New()

		if got := driver.FocusedFieldID(); got == "" {
			t.Fatal("sanity check failed: New() did not focus a field")
		}

		driver.Cancel()

		if got := driver.FocusedFieldID(); got != "" {
			t.Errorf("expected Cancel to clear the focused field, got %q", got)
		}
	})

	t.Run("no_delete_capability_without_deleter", func(t *testing.T) {
		b := &listOnlyLister{}
		record := &MockRecord{}
		p := view.New(b, record)

		if _, ok := p.(view.Deleter); ok {
			t.Errorf("expected presenter to not implement view.Deleter when backend does not implement view.Deleter")
		}
	})

	t.Run("no_update_capability_without_updater", func(t *testing.T) {
		b := &listOnlyLister{}
		record := &MockRecord{}
		p := view.New(b, record)

		if _, ok := p.(view.Updater); ok {
			t.Errorf("expected presenter to not implement view.Updater when backend does not implement view.Updater")
		}
	})

	t.Run("saver_capability_mirrors_backend", func(t *testing.T) {
		b := &listSaveLister{}
		// The contract is genuinely shared: the backend itself satisfies the
		// same interface the renderer asserts on the presenter.
		var _ view.Saver = b
		record := &MockRecord{}
		p := view.New(b, record)

		if _, ok := p.(view.Saver); !ok {
			t.Errorf("expected presenter to implement view.Saver when backend implements view.Saver")
		}
		if _, ok := p.(view.Updater); ok {
			t.Errorf("expected presenter to not implement view.Updater when backend does not implement view.Updater")
		}
		if _, ok := p.(view.Deleter); ok {
			t.Errorf("expected presenter to not implement view.Deleter when backend does not implement view.Deleter")
		}
	})

	t.Run("plural_save_delete_and_update", func(t *testing.T) {
		fb := &FakeLister{
			Rows: []model.Model{
				&MockRecord{ID: "1", Name: "Alice"},
				&MockRecord{ID: "2", Name: "Bob"},
			},
		}
		record := &MockRecord{}
		p := view.New(fb, record)

		if err := p.Reload(); err != nil {
			t.Fatalf("reload failed: %v", err)
		}

		s := p.(view.Saver)
		u := p.(view.Updater)
		d := p.(view.Deleter)

		// Plural Save
		r1 := &MockRecord{ID: "10", Name: "Ten"}
		r2 := &MockRecord{ID: "11", Name: "Eleven"}
		if err := s.Save(r1, r2); err != nil {
			t.Fatalf("plural Save failed: %v", err)
		}
		if len(fb.SavedRecords) != 2 {
			t.Errorf("expected 2 saved records in 1 call, got %d", len(fb.SavedRecords))
		}

		// Plural Delete
		if err := d.Delete("1", "2"); err != nil {
			t.Fatalf("plural Delete failed: %v", err)
		}
		if len(fb.DeletedIDs) != 2 {
			t.Errorf("expected 2 deleted ids in 1 call, got %d", len(fb.DeletedIDs))
		}

		// Plural Update
		patch := &MockRecord{Name: "Patched"}
		if err := u.Update([]string{"1", "2"}, patch, []string{"name"}); err != nil {
			t.Fatalf("plural Update failed: %v", err)
		}
		if len(fb.UpdatedIDs) != 2 || len(fb.UpdatedFields) != 1 || fb.UpdatedFields[0] != "name" {
			t.Errorf("expected 2 ids and 1 field in update call, got ids=%v fields=%v", fb.UpdatedIDs, fb.UpdatedFields)
		}
	})
}
