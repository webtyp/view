package view

import (
	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/model"
)

// errUnknownActionPrefix is the exact message of SPECS §9 for an op no action declares.
const errUnknownActionPrefix = "view: Run: unknown action "

func errUnknownAction(op string) error { return fmt.Err(errUnknownActionPrefix + op) }

// Action is a command an operator runs on the list as a whole.
// It is not an edit of a record: it has no form, it is usually not undoable,
// and its arguments come from the data on screen.
type Action struct {
	Op      string    // bare op name; qualified with Ops.Module exactly like List/Save
	Label   lang.Text // button text (dictionary key)
	Confirm lang.Text // question asked before running; "" = runs on click
	// Args builds the op's arguments from the records of the last Reload.
	// nil = the op takes no arguments.
	Args func(records []model.Model) model.Encodable
}

// Actioner is implemented by EVERY Presenter view.New returns.
type Actioner interface {
	Actions() []Action               // the backend's actions; empty when it offers none
	Run(op string, done func(error)) // see behaviour below
}

// ActionRunner is the Lister-side half, implemented by the Lister NewCallerLister
// returns (and by any hand-written Lister that offers actions).
type ActionRunner interface {
	Actions() []Action
	RunAction(op string, args model.Encodable, done func(error))
}

// Compile-time checks that every presenter variant implements Actioner.
var (
	_ Actioner = (*core)(nil)
	_ Actioner = (*saveable)(nil)
	_ Actioner = (*updatable)(nil)
	_ Actioner = (*deletable)(nil)
	_ Actioner = (*saveableUpdatable)(nil)
	_ Actioner = (*saveableDeletable)(nil)
	_ Actioner = (*updatableDeletable)(nil)
	_ Actioner = (*crudPresenter)(nil)
)
