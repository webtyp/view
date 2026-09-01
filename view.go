package view

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
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
	Reload() error             // synchronously calls ListOp, decodes, projects and indexes

	Selected() string             // currently selected id ("" if none)
	Select(id string) model.Model // marks id and returns its record from the internal index; unknown id → nil, selection unchanged
	Deselect()                    // clears the selection
}

// Saver represents the capability to save a record.
// The renderer discovers it by type assertion at the seam.
type Saver interface {
	Save(payload model.Model) error // synchronously ships the explicit payload to SaveOp
}

// Deleter represents the capability to delete a record.
// The renderer discovers it by type assertion at the seam.
type Deleter interface {
	Delete(id string) error // ships the indexed record of id to DeleteOp; unknown id → error
}

type config struct {
	title             string
	searchPlaceholder string
	saveOp            string
	deleteOp          string
	args              func() model.Encodable
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

// WithSaveOp sets the save operation.
func WithSaveOp(op string) Option {
	return func(c *config) {
		c.saveOp = op
	}
}

// WithDeleteOp sets the delete operation.
func WithDeleteOp(op string) Option {
	return func(c *config) {
		c.deleteOp = op
	}
}

// WithArgs sets the function to retrieve arguments for the list operation.
func WithArgs(args func() model.Encodable) Option {
	return func(c *config) {
		c.args = args
	}
}

// New builds the presenter. Mandatory collaborators are positional;
// a nil/empty mandatory value panics at construction — a loud development diagnostic.
func New(
	caller router.Caller,
	record model.Model,
	listOp string,
	newList func() model.ModelSlice,
	opts ...Option,
) Presenter {
	if caller == nil {
		panic("view: New: caller is required")
	}
	if model.IsNil(record) {
		panic("view: New: record is required")
	}
	if listOp == "" {
		panic("view: New: listOp is required")
	}
	if newList == nil {
		panic("view: New: newList is required")
	}

	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	c := &core{
		caller:            caller,
		record:            record,
		listOp:            listOp,
		newList:           newList,
		title:             cfg.title,
		searchPlaceholder: cfg.searchPlaceholder,
		args:              cfg.args,
		saveOp:            cfg.saveOp,
		deleteOp:          cfg.deleteOp,
		index:             make(map[string]model.Model),
	}

	switch {
	case cfg.saveOp != "" && cfg.deleteOp != "":
		return &crud{core: c} // Presenter + Saver + Deleter
	case cfg.saveOp != "":
		return &saveable{core: c} // Presenter + Saver
	case cfg.deleteOp != "":
		return &deletable{core: c} // Presenter + Deleter
	default:
		return c // solo Presenter
	}
}
