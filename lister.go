package view

import "github.com/tinywasm/model"

// Lister is what a view needs from the application: the records to show.
// Mandatory — a view with nothing to list is not a view.
//
// Writing is optional and declared by implementing the SAME capability
// interfaces a renderer asserts on the Presenter: Saver, Updater, Deleter (see
// view.go). There is deliberately no separate backend-only twin — "this
// backend can save" and "this presenter can save" are one predicate, and
// view.New mirrors the backend's set onto the Presenter it returns. A missing
// method is a compile-time fact, not a configuration string that can be
// misspelled.
type Lister interface {
	// List returns every record, newest-first or in whatever order the
	// application considers natural. view projects them through Itemizer.
	List() ([]model.Model, error)
}

// --- The capability-wrapper pattern -----------------------------------------
//
// The capability wrappers in presenter.go and caller_lister.go are thin on
// purpose: every method delegates to the matching core method, which is the
// ONE place each operation is written. What varies between them is only the
// method SET, because that is what a consumer's `if u, ok :=
// p.(view.Updater); ok` reads.
//
// The count is 2^n-1 for n optional capabilities: 3 types for two, 7 for three,
// and 15 if a fourth is ever added. That is the price of discovering
// capabilities by type assertion rather than by a runtime `Can(...)` check —
// and it is the right price here, because the assertion is what lets a
// renderer decide at wiring time not to paint a control it could never run.
// A fourth capability is the moment to stop and reconsider, not to type out
// eight more structs.
