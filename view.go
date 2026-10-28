package view

import (
	"webtyp.com/lang"
	"webtyp.com/model"
)

// Record is a record a list can show: a model whose Item() ormc generates from the
// fields marked Item:… (model.ItemLabel, …). A model without marks has no Item()
// and cannot be listed — the compiler says so.
type Record interface {
	model.Model
	model.Itemizer
}

// Presenter is the UI-agnostic core behind any CRUD view: list, select, reload.
// Always present. Save/Delete are separate capabilities (see Saver/Deleter).
type Presenter interface {
	Title() string
	SearchPlaceholder() lang.Text

	// Description is what this view is for, shown when no record is selected. "" when WithDescription was not used.
	Description() lang.Text

	// EmptyMessage is what to show, and what to do, when Items() is empty. "" when WithEmpty was not used.
	EmptyMessage() lang.Text
	Record() Record

	Items() []model.Item             // the rows of this presenter's model, newest first
	Filter(term string) []model.Item // local case-insensitive match over Label+Description; "" returns all

	// Reload asks the Lister for the records, then projects and indexes them.
	// The result is asynchronous: done runs once, after List delivers, and the
	// renderer paints Items() from inside it. done is always safe to call even
	// if the caller passes nil.
	Reload(done func(error))

	Selected() string        // currently selected id ("" if none)
	Select(id string) Record // marks id and returns its record from the internal index; unknown id → nil, selection unchanged
	Deselect()               // clears the selection

	// OnChange registers fn to run after any record of this presenter's model
	// changes in the shared Records (a save from any screen, a sync). A renderer
	// re-reads Items() inside fn.
	OnChange(fn func())
}

// Saver creates or replaces WHOLE records. A slice, not a single record: the
// one-record case is a slice of one, so there is exactly one write path to
// implement, test and reason about. This is what the "+" new-record flow and
// the edit-one-record form use.
//
// The outcome arrives asynchronously through done, which is always non-nil:
// Save never blocks and returns no error. Every failure — including the
// programming errors view validates before reaching the lister — travels
// through done, so there is exactly one way to report the result.
//
// Implemented by a Lister to declare the capability, and re-exposed by the
// Presenter view.New returns when the backend has it.
type Saver interface {
	Save(recs []model.Model, done func(error))
}

// Updater patches ONLY the named columns across every id, in a single
// statement. It is deliberately NOT Save with a partial record: sending whole
// records to change one column reverts, in silence, every other column that
// someone else changed since this client last reloaded — the classic lost
// update. Naming the columns makes that impossible: what is not in fields is
// never written.
//
// rec carries the values (it is the form's own record, already validated);
// fields names the columns to write. Values stay inside the generated typed
// struct — no name→value bag, no `any`, no reflection.
//
// fields holds Schema() field names and pairs directly with
// form.DirtyFields(). An empty fields slice is an error, not a no-op.
//
// The outcome arrives asynchronously through done, which is always non-nil:
// Update never blocks and returns no error. Every failure travels through
// done, so there is exactly one way to report the result.
//
// Implemented by a Lister to declare the capability, and re-exposed by the
// Presenter view.New returns when the backend has it.
type Updater interface {
	Update(ids []string, rec model.Model, fields []string, done func(error))
}

// Deleter removes records. A slice for the same reason as Saver: the
// one-record case is a slice of one.
//
// The outcome arrives asynchronously through done, which is always non-nil:
// Delete never blocks and returns no error. Every failure travels through
// done, so there is exactly one way to report the result.
//
// Implemented by a Lister to declare the capability, and re-exposed by the
// Presenter view.New returns when the backend has it.
type Deleter interface {
	Delete(ids []string, done func(error))
}

type config struct {
	title             string
	searchPlaceholder lang.Text
	description       lang.Text
	emptyMessage      lang.Text
}

// Option is a functional configuration option for New.
type Option func(*config)

// WithTitle sets the title of the view.
func WithTitle(title string) Option {
	return func(c *config) {
		c.title = title
	}
}

// WithSearchPlaceholder sets the search placeholder of the view.
func WithSearchPlaceholder(placeholder lang.Text) Option {
	return func(c *config) {
		c.searchPlaceholder = placeholder
	}
}

// WithDescription sets what this view is for, shown when no record is selected.
func WithDescription(d lang.Text) Option {
	return func(c *config) { c.description = d }
}

// WithEmpty sets what to show, and what to do, when the list has no records yet.
func WithEmpty(msg lang.Text) Option {
	return func(c *config) { c.emptyMessage = msg }
}

// New builds the presenter over a Lister. Mandatory collaborators are
// positional; a nil/empty mandatory value panics — a loud development
// diagnostic.
//
// The Presenter's capabilities MIRROR the lister's: it is a Saver iff l
// implements Saver, and so on. There is no configuration that can claim
// a capability the lister does not have.
func New(
	l Lister,
	record Record,
	opts ...Option,
) Presenter {
	if l == nil {
		panic("view: New: lister is required")
	}
	if model.IsNil(record) {
		panic("view: New: record is required")
	}

	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	c := &core{
		lister:            l,
		record:            record,
		title:             cfg.title,
		searchPlaceholder: cfg.searchPlaceholder,
		description:       cfg.description,
		emptyMessage:      cfg.emptyMessage,
	}

	_, hasS := l.(Saver)
	_, hasU := l.(Updater)
	_, hasD := l.(Deleter)

	switch {
	case hasS && hasU && hasD:
		return &crud{core: c}
	case hasS && hasU:
		return &saveableUpdatable{core: c}
	case hasS && hasD:
		return &saveableDeletable{core: c}
	case hasU && hasD:
		return &updatableDeletable{core: c}
	case hasS:
		return &saveable{core: c}
	case hasU:
		return &updatable{core: c}
	case hasD:
		return &deletable{core: c}
	default:
		return c
	}
}
