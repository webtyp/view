package conformance

import (
	"testing"

	"github.com/tinywasm/input"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

// FakeCall records one invocation the suite (or a consumer's module test) can inspect.
type FakeCall struct {
	Op   string
	Args model.Encodable
}

// FakeCaller is a codec-free router.Caller test double. Unlike router/mock.Caller it does
// NOT decode a wire response — it fills the typed target directly via Reply — so this
// package (and any renderer or module that imports it for tests) depends only on model and
// router, never a codec. That is the same discipline router/conformance keeps: the arnés of
// a contract must not drag in an implementation.
type FakeCaller struct {
	Calls []FakeCall
	// Reply fills `into` with canned TYPED data for op (no serialization); nil = no result.
	Reply func(op string, into model.Decodable)
	// Err, if set, is what every Call reports (and suppresses Reply).
	Err error
}

func (c *FakeCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	c.Calls = append(c.Calls, FakeCall{Op: op, Args: args})
	if c.Err == nil && c.Reply != nil && into != nil {
		c.Reply(op, into)
	}
	if done != nil {
		done(c.Err)
	}
}

func (c *FakeCaller) Dispatch(op string, args model.Encodable) {
	c.Calls = append(c.Calls, FakeCall{Op: op, Args: args})
}

var _ router.Caller = (*FakeCaller)(nil)

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
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()

		found := false
		for _, call := range caller.Calls {
			if call.Op == "test_list_op" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected call to %q, but was not found in %v", "test_list_op", caller.Calls)
		}
	})

	t.Run("list_renders_item_labels", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				l := into.(*MockList)
				a := l.Append().(*MockRecord)
				a.ID, a.Name = "1", "Alice"
				b := l.Append().(*MockRecord)
				b.ID, b.Name = "2", "Bob"
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()

		labels := driver.Labels()
		if len(labels) != 2 || labels[0] != "Alice" || labels[1] != "Bob" {
			t.Errorf("expected labels %v, got %v", []string{"Alice", "Bob"}, labels)
		}
	})

	t.Run("select_fills_form", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
			view.WithDeleteOp("test_delete_op"),
		)

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

		var savedRecord *MockRecord
		for _, call := range caller.Calls {
			if call.Op == "test_save_op" {
				if args, ok := call.Args.(model.Encodable); ok {
					w := &recordInspectWriter{}
					args.EncodeFields(w)
					if len(w.recs) > 0 {
						savedRecord = w.recs[0].(*MockRecord)
					}
				}
			}
		}

		if savedRecord == nil {
			t.Errorf("expected a save call with MockRecord payload")
		} else if savedRecord.ID != "2" || savedRecord.Name != "Bob Updated" {
			t.Errorf("expected saved record to be ID '2' (loaded by Select) Name 'Bob Updated' (edited), got ID %q Name %q", savedRecord.ID, savedRecord.Name)
		}
	})

	t.Run("save_ships_form_values", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.SetField("name", "X")
		driver.Save()

		var savedRecord *MockRecord
		for _, call := range caller.Calls {
			if call.Op == "test_save_op" {
				if args, ok := call.Args.(model.Encodable); ok {
					w := &recordInspectWriter{}
					args.EncodeFields(w)
					if len(w.recs) > 0 {
						savedRecord = w.recs[0].(*MockRecord)
					}
				}
			}
		}

		if savedRecord == nil {
			t.Fatalf("expected a save call with MockRecord payload")
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
	// this the same way structurally: tinywasm/form.Form.IsDirty compares
	// live signals against a baseline snapshotted on load; view/mock.Renderer
	// compares its form map against a baseline snapshotted on Select/Deselect.
	t.Run("unchanged_save_does_not_ship", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		driver.Save() // no SetField at all — nothing changed since the load

		for _, call := range caller.Calls {
			if call.Op == "test_save_op" {
				t.Fatalf("expected no save call when nothing changed since Select, got one")
			}
		}
	})

	t.Run("revert_edit_is_not_dirty", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("1")
		driver.SetField("name", "Temporary")
		driver.SetField("name", "Alice") // back to the value Select loaded
		driver.Save()

		for _, call := range caller.Calls {
			if call.Op == "test_save_op" {
				t.Fatalf("expected no save call after editing then reverting to the loaded value, got one")
			}
		}
	})

	t.Run("delete_ships_selected_record", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithDeleteOp("test_delete_op"),
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.Select("2")
		driver.Delete()

		var deletedIDs []string
		for _, call := range caller.Calls {
			if call.Op == "test_delete_op" {
				if args, ok := call.Args.(model.Encodable); ok {
					w := &recordInspectWriter{}
					args.EncodeFields(w)
					deletedIDs = w.ids
				}
			}
		}

		if len(deletedIDs) == 0 {
			t.Errorf("expected a delete call with IDs payload")
		} else if len(deletedIDs) != 1 || deletedIDs[0] != "2" {
			t.Errorf("expected deleted ID to be '2', got %v", deletedIDs)
		}
	})

	t.Run("no_save_capability_when_saveop_empty", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		if _, ok := p.(view.Saver); ok {
			t.Errorf("expected presenter to not implement view.Saver when WithSaveOp is empty")
		}
	})

	t.Run("deselect_clears_selection", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

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
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

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
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice" // description will be "Desc of Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob" // description will be "Desc of Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

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
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithDeleteOp("test_delete_op"),
		)

		d, ok := p.(view.Deleter)
		if !ok {
			t.Fatalf("expected presenter to implement view.Deleter")
		}

		err := d.Delete("unknown")
		if err == nil {
			t.Errorf("expected error deleting unknown id, got nil")
		}

		for _, call := range caller.Calls {
			if call.Op == "test_delete_op" {
				t.Errorf("unexpected delete op call on unknown ID")
			}
		}
	})

	// "+" and ⋮ Editar must move focus to the form's first field so the user
	// can start typing immediately — a standard behavior every renderer must
	// implement identically, not a crudview-specific nicety.
	t.Run("new_focuses_first_field", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		driver := f.New(t, p)
		driver.Mount()
		driver.New()

		if got := driver.FocusedFieldID(); got == "" {
			t.Error("expected the \"+\" action to focus the form's first field, got no focus request")
		}
	})

	t.Run("edit_focuses_first_field", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

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
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

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

	t.Run("no_delete_capability_when_deleteop_empty", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		if _, ok := p.(view.Deleter); ok {
			t.Errorf("expected presenter to not implement view.Deleter when WithDeleteOp is empty")
		}
	})

	t.Run("no_update_capability_when_updateop_empty", func(t *testing.T) {
		caller := &FakeCaller{}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
		)

		if _, ok := p.(view.Updater); ok {
			t.Errorf("expected presenter to not implement view.Updater when WithUpdateOp is empty")
		}
	})

	t.Run("plural_save_delete_and_update", func(t *testing.T) {
		caller := &FakeCaller{
			Reply: func(op string, into model.Decodable) {
				if op == "test_list_op" {
					l := into.(*MockList)
					a := l.Append().(*MockRecord)
					a.ID, a.Name = "1", "Alice"
					b := l.Append().(*MockRecord)
					b.ID, b.Name = "2", "Bob"
				}
			},
		}
		record := &MockRecord{}
		p := view.New(
			caller,
			record,
			"test_list_op",
			func() model.ModelSlice { return &MockList{} },
			view.WithSaveOp("test_save_op"),
			view.WithUpdateOp("test_update_op"),
			view.WithDeleteOp("test_delete_op"),
		)

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

		// Plural Delete
		if err := d.Delete("1", "2"); err != nil {
			t.Fatalf("plural Delete failed: %v", err)
		}

		// Plural Update
		patch := &MockRecord{Name: "Patched"}
		if err := u.Update([]string{"1", "2"}, patch, []string{"name"}); err != nil {
			t.Fatalf("plural Update failed: %v", err)
		}

		var saveCalls, updateCalls, deleteCalls int
		for _, call := range caller.Calls {
			switch call.Op {
			case "test_save_op":
				saveCalls++
				w := &recordInspectWriter{}
				call.Args.EncodeFields(w)
				if len(w.recs) != 2 {
					t.Errorf("expected 2 saved records in 1 call, got %d", len(w.recs))
				}
			case "test_update_op":
				updateCalls++
				w := &recordInspectWriter{}
				call.Args.EncodeFields(w)
				if len(w.ids) != 2 || len(w.fields) != 1 || w.fields[0] != "name" {
					t.Errorf("expected 2 ids and 1 field in update call, got ids=%v fields=%v", w.ids, w.fields)
				}
			case "test_delete_op":
				deleteCalls++
				w := &recordInspectWriter{}
				call.Args.EncodeFields(w)
				if len(w.ids) != 2 {
					t.Errorf("expected 2 deleted ids in 1 call, got %d", len(w.ids))
				}
			}
		}

		if saveCalls != 1 || updateCalls != 1 || deleteCalls != 1 {
			t.Errorf("expected exactly 1 call each for save, update, delete; got save=%d update=%d delete=%d", saveCalls, updateCalls, deleteCalls)
		}
	})
}

