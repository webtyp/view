package mock

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

// Renderer is a headless reference renderer for browser-less simulation and tests.
type Renderer struct {
	p           view.Presenter
	form        map[string]string
	baseline    map[string]string // last loaded/reset value per field — see isDirty
	focused     string            // field name New()/Edit() last targeted (see FocusedFieldID)
	loadedID    string            // id of the record Select loaded ("": a new-record draft) — see Save
	nextID      int               // minted ids for new-record drafts — see Save
	openConfirm router.Operation  // op of the currently open confirmation dialog (zero = none)
}

// New creates a reference renderer instance for a given Presenter.
func New(p view.Presenter) *Renderer {
	return &Renderer{
		p:        p,
		form:     make(map[string]string),
		baseline: make(map[string]string),
	}
}

// Presenter returns the underlying presenter.
func (r *Renderer) Presenter() view.Presenter {
	return r.p
}

// Mount triggers loading data into the presenter. Load is asynchronous: a
// lister may deliver its rows in a later turn of the event loop, so callers
// must not assume Labels() is populated right after Mount — they must paint
// from inside the Reload callback.
func (r *Renderer) Mount() {
	r.p.Reload(func(error) {})
}

// Labels returns labels of the loaded items.
func (r *Renderer) Labels() []string {
	items := r.p.Items()
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}

// Select marks id as selected, and populates r.form with the record's schema fields.
func (r *Renderer) Select(id string) {
	m := r.p.Select(id)
	r.form = make(map[string]string)
	r.loadedID = "" // unknown until the row below proves otherwise
	if !model.IsNil(m) {
		schema := m.Schema()
		pointers := m.Pointers()
		for i, f := range schema {
			ptr := pointers[i]
			var val string
			switch p := ptr.(type) {
			case *string:
				val = *p
			case *int64:
				val = fmt.Sprintf("%d", *p)
			case *float64:
				val = fmt.Sprintf("%f", *p)
			case *bool:
				val = fmt.Sprintf("%t", *p)
			}
			r.form[f.Name] = val
		}
		r.loadedID = id // what was loaded defines the identity — see Save
	}
	r.rebaseline() // a freshly selected/loaded record is pristine — see isDirty
}

// Deselect clears the selection and form.
func (r *Renderer) Deselect() {
	r.p.Deselect()
	r.form = make(map[string]string)
	r.loadedID = "" // abandoning the draft/selection drops its identity too
	r.rebaseline()
}

// rebaseline snapshots the baseline to the CURRENT form values — the
// reference implementation of the "form isn't dirty against what was just
// loaded/saved" contract, mirroring webtyp/form's Form.MarkPristine.
func (r *Renderer) rebaseline() {
	r.baseline = make(map[string]string, len(r.form))
	for k, v := range r.form {
		r.baseline[k] = v
	}
}

// isDirty reports whether any field differs from the baseline captured at
// the last Select/Deselect/Save — the same "did the user actually change
// anything" question webtyp/form.Form.IsDirty answers for a real renderer.
func (r *Renderer) isDirty() bool {
	if len(r.form) != len(r.baseline) {
		return true
	}
	for k, v := range r.form {
		if r.baseline[k] != v {
			return true
		}
	}
	return false
}

// firstFieldName returns the name of the record's first schema field, or ""
// if it has none — the "first field" every renderer focuses on New/Edit.
func (r *Renderer) firstFieldName() string {
	schema := r.p.Record().Schema()
	if len(schema) == 0 {
		return ""
	}
	return schema[0].Name
}

// New simulates the "+" (create new) action: clears the selection/form and
// focuses the first field, same as a real renderer would.
func (r *Renderer) New() {
	r.Deselect()
	r.focused = r.firstFieldName()
}

// Edit simulates ⋮ → Editar: selects id then focuses the first field.
func (r *Renderer) Edit(id string) {
	r.Select(id)
	r.focused = r.firstFieldName()
}

// FocusedFieldID returns the field name New()/Edit() last targeted.
func (r *Renderer) FocusedFieldID() string { return r.focused }

