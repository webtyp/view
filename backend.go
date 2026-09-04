package view

import "github.com/tinywasm/model"

// Backend is what a view needs from the application: the records to show.
// Mandatory — a view with nothing to list is not a view.
//
// Writing is optional and declared by implementing BackendSaver / BackendUpdater
// / BackendDeleter. view.New asserts each one and returns a Presenter carrying
// exactly the matching capabilities, so a renderer never paints a control the
// application cannot run. A missing method is a compile-time fact, not a
// configuration string that can be misspelled.
type Backend interface {
	// List returns every record, newest-first or in whatever order the
	// application considers natural. view projects them through Itemizer.
	List() ([]model.Model, error)
}

// BackendSaver creates or replaces WHOLE records. Slice, not variadic: it is
// always a batch, N=1 included, so there is one write path to implement.
type BackendSaver interface {
	Save(recs []model.Model) error
}

// BackendUpdater patches ONLY the named columns across every id, in a single
// statement. rec carries the values; fields names the columns. See the Updater
// doc in view.go for why this is not Save with a partial record.
type BackendUpdater interface {
	Update(ids []string, rec model.Model, fields []string) error
}

// BackendDeleter removes records by id.
type BackendDeleter interface {
	Delete(ids []string) error
}
