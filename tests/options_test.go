package tests

import (
	"testing"

	"webtyp.com/lang"
	"webtyp.com/model"
	"webtyp.com/view"
)

type plainLister struct{
	records *view.Records
}

func (l *plainLister) Records() *view.Records {
	if l.records == nil {
		l.records = view.NewRecords()
	}
	return l.records
}

func (l *plainLister) List(done func([]model.Model, error)) {
	done(nil, nil)
}

type crudLister struct{
	records *view.Records
}

func (l *crudLister) Records() *view.Records {
	if l.records == nil {
		l.records = view.NewRecords()
	}
	return l.records
}

func (l *crudLister) List(done func([]model.Model, error)) {
	done(nil, nil)
}

func (l *crudLister) Save(recs []model.Model, done func(error)) {
	done(nil)
}

func (l *crudLister) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	done(nil)
}

func (l *crudLister) Delete(ids []string, done func(error)) {
	done(nil)
}

type dummyRecordForOptions struct{}

func (r *dummyRecordForOptions) ID() string                        { return "" }
func (r *dummyRecordForOptions) Schema() []model.Field             { return nil }
func (r *dummyRecordForOptions) Pointers() []any                   { return nil }
func (r *dummyRecordForOptions) EncodeFields(w model.FieldWriter)  {}
func (r *dummyRecordForOptions) DecodeFields(r2 model.FieldReader) {}
func (r *dummyRecordForOptions) Item() view.Item                   { return view.Item{} }
func (r *dummyRecordForOptions) IsNil() bool                       { return r == nil }
func (r *dummyRecordForOptions) ModelName() string                 { return "dummy" }

func TestWithDescriptionAndWithEmptyUnset(t *testing.T) {
	p := view.New(&plainLister{}, &dummyRecordForOptions{})
	if p.Description() != "" {
		t.Errorf("expected Description to be empty, got %q", p.Description())
	}
	if p.EmptyMessage() != "" {
		t.Errorf("expected EmptyMessage to be empty, got %q", p.EmptyMessage())
	}
}

func TestWithDescriptionAndWithEmptySetPlain(t *testing.T) {
	desc := lang.Text("My custom description")
	empty := lang.Text("No items found. Create one!")

	p := view.New(&plainLister{}, &dummyRecordForOptions{}, view.WithDescription(desc), view.WithEmpty(empty))
	if p.Description() != desc {
		t.Errorf("expected Description %q, got %q", desc, p.Description())
	}
	if p.EmptyMessage() != empty {
		t.Errorf("expected EmptyMessage %q, got %q", empty, p.EmptyMessage())
	}
}

func TestWithDescriptionAndWithEmptySetCrud(t *testing.T) {
	desc := lang.Text("My crud description")
	empty := lang.Text("Nothing here, add some!")

	p := view.New(&crudLister{}, &dummyRecordForOptions{}, view.WithDescription(desc), view.WithEmpty(empty))
	if p.Description() != desc {
		t.Errorf("expected Description %q, got %q", desc, p.Description())
	}
	if p.EmptyMessage() != empty {
		t.Errorf("expected EmptyMessage %q, got %q", empty, p.EmptyMessage())
	}

	// Verify capability mirroring to make sure it's a wrapper type.
	if _, ok := p.(view.Saver); !ok {
		t.Errorf("expected presenter to be a view.Saver")
	}
	if _, ok := p.(view.Updater); !ok {
		t.Errorf("expected presenter to be a view.Updater")
	}
	if _, ok := p.(view.Deleter); !ok {
		t.Errorf("expected presenter to be a view.Deleter")
	}
}
