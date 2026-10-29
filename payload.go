package view

import (
	"webtyp.com/model"
)

// saveArgs carries N whole records. It is a wire shape, written and read
// only by caller_lister.go; nothing else in the ecosystem should need to
// know it: MountSave/MountDelete read them on the server.
type saveArgs struct {
	recs []model.Model
}

func (a *saveArgs) IsNil() bool { return a == nil }

func (a *saveArgs) EncodeFields(w model.FieldWriter) {
	arr := w.Array("records", len(a.recs))
	for _, r := range a.recs {
		arr.Object(r)
	}
}

// updateArgs is the wire shape of a field patch: which rows, which columns,
// and a record carrying the values. It is a wire shape, written and read
// only by caller_lister.go; nothing else in the ecosystem should need to
// know it: MountSave/MountDelete read them on the server.
type updateArgs struct {
	ids    []string
	fields []string
	rec    model.Model
}

func (a *updateArgs) IsNil() bool { return a == nil }

func (a *updateArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array("ids", len(a.ids))
	for _, id := range a.ids {
		ids.String(id)
	}
	fields := w.Array("fields", len(a.fields))
	for _, f := range a.fields {
		fields.String(f)
	}
	w.Object("record", a.rec)
}

// deleteArgs carries N ids to remove. It is a wire shape, written and read
// only by caller_lister.go; nothing else in the ecosystem should need to
// know it: MountSave/MountDelete read them on the server.
type deleteArgs struct {
	ids []string
}

func (a *deleteArgs) IsNil() bool { return a == nil }

func (a *deleteArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array("ids", len(a.ids))
	for _, id := range a.ids {
		ids.String(id)
	}
}

const (
	ErrNoRecords  model.Invalid = "view: no records to save"
	ErrNoIDs      model.Invalid = "view: no ids given"
	ErrCannotRead model.Invalid = "view: cannot read the request"
)

func (s *saveIn[T, PT]) IsNil() bool { return s == nil }

type saveIn[T any, PT interface {
	*T
	model.Model
}] struct {
	recs []PT
}

func (s *saveIn[T, PT]) DecodeFields(r model.FieldReader) {
	if arr, ok := r.Array("records"); ok {
		n := arr.Len()
		s.recs = make([]PT, 0, n)
		for i := 0; i < n; i++ {
			p := PT(new(T))
			if arr.Object(i, p) {
				s.recs = append(s.recs, p)
			}
		}
	}
}

func (d *deleteIn) IsNil() bool { return d == nil }

type deleteIn struct {
	ids []string
}

func (d *deleteIn) DecodeFields(r model.FieldReader) {
	if arr, ok := r.Array("ids"); ok {
		n := arr.Len()
		d.ids = make([]string, 0, n)
		for i := 0; i < n; i++ {
			d.ids = append(d.ids, arr.String(i))
		}
	}
}
