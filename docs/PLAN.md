---
PLAN: "refactor!: view addresses operations with router.Operation, list-wide Actions carry a typed operation, and view holds the layout contract (Brand, Session, Layout)"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Commit after every stage so nothing is lost, and the last step is `git push` and `gh pr create`; your final message is the pull request link, never a question.**

## Goal

`view.NewCallerLister` builds four operation names with `ops.Module + "." + crud.Op(sample, action)` and calls `router.Caller.Call` with a string; list-wide commands (`view.Action`) carry a bare string op name that `Ops.qualified()` prefixes
with the module. `crud.Op` is deleted (crud plan) and `router.Caller` takes a `router.Operation`. After this plan the lister builds `router.Operation{Module, Entity, Action}` once and an `Action` carries the whole `router.Operation`
(per the architecture rule "a different permission is its own model": a list-wide command is a CRUD operation on some model, so it has a module, an entity and an action like any other).

## Ecosystem search

Searched `qualified`, `Action{`, `Run(op`, `RunAction` over `webtyp/*` and `veltylabs/*`: the only consumers of `view.Action.Operation` are `layout/crudview` (renders the buttons and calls `Actioner.Run`, layout plan) and the fixtures in `view/conformance`
and `view/tests`. **Extend** `view`; reuse `router.Operation` (router plan). No new library.

## Design gate

1. **Prior art**: none new; this follows the `router` plan.
2. **Novice-name test**: `view.Action{Operation: router.Operation{Module: "editorial", Entity: "post_transition", Action: model.Create}, Label: "Approve"}` says what the button does.
3. **Ledger**: concepts 0; string concatenations −5; `Ops.qualified()` −1; ways to name an operation in a view 2 → 1.
4. **Where**: `view` owns the list contract and the caller adapter; the identity type is `router.Operation`.
5. **Deletes**: `Ops.qualified()`, the `opRead/opCreate/opUpdate/opDelete` string fields, `errUnknownAction(op string)`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

Dependencies (all published, no `replace`): `go get webtyp.com/model@v0.2.18 webtyp.com/router@v0.4.21 webtyp.com/crud@v0.0.9 webtyp.com/storage@v0.2.1 && go mod tidy` (`webtyp.com/mcp` v0.3.24 is the published version that compiles with `router.Operation`; if `view` still imports it anywhere after this plan, `go get webtyp.com/mcp@v0.3.24`); the `conformance/` and `tests/` packages use the same module (no separate `go.mod`: check with `ls`). **`view` no longer depends on `mcp`** (see the deleted test below): after `go mod tidy`, `webtyp.com/mcp` is gone from `go.mod`.

## APIs used

| symbol | exact signature | file |
|---|---|---|
| `router.Operation` | `struct{ Module, Entity string; Action model.Action }`; `Valid() bool`; `String() string` | `router/op.go` (router plan) |
| `router.Caller` | `Call(op Operation, args model.Encodable, into model.Decodable, done func(err error))`; `Dispatch(op Operation, args model.Encodable)` | `router/caller.go` (router plan) |
| `crud.ReadArgs`, `crud.CreateArgs`, `crud.UpdateArgs`, `crud.DeleteArgs` | unchanged wire shapes | `crud/read.go`, `create.go`, `update.go`, `delete.go` |
| `crud.Module` | `ModelName() string; Definition() model.ModuleDefinition; MountOperations(reg Registry)` | `crud/contract.go` (crud plan) |

If a signature differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — tests first (red)

Edits in `tests/` and `conformance/` (find every site with `grep -rn "crud\.Operation\|op string\|Operation: \"" tests conformance`):

| Where | Old | New |
|---|---|---|
| fake callers: `testCaller` (`action_test.go:205,210`), `recordCaller` (`caller_backend_test.go:23,36`), `singleAnswerCaller` (`records_test.go:162,171`), `dummyCaller` (`plural_test.go:23,41`) | `Call(op string, …)`, `Dispatch(op string, …)`; they record `op` as a string | `Call(op router.Operation, …)`, `Dispatch(op router.Operation, …)`; record the `router.Operation` |
| expectations `expectedOp := "m." + crud.Op(&conformance.MockRecord{}, model.Read)` (`page_test.go:25`, `caller_backend_test.go:71,88,111`, `plural_test.go:25,130,181,249`, `module_test.go:70,92`, `payload_json_test.go:32`) | a string | `router.Operation{Module: "m", Entity: (&conformance.MockRecord{}).ModelName(), Action: model.Read}` (Create/Update/Delete likewise); compare with `==` |
| `tests/harvest_consumer_test.go` (`TestQualifiedNameMatchesHarvestOps`) | compared the string the client calls with the tool name `mcp` builds, to catch the two sides building different strings | **delete the file** (this repo has no `docs/TESTS.md` row for it): both sides now use the same `router.Operation` value, so they cannot disagree (the `mcp` plan tests `ToolNameOf`). This removes the only import of `mcp` in `view` |
| `demo_test.go:14` `mockDemoModule.MountOperations` | no `Definition()` | add `func (mockDemoModule) Definition() model.ModuleDefinition { return model.ModuleDefinition{Name: <its ModelName()>} }` |
| `Action` literals (`action_test.go:47,83,117`, `conformance/conformance.go:335,336,355,381,401`) | `Op: "my_op"`, `"apply_network"`, `"o"`, `"a"`, `"b"` | `Operation: router.Operation{Module: "m", Entity: "my_op", Action: model.Create}` (any valid triple; use distinct `Entity` per old name) |
| `Run`/`RunAction`/`ClickAction`/`ActionEnabled` (`conformance/conformance.go:47,232,234`) | `op string` | `op router.Operation` |

