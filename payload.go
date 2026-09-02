package view

import (
	"github.com/tinywasm/model"
)

// saveArgs carries N whole records.
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
// and a record carrying the values.
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

// deleteArgs carries N ids to remove.
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
