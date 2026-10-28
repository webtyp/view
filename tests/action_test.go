package tests

import (
	"testing"

	"webtyp.com/model"
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
		a.Run("x", func(err error) {
			runErr = err
		})
		if runErr == nil || runErr.Error() != "view: Run: unknown action x" {
			t.Errorf("expected 'view: Run: unknown action x', got %v", runErr)
		}
	})

	t.Run("with a fake ActionRunner, Run passes Args built from Reload in order", func(t *testing.T) {
		fb := &conformance.FakeLister{
			Rows: []view.Record{
				&conformance.MockRecord{ID: "1", Name: "Alice"},
				&conformance.MockRecord{ID: "2", Name: "Bob"},
			},
			ActionList: []view.Action{
				{
					Op:    "my_op",
					Label: "My Action",
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
		p.(view.Actioner).Run("my_op", func(err error) { runErr = err })

		if runErr != nil {
			t.Fatalf("unexpected error running action: %v", runErr)
		}

		if len(fb.Ran) != 1 || fb.Ran[0] != "my_op" {
			t.Errorf("expected my_op to have run, got %v", fb.Ran)
		}

		if fb.Calls != initialCalls+1 {
			t.Errorf("expected 1 additional list call (reload on success), got %d total", fb.Calls)
		}
	})

	t.Run("NewCallerLister ops slice is unchanged and calls correct op", func(t *testing.T) {
		caller := &testCaller{}
		recordsCaller := view.NewRecords().Caller(caller)
		ops := view.Ops{
			Module: "network_manager",
			List:   "plan_network",
			Actions: []view.Action{
				{Op: "apply_network", Label: "Apply"},
			},
		}
		lister := view.NewCallerLister(recordsCaller, ops, func() model.ModelSlice { return &conformance.MockList{} })

		// Ensure caller's ops slice is unmodified
		if ops.Actions[0].Op != "apply_network" {
			t.Errorf("expected caller's ops slice to remain unmodified, got %s", ops.Actions[0].Op)
		}

		p := view.New(lister, &conformance.MockRecord{})
		p.(view.Actioner).Run("apply_network", func(err error) {})

		if len(caller.calls) == 0 {
			t.Fatalf("expected caller.Call to be invoked")
		}
		if caller.calls[0].op != "network_manager.apply_network" {
			t.Errorf("expected network_manager.apply_network, got %s", caller.calls[0].op)
		}
	})

	t.Run("Panics: empty Op, empty Label", func(t *testing.T) {
		caller := &testCaller{}
		recordsCaller := view.NewRecords().Caller(caller)
		assertPanic(t, "view: NewCallerLister: Action.Op is required", func() {
			view.NewCallerLister(recordsCaller, view.Ops{
				Module: "m", List: "l",
				Actions: []view.Action{{Label: "l"}},
			}, func() model.ModelSlice { return &conformance.MockList{} })
		})

		assertPanic(t, "view: NewCallerLister: Action.Label is required", func() {
			view.NewCallerLister(recordsCaller, view.Ops{
				Module: "m", List: "l",
				Actions: []view.Action{{Op: "o"}},
			}, func() model.ModelSlice { return &conformance.MockList{} })
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
	op   string
	args model.Encodable
}

type testCaller struct {
	calls []testCallerCall
}

func (c *testCaller) Call(op string, req model.Encodable, res model.Decodable, done func(error)) {
	c.calls = append(c.calls, testCallerCall{op, req})
	done(nil)
}

func (c *testCaller) Dispatch(op string, args model.Encodable) {
	c.calls = append(c.calls, testCallerCall{op, args})
}