type recordInspectWriter struct {
	recs   []model.Encodable
	ids    []string
	fields []string
	rec    model.Encodable
}

func (w *recordInspectWriter) String(name, val string)        {}
func (w *recordInspectWriter) Int(name string, val int64)     {}
func (w *recordInspectWriter) Float(name string, val float64) {}
func (w *recordInspectWriter) Bool(name string, val bool)     {}
func (w *recordInspectWriter) Bytes(name string, val []byte) {}
func (w *recordInspectWriter) Null(name string)               {}
func (w *recordInspectWriter) Raw(name, val string)           {}
func (w *recordInspectWriter) Object(name string, val model.Encodable) {
	if name == "record" {
		w.rec = val
	}
}
func (w *recordInspectWriter) Array(name string, n int) model.ArrayWriter {
	return &inspectArrayWriter{parent: w, name: name}
}

type inspectArrayWriter struct {
	parent *recordInspectWriter
	name   string
}

func (a *inspectArrayWriter) String(val string) {
	switch a.name {
	case "ids":
		a.parent.ids = append(a.parent.ids, val)
	case "fields":
		a.parent.fields = append(a.parent.fields, val)
	}
}
func (a *inspectArrayWriter) Int(val int64)        {}
func (a *inspectArrayWriter) Float(val float64)    {}
func (a *inspectArrayWriter) Bool(val bool)        {}
func (a *inspectArrayWriter) Bytes(val []byte)     {}
func (a *inspectArrayWriter) Object(val model.Encodable) {
	if a.name == "records" {
		a.parent.recs = append(a.parent.recs, val)
	}
}
func (a *inspectArrayWriter) Close() {}
