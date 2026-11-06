package view

import "webtyp.com/model"

// Page is how much of a list the module loads, declared explicitly by the module because only it knows what its
// screen needs:
//
//	view.Page{First: 0, Size: 20, More: true}  // patients: nothing on open, each search brings 20, scrolling the next 20
//	view.Page{First: view.All}                 // staff: the administrator needs every row (bounded by MaxPage)
//
// First is how many rows the screen loads when it opens (0 = it waits for a search; All = every row up to MaxPage).
// Size is how many rows each search and each scroll step bring. More says whether reaching the end of the list asks
// the server for the next rows, until a page arrives shorter than Size ("no more results for this search").
//
// A Lister without a declared Page (Size 0) gets DefaultPage. The server enforces MaxPage whatever the browser asks
// (Cap), and the search runs on the server: with pages, the browser never holds every row, so searching there would
// miss a record that exists and invite a duplicate.
type Page struct {
	First int
	Size  int
	More  bool
}

// All, as Page.First, loads every row the server gives up to MaxPage.
const All = -1

// MaxPage is the most rows one list call returns; the server caps every request to it (Cap).
const MaxPage = 500

// DefaultPage is the Page of a Lister that declares none: 50 rows on open, 50 more per scroll step.
var DefaultPage = Page{First: 50, Size: 50, More: true}

// declared returns p, or DefaultPage when p declares nothing (Size 0).
func (p Page) declared() Page {
	if p.Size == 0 && p.First != All {
		return DefaultPage
	}
	return p
}

// Cap is the limit a list operation must use: the asked limit, or MaxPage when it is missing or larger.
//
// Example: rows, err := m.listPatients(m.tenantID, args.Search, args.After, view.Cap(args.Limit))
func Cap(limit int64) int64 {
	if limit <= 0 || limit > MaxPage {
		return MaxPage
	}
	return limit
}

// ListArgs is what the view sends to a list operation: the search term ("" = no search), the id of the last row
// already shown ("" = from the start; the server returns the rows after it in its own stable order) and how many
// rows to return. A list operation declares it with .Accepts(&view.ListArgs{}) and decodes it with ctx.Decode.
//
// Example: var args view.ListArgs; if err := ctx.Decode(&args); err != nil { … }
type ListArgs struct {
	Search string
	After  string
	Limit  int64
}

// ListArgsModel is the wire schema of ListArgs.
var ListArgsModel = model.Definition{
	Name: "list_args",
	Fields: model.Fields{
		{Name: "search", Type: model.Text()},
		{Name: "after", Type: model.Text()},
		{Name: "limit", Type: model.Int()},
	},
}

func (a *ListArgs) Schema() []model.Field { return ListArgsModel.Fields }
func (a *ListArgs) Pointers() []any       { return []any{&a.Search, &a.After, &a.Limit} }
func (a *ListArgs) IsNil() bool           { return a == nil }

func (a *ListArgs) EncodeFields(w model.FieldWriter) {
	w.String("search", a.Search)
	w.String("after", a.After)
	w.Int("limit", a.Limit)
}

func (a *ListArgs) DecodeFields(r model.FieldReader) {
	a.Search, _ = r.String("search")
	a.After, _ = r.String("after")
	a.Limit, _ = r.Int("limit")
}