New tests (`tests/action_test.go`, `tests/caller_backend_test.go`):

| Test | setup → call → expected |
|---|---|
| `TestAction_RunCallsTheActionsOwnOperation` | `Ops{Module: "m", Actions: [{Operation: opX, Label: "X"}]}` over a recording caller; `RunAction(opX, args, done)` → the caller received exactly `opX` (module of the action, not `Ops.Module`) |
| `TestAction_UnknownOperationIsRefused` | `RunAction(otherOp, nil, done)` → `done` receives an error containing `otherOp.String()` |
| `TestNewCallerLister_ActionNeedsAValidOperation` | an `Action{Operation: router.Operation{}, Label: "X"}` → `NewCallerLister` panics with a message containing `Action.Operation` |
| `TestCallerLister_OperationsAreTypedPerAction` | a lister over `MockRecord` with module `m` → read, create, update, delete each call the caller with `router.Operation{Module:"m", Entity:"mock_record"-style name, Action: …}` (one test, four assertions) |

Run `gotest`: it does not compile. Commit: `test: typed operations in view (red)`.

## Stage 2 — the code

`action.go`:

```go
// errUnknownActionPrefix is the exact message of SPECS §9 for an op no action declares.
const errUnknownActionPrefix = "view: Run: unknown action "

func errUnknownAction(op router.Operation) error { return fmt.Err(errUnknownActionPrefix + op.String()) }

// Action is a command an operator runs on the list as a whole. It is not an edit of a record: it has no form, it is usually not
// undoable, and its arguments come from the data on screen. Its operation is a CRUD operation on some model (a different
// permission is its own model).
type Action struct {
	Operation      router.Operation // the operation the button runs, with its module
	Label   lang.Text // button text (dictionary key)
	Confirm lang.Text // question asked before running; "" = runs on click
	// Args builds the op's arguments from the records of the last Reload. nil = the op takes no arguments.
	Args func(records []model.Model) model.Encodable
}

type Actioner interface {
	Actions() []Action                      // the backend's actions; empty when it offers none
	Run(op router.Operation, done func(error))     // see behaviour below
}

type ActionRunner interface {
	Actions() []Action
	RunAction(op router.Operation, args model.Encodable, done func(error))
}
```

(import `webtyp.com/router`; the compile-time `var _ Actioner = …` block does not change.)

`presenter.go:224`: `func (c *core) Run(op router.Operation, done func(error))` — the body compares `a.Operation == op` instead of strings; the messages that interpolate the op use `op.String()`.

`caller_lister.go`:

```go
// Ops names the module a CallerLister talks to. The four CRUD operations are derived from the model:
// router.Operation{Module: Module, Entity: <the model's name>, Action: …}. Write capabilities are governed by the model's Allow().
type Ops struct {
	Module  string
	Actions []Action
	Page    Page // how much the list loads on open and per search or scroll step; zero = DefaultPage
}
```

Delete `func (o Ops) qualified() Ops`. In `NewCallerLister`:

```go
	for _, act := range ops.Actions {
		if !act.Operation.Valid() {
			panic("view: NewCallerLister: Action.Operation must be a valid router.Operation")
		}
		if act.Label == "" {
			panic("view: NewCallerLister: Action.Label is required")
		}
	}
	// …sample/allow checks unchanged…
	entity := sample.ModelName()
	cb := &callerLister{
		caller:     c,
		rawActions: ops.Actions,
		newList:    func() model.ModelSlice { return newList() },
		records:    rc.records,
		modelName:  entity,
		opRead:     router.Operation{Module: ops.Module, Entity: entity, Action: model.Read},
		opCreate:   router.Operation{Module: ops.Module, Entity: entity, Action: model.Create},
		opUpdate:   router.Operation{Module: ops.Module, Entity: entity, Action: model.Update},
		opDelete:   router.Operation{Module: ops.Module, Entity: entity, Action: model.Delete},
	}
```

(keep `ops: ops` in the struct for `Page()`; remove the `ops.qualified()` call.) In `callerLister` the four fields become `router.Operation`; `RunAction` becomes

