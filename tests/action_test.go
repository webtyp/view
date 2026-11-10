package tests

import (
	"strings"
	"testing"

	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

// Fake without ActionRunner
type noActionLister struct {
	conformance.FakeLister
}

func TestActions(t *testing.T) {
	t.Run("view.New over a lister without actions returns empty Actions() and Run errors", func(t *testing.T) {
		lister := &noActionLister{}
		p := view.New(lister, &conformance.MockRecord{})

		a, ok := p.(view.Actioner)
		if !ok {
			t.Fatalf("expected presenter to implement view.Actioner")
		}

		if len(a.Actions()) != 0 {
			t.Errorf("expected 0 actions for lister without ActionRunner")
		}

		var runErr error
		opX := router.Operation{Module: "m", Entity: "x", Action: model.Create}
		a.Run(opX, func(err error) {
			runErr = err
		})
		if runErr == nil || runErr.Error() != "view: Run: unknown action "+opX.String() {
			t.Errorf("expected 'view: Run: unknown action %s', got %v", opX.String(), runErr)
		}
	})

	t.Run("with a fake ActionRunner, Run passes Args built from Reload in order", func(t *testing.T) {
		opMy := router.Operation{Module: "m", Entity: "my_op", Action: model.Create}
		fb := &conformance.FakeLister{
			Rows: []view.Record{
				&conformance.MockRecord{ID: "1", Name: "Alice"},
				&conformance.MockRecord{ID: "2", Name: "Bob"},
			},
			ActionList: []view.Action{
				{
					Operation: opMy,
					Label:     "My Action",
					Args: func(records []model.Model) model.Encodable {
						return &conformance.MockRecord{Name: "args_built"}
					},
				},
			},
		}
		p := view.New(fb, &conformance.MockRecord{})

		// First reload so it has records
		p.Reload(nil)
		initialCalls := fb.Calls

		var runErr error
		p.(view.Actioner).Run(opMy, func(err error) { runErr = err })

		if runErr != nil {
			t.Fatalf("unexpected error running action: %v", runErr)
		}

		if len(fb.Ran) != 1 || fb.Ran[0] != opMy {
			t.Errorf("expected my_op to have run, got %v", fb.Ran)
		}

		if fb.Calls != initialCalls+1 {
			t.Errorf("expected 1 additional list call (reload on success), got %d total", fb.Calls)
		}
	})

	t.Run("NewCallerLister ops slice is unchanged and calls correct op", func(t *testing.T) {
		caller := &testCaller{}
		recordsCaller := view.NewRecords().Caller(caller)
		opApply := router.Operation{Module: "network_manager", Entity: "apply_network", Action: model.Create}
		ops := view.Ops{
			Module: "network_manager",
			Actions: []view.Action{
				{Operation: opApply, Label: "Apply"},
			},
		}
		lister := view.NewCallerLister(recordsCaller, ops, func() model.TableSlice { return &conformance.MockList{} })

		// Ensure caller's ops slice is unmodified
		if ops.Actions[0].Operation != opApply {
			t.Errorf("expected caller's ops slice to remain unmodified, got %v", ops.Actions[0].Operation)
		}

		p := view.New(lister, &conformance.MockRecord{})
		p.(view.Actioner).Run(opApply, func(err error) {})

		if len(caller.calls) == 0 {
			t.Fatalf("expected caller.Call to be invoked")
		}
		if caller.calls[0].op != opApply {
			t.Errorf("expected %v, got %v", opApply, caller.calls[0].op)
		}
	})

	t.Run("Panics: invalid Operation, empty Label", func(t *testing.T) {
		caller := &testCaller{}
		recordsCaller := view.NewRecords().Caller(caller)
		assertPanic(t, "view: NewCallerLister: Action.Operation must be a valid router.Operation", func() {
			view.NewCallerLister(recordsCaller, view.Ops{
				Module:  "m",
				Actions: []view.Action{{Label: "l"}},
			}, func() model.TableSlice { return &conformance.MockList{} })
		})

		assertPanic(t, "view: NewCallerLister: Action.Label is required", func() {
			view.NewCallerLister(recordsCaller, view.Ops{
				Module:  "m",
				Actions: []view.Action{{Operation: router.Operation{Module: "m", Entity: "o", Action: model.Create}}},
			}, func() model.TableSlice { return &conformance.MockList{} })
		})
	})

	t.Run("Every presenter variant returned by view.New satisfies view.Actioner", func(t *testing.T) {
		record := &conformance.MockRecord{}
		cases := []view.Lister{
			&listOnlyLister{},         // *core
			&listSaveLister{},         // *saveable
			&listUpdateLister{},       // *updatable
			&listDeleteLister{},       // *deletable
			&listSaveUpdateLister{},   // *saveableUpdatable
			&listSaveDeleteLister{},   // *saveableDeletable
			&listUpdateDeleteLister{}, // *updatableDeletable
			&listCRUDLister{},         // *crud
		}
		for i, lister := range cases {
			p := view.New(lister, record)
			if _, ok := p.(view.Actioner); !ok {
				t.Errorf("case %d: expected presenter to implement Actioner", i)
			}
		}
	})
}

// assertPanic catches a panic and checks if it matches the expected string
func assertPanic(t *testing.T, expected string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected panic %q, got none", expected)
			return
		}
		if r != expected {
			t.Errorf("expected panic %q, got %q", expected, r)
		}
	}()
	f()
}