// ActionLabels returns the labels of the action controls, in order.
func (r *Renderer) ActionLabels() []string {
	if a, ok := r.p.(view.Actioner); ok {
		actions := a.Actions()
		labels := make([]string, len(actions))
		for i, act := range actions {
			labels[i] = string(act.Label)
		}
		return labels
	}
	return nil
}

// ActionEnabled returns true if the action control is enabled.
func (r *Renderer) ActionEnabled(op router.Operation) bool {
	// Disabled when presenter has no items
	return len(r.p.Items()) > 0
}

// ClickAction clicks the action control.
func (r *Renderer) ClickAction(op router.Operation) {
	if a, ok := r.p.(view.Actioner); ok {
		actions := a.Actions()
		for _, act := range actions {
			if act.Operation == op {
				if string(act.Confirm) != "" {
					r.openConfirm = op
				} else {
					a.Run(op, func(error) {})
				}
				return
			}
		}
	}
}

// ConfirmAction accepts the open confirmation; no-op when none is open.
func (r *Renderer) ConfirmAction() {
	if r.openConfirm != (router.Operation{}) {
		if a, ok := r.p.(view.Actioner); ok {
			a.Run(r.openConfirm, func(error) {})
		}
		r.openConfirm = router.Operation{}
	}
}

// Cancel simulates the "↺" (undo) action: abandons the draft/selection and
// clears the tracked focus — nothing must be left focused after a cancel.
func (r *Renderer) Cancel() {
	r.Deselect()
	r.focused = ""
}

// Search asks the presenter to search term and returns the labels of the rows it lists.
func (r *Renderer) Search(term string) []string {
	r.p.Search(term, nil)
	items := r.p.Items()
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}

// SetField sets a form field value.
func (r *Renderer) SetField(name, value string) {
	r.form[name] = value
}

// Save synchronizes the headless form values with the Record, and calls Save
// — but only when something actually changed since the last load/save (see
// isDirty). A commit that touches nothing (e.g. focus moved through a field
// and back) must never reach the Saver: it isn't a save, it's a no-op.
func (r *Renderer) Save() {
	s, ok := r.p.(view.Saver)
	if !ok {
		return
	}
	if !r.isDirty() {
		return
	}
	rec := r.p.Record()
	if !model.IsNil(rec) {
		schema := rec.Schema()
		pointers := rec.Pointers()
		for i, f := range schema {
			if f.Name == "id" {
				continue // the hidden PK travels via loadedID below, never the form map
			}
			if val, ok := r.form[f.Name]; ok {
				ptr := pointers[i]
				switch p := ptr.(type) {
				case *string:
					*p = val
				case *int64:
					var v int64
					_, _ = fmt.Sscanf(val, "%d", &v)
					*p = v
				case *float64:
					var v float64
					_, _ = fmt.Sscanf(val, "%f", &v)
					*p = v
				case *bool:
					if val == "true" || val == "1" {
						*p = true
					} else {
						*p = false
					}
				}
			}
		}
		// The hidden PK is renderer state: Select remembered it, Deselect
		// cleared it. The shared Record's own id is never read — trusting
		// it wrote edits onto whichever record was synced last. A draft
		// with no loaded id mints one and keeps it across retries until
		// the next Deselect.
		if r.loadedID == "" {
			r.nextID++
			r.loadedID = fmt.Sprintf("mock-%d", r.nextID)
			r.form["id"] = r.loadedID
		}
		for i, f := range schema {
			if f.Name == "id" {
				if p, ok := pointers[i].(*string); ok {
					*p = r.loadedID
				}
				break
			}
		}
	}
	s.Save([]model.Model{rec}, func(error) {})
	r.rebaseline() // saved successfully — a later untouched commit isn't dirty again
}

// Delete deletes the selected record.
func (r *Renderer) Delete() {
	d, ok := r.p.(view.Deleter)
	if !ok {
		return
	}
	d.Delete([]string{r.p.Selected()}, func(error) {})
}