```go
func (b *callerLister) RunAction(op router.Operation, args model.Encodable, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	for _, act := range b.rawActions {
		if act.Operation == op {
			b.caller.Call(op, args, nil, done)
			return
		}
	}
	done(errUnknownAction(op))
}
```

`demo.go:26` comment: `crud.Caller` → `crud.CurrentUser`. `harvest`/`demo` code that calls `crud.LoopbackActingAs` (if any in this repo: `grep -rn "Loopback" *.go`) passes `nil` as the publisher first. `gotest` passes. Commit: `refactor!: view addresses operations with router.Operation`.

## Stage 3 — the layout contract: `Brand`, `Session`, `Layout` (decision D50)

`view` is the one contract between an app and the layout that draws it (D16). Today `appkit` and `layout` import each other (`layout/app` takes `appkit.App` and `appkit.Brand`); with the three types below they exchange only `view` types, so neither imports the other and the consumer app picks the layout.
Nothing here is used by `view` itself: it is the contract. Add `session.go`:

```go
package view

import (
	"webtyp.com/router"
	"webtyp.com/time"
)

// Brand is what an app shows on its entry screens and in its layout.
//
//	view.Brand{Name: "MJosefa CMS", LogoMark: logo}
type Brand struct {
	Name     string
	LogoMark string // data: URL or path of the logo image
}

// Session is everything appkit hands to a Layout once it has built the app's modules for the browser: the app does not know how it is drawn.
//
//	layout.Show(view.Session{Brand: brand, Caller: caller, Zone: zone, Screens: screens, Entry: entry, Log: log})
type Session struct {
	Brand   Brand
	Caller  router.Caller // the connection to the server: the layout asks who the user is and signs out through it
	Zone    time.Zone
	Screens []Screen     // the menu, in the order of the app's modules (the first one opens first)
	Entry   EntryScreens // sign-in and first-setup screens; zero when no module sets them
	Log     func(...any) // where the layout writes warnings
}

// Layout is the layout an app chose, seen by appkit: it draws a Session. The app's main picks the implementation; appkit never imports it.
//
//	appkit.Browser(config.App, layoutapp.New(devtools.Menu()))
type Layout interface {
	Show(Session)
}
```

(`appkit.Brand` is deleted by the `appkit` plan: `App.Brand` becomes a `view.Brand`.) Tests first, `tests/session_test.go` (package `tests`, with the fixtures already in `tests/`):

| Test | setup → call → expected |
|---|---|
| `TestLayout_ADrawsWhatItIsGiven` | a fake type `recordingLayout{got view.Session}` with `Show(s view.Session) { l.got = s }`, used through a `view.Layout` variable; `Show(view.Session{Brand: view.Brand{Name: "acme"}, Screens: []view.Screen{{ID: "a"}}})` → `got.Brand.Name == "acme"` and `len(got.Screens) == 1` (the contract carries the values and no module or layout package is imported by the test) |

Commit: `feat: Brand, Session and Layout (the contract an app and a layout exchange)`.

## Stage 4 — translations and documentation

- `langc sync .`: zero untranslated keys, zero reuse warnings (`errUnknownActionPrefix` is unchanged).
- `README.md` (≤ 150 lines, four ordered sections): the guide shows `view.Ops{Module: "agenda"}` and, for an action, `view.Action{Operation: router.Operation{…}, Label: …}`.
- `docs/ARCHITECTURE.md`: state that every operation a view calls is a `router.Operation`; the four CRUD ones are derived from the model, custom ones are CRUD on another model.
- This repo has no `docs/TESTS.md` and `gotest` checks no test index here: do not create one. Do not link `docs/PLAN.md` from permanent docs.

Commit: `docs: typed operations`.

## Delivery check

0. `grep -rn "webtyp.com/mcp" --include=*.go --include=go.mod .` → empty (view does not depend on mcp).
1. `gotest` (builds, vets, runs; a PR that does not compile, or whose tests could not run, is a failed delivery).
2. `langc sync .` → zero untranslated, zero reuse warnings.
3. `grep -rn "crud\.Operation(\|qualified()\|op string" --include=*.go .` → empty (outside generated code).
4. `git diff --stat main` lists only `action.go`, `presenter.go`, `caller_lister.go`, `demo.go`, `session.go`, `conformance/*`, `tests/*`, `go.mod`, `go.sum`, `README.md`, `docs/*`.
5. `layout`, `appkit` and the modules call `Actioner.Run`/`view.Action`; they change in their own plans.

| Stage | Files | Commit |
|---|---|---|
| 1 | `tests/*`, `conformance/conformance.go` | `test: … (red)` |
| 2 | `action.go`, `presenter.go`, `caller_lister.go`, `demo.go` | `refactor!: …` |
| 3 | `session.go`, `tests/session_test.go` | `feat: Brand, Session and Layout` |
| 4 | `lang.json`, `README.md`, `docs/*` | `docs: …` |
