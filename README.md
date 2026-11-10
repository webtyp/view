# webtyp/view
<img src="docs/img/badges.svg">

UI-agnostic CRUD view and layout contract for the webtyp ecosystem.

## Why This Exists

Domain modules and rendering engines often suffer from tight coupling: views hard-code DOM/HTML constructs, while rendering libraries depend directly on specific domain models and transports. When transport protocols or rendering targets change, modules must be rewritten.

| Without this | With this |
|---|---|
| Domain screens import specific UI engines (`dom`, `htmx`) | Modules declare abstract presenters; any renderer draws them |
| String-based operation routing prone to silent runtime typos | Every operation is a typed [`router.Operation`](session.go) verified at compile time |
| Layout packages import application config and vice-versa | Clean seam via [`Brand`](session.go), [`Session`](session.go) and [`Layout`](session.go) contracts |

## Before You Start

- Define your domain model with `model.Definition` and generate helpers with `ormc`.
- Ensure your model declares allowed actions in `Allow()` (e.g. `model.Read | model.Create`).
- Wrap client transports with `view.NewRecords().Caller(caller)`.

## Step-by-Step Guide

### Step 1: Declare a remote lister with typed operations

Goal: Adapt a `router.Caller` to provide CRUD records and custom actions to the view.

```go
package main

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

type AgendaRecord struct {
	ID    string
	Title string
}

func (a *AgendaRecord) ModelName() string     { return "agenda" }
func (a *AgendaRecord) Item() model.Item      { return model.Item{ID: a.ID, Label: a.Title} }
func (a *AgendaRecord) Schema() []model.Field { return nil }
func (a *AgendaRecord) Pointers() []any       { return []any{&a.ID, &a.Title} }
func (a *AgendaRecord) IsNil() bool           { return a == nil }

type AgendaList struct {
	items []*AgendaRecord
}

func (l *AgendaList) ModelName() string      { return "agenda" }
func (l *AgendaList) Allow() model.Action    { return model.Read | model.Create }
func (l *AgendaList) Len() int               { return len(l.items) }
func (l *AgendaList) At(i int) any           { return l.items[i] }
func (l *AgendaList) Append() any            { r := &AgendaRecord{}; l.items = append(l.items, r); return r }
func (l *AgendaList) Schema() []model.Field  { return nil }
func (l *AgendaList) Pointers() []any        { return nil }
func (l *AgendaList) IsNil() bool            { return l == nil }

func SetupView(caller router.Caller) view.Presenter {
	records := view.NewRecords()
	lister := view.NewCallerLister(
		records.Caller(caller),
		view.Ops{
			Module: "agenda",
			Actions: []view.Action{
				{
					Operation: router.Operation{Module: "agenda", Entity: "post_transition", Action: model.Create},
					Label:     "Approve",
				},
			},
		},
		func() model.TableSlice { return &AgendaList{} },
	)
	return view.New(lister, &AgendaRecord{}, view.WithTitle("Agenda"))
}

func main() {
	fmt.Println("view initialized")
}
```

Expected result: The presenter is ready for any renderer, offering `Items()`, selection state, and typed actions.

### Step 2: Render and run actions from the UI layer

Goal: Mount the presenter and trigger actions without knowing server transport details.

```go
package main

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

func RunAction(p view.Presenter, op router.Operation) {
	if a, ok := p.(view.Actioner); ok {
		a.Run(op, func(err error) {
			if err != nil {
				fmt.Println("action failed:", err)
				return
			}
			fmt.Println("action succeeded, list reloaded")
		})
	}
}

func main() {
	fmt.Println("action runner ready")
}
```

Expected result: The operation executes asynchronously and reloads the presenter on success.

## Documentation Map

| Order | Document | What question it answers & when to read it |
|---|---|---|
| 1 | [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Component architecture, layout contracts and typed operations |
| 2 | [docs/SPECS.md](docs/SPECS.md) | Exact behavior, capability mirroring and error messages |
| 3 | [docs/LIST_PAGING.md](docs/LIST_PAGING.md) | Pagination cursors and infinite-scroll mechanics |
| 4 | [docs/LAST_PLAN_EXECUTED.md](docs/LAST_PLAN_EXECUTED.md) | Specification of the most recent locally executed plan |