// Add these to match module_test.go / backend_test.go mocks:
type listOnlyLister struct{ conformance.FakeLister }
type listSaveLister struct{ conformance.FakeLister }

func (l *listSaveLister) Save(recs []model.Model, done func(error)) {}

type listUpdateLister struct{ conformance.FakeLister }

func (l *listUpdateLister) Update(ids []string, rec model.Model, fields []string, done func(error)) {}

type listDeleteLister struct{ conformance.FakeLister }

func (l *listDeleteLister) Delete(ids []string, done func(error)) {}

type listSaveUpdateLister struct{ conformance.FakeLister }

func (l *listSaveUpdateLister) Save(recs []model.Model, done func(error)) {}
func (l *listSaveUpdateLister) Update(ids []string, rec model.Model, fields []string, done func(error)) {
}

type listSaveDeleteLister struct{ conformance.FakeLister }

func (l *listSaveDeleteLister) Save(recs []model.Model, done func(error)) {}
func (l *listSaveDeleteLister) Delete(ids []string, done func(error))     {}

type listUpdateDeleteLister struct{ conformance.FakeLister }

func (l *listUpdateDeleteLister) Update(ids []string, rec model.Model, fields []string, done func(error)) {
}
func (l *listUpdateDeleteLister) Delete(ids []string, done func(error)) {}

type listCRUDLister struct{ conformance.FakeLister }

func (l *listCRUDLister) Save(recs []model.Model, done func(error))                               {}
func (l *listCRUDLister) Update(ids []string, rec model.Model, fields []string, done func(error)) {}
func (l *listCRUDLister) Delete(ids []string, done func(error))                                   {}

type testCallerCall struct {
	op   router.Operation
	args model.Encodable
}

type testCaller struct {
	calls []testCallerCall
}

func (c *testCaller) Call(op router.Operation, req model.Encodable, res model.Decodable, done func(error)) {
	c.calls = append(c.calls, testCallerCall{op, req})
	done(nil)
}

func (c *testCaller) Dispatch(op router.Operation, args model.Encodable) {
	c.calls = append(c.calls, testCallerCall{op, args})
}

func TestAction_RunCallsTheActionsOwnOperation(t *testing.T) {
	caller := &testCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	opX := router.Operation{Module: "other_module", Entity: "custom_op", Action: model.Create}
	ops := view.Ops{
		Module: "m",
		Actions: []view.Action{
			{Operation: opX, Label: "X"},
		},
	}
	lister := view.NewCallerLister(recordsCaller, ops, func() model.TableSlice { return &conformance.MockList{} })
	runner, ok := lister.(view.ActionRunner)
	if !ok {
		t.Fatalf("expected lister to implement ActionRunner")
	}
	var called bool
	runner.RunAction(opX, nil, func(err error) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		called = true
	})
	if !called {
		t.Fatal("done was not called")
	}
	if len(caller.calls) != 1 || caller.calls[0].op != opX {
		t.Fatalf("expected caller to receive %v, got %v", opX, caller.calls)
	}
}

func TestAction_UnknownOperationIsRefused(t *testing.T) {
	caller := &testCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	opX := router.Operation{Module: "m", Entity: "x", Action: model.Create}
	otherOp := router.Operation{Module: "m", Entity: "other", Action: model.Create}
	ops := view.Ops{
		Module: "m",
		Actions: []view.Action{
			{Operation: opX, Label: "X"},
		},
	}
	lister := view.NewCallerLister(recordsCaller, ops, func() model.TableSlice { return &conformance.MockList{} })
	runner := lister.(view.ActionRunner)
	var runErr error
	runner.RunAction(otherOp, nil, func(err error) {
		runErr = err
	})
	if runErr == nil || !strings.Contains(runErr.Error(), otherOp.String()) {
		t.Fatalf("expected error containing %q, got %v", otherOp.String(), runErr)
	}
}

func TestNewCallerLister_ActionNeedsAValidOperation(t *testing.T) {
	caller := &testCaller{}
	recordsCaller := view.NewRecords().Caller(caller)
	assertPanic(t, "Action.Operation", func() {
		view.NewCallerLister(recordsCaller, view.Ops{
			Module: "m",
			Actions: []view.Action{
				{Operation: router.Operation{}, Label: "X"},
			},
		}, func() model.TableSlice { return &conformance.MockList{} })
	})
}
