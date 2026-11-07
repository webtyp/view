package view

// Page is how much of a list the module loads, declared explicitly by the module because only it knows what its
// screen needs:
//
//	view.Page{First: 0, Size: 20, More: true}  // patients: nothing on open, each search brings 20, scrolling the next 20
//	view.Page{First: view.All}                 // staff: the administrator needs every row (bounded by crud.MaxPage)
//
// First is how many rows the screen loads when it opens (0 = it waits for a search; All = every row up to crud.MaxPage).
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

// All, as Page.First, loads every row the server gives up to crud.MaxPage.
const All = -1

// DefaultPage is the Page of a Lister that declares none: 50 rows on open, 50 more per scroll step.
var DefaultPage = Page{First: 50, Size: 50, More: true}

// declared returns p, or DefaultPage when p declares nothing (Size 0).
func (p Page) declared() Page {
	if p.Size == 0 && p.First != All {
		return DefaultPage
	}
	return p
}
