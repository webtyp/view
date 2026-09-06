package view

import (
	"webtyp.com/model"
)

// Item is ONE projected row of the list — the neutral form any renderer can draw.
// No markup: only what a list needs to display and let the user pick a record.
type Item struct {
	ID          string // selection key
	Label       string // primary text
	Description string // secondary text (a SKU, an IP, a subtitle)

	// LeadTop/LeadMain/LeadBottom are an OPTIONAL three-line badge for the row's
	// leading slot — position only, no assumed content: a renderer that draws a
	// plain row (targetlist) ignores them; one built around a prominent leading
	// badge (targetdate) reads them instead of Label for that slot. What goes in
	// them — a date, an hour, anything else — is the consumer's call, not this
	// type's. Empty means "no badge" — Filter (below) never looks at these, so
	// leaving them blank never breaks search.
	LeadTop    string
	LeadMain   string
	LeadBottom string
}

// Itemizer is implemented by a domain record that knows how to project itself as a
// list row. It is the ONLY view-specific code a module writes on its model.
type Itemizer interface {
	Item() Item
}

// Presenter is the UI-agnostic core behind any CRUD view: list, select, reload.
// Always present. Save/Delete are separate capabilities (see Saver/Deleter).
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item             // projected list from the last Reload
	Filter(term string) []Item // local case-insensitive match over Label+Description; "" returns all
	Reload() error             // asks the Lister for the records, then projects and indexes them

	Selected() string             // currently selected id ("" if none)
	Select(id string) model.Model // marks id and returns its record from the internal index; unknown id → nil, selection unchanged
	Deselect()                    // clears the selection
}

// Saver creates or replaces WHOLE records. Variadic, not single: the one-record
// case is N=1, so there is exactly one write path to implement, test and reason
// about. This is what the "+" new-record flow and the edit-one-record form use.
//
// Implemented by a Lister to declare the capability, and re-exposed by the
// Presenter view.New returns when the backend has it.
type Saver interface {
	Save(recs ...model.Model) error
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
// Implemented by a Lister to declare the capability, and re-exposed by the
// Presenter view.New returns when the backend has it.
type Updater interface {
	Update(ids []string, rec model.Model, fields []string) error
}

// Deleter removes records. Variadic for the same reason as Saver.
//
// Implemented by a Lister to declare the capability, and re-exposed by the
// Presenter view.New returns when the backend has it.
type Deleter interface {
	Delete(ids ...string) error
}

type config struct {
	title             string
	searchPlaceholder string
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
func WithSearchPlaceholder(placeholder string) Option {
	return func(c *config) {
		c.searchPlaceholder = placeholder
	}
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
	record model.Model,
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
