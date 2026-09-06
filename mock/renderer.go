package mock

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/view"
)

// Renderer is a headless reference renderer for browser-less simulation and tests.
type Renderer struct {
	p        view.Presenter
	form     map[string]string
	baseline map[string]string // last loaded/reset value per field — see isDirty
	focused  string            // field name New()/Edit() last targeted (see FocusedFieldID)
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

// Mount triggers loading data into the presenter synchronously.
func (r *Renderer) Mount() {
	_ = r.p.Reload()
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
	}
	r.rebaseline() // a freshly selected/loaded record is pristine — see isDirty
}

// Deselect clears the selection and form.
func (r *Renderer) Deselect() {
	r.p.Deselect()
	r.form = make(map[string]string)
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

// Cancel simulates the "↺" (undo) action: abandons the draft/selection and
// clears the tracked focus — nothing must be left focused after a cancel.
func (r *Renderer) Cancel() {
	r.Deselect()
	r.focused = ""
}

// Filter filters the presenter items and returns their labels.
func (r *Renderer) Filter(term string) []string {
	items := r.p.Filter(term)
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
	}
	_ = s.Save(rec)
	r.rebaseline() // saved successfully — a later untouched commit isn't dirty again
}

// Delete deletes the selected record.
func (r *Renderer) Delete() {
	d, ok := r.p.(view.Deleter)
	if !ok {
		return
	}
	_ = d.Delete(r.p.Selected())
}
