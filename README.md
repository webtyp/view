# tinywasm/view
<img src="docs/img/badges.svg">

Tech-agnostic CRUD view contract: a domain module declares its list, record and
operations; any renderer (DOM, HTMX, SSR, native, headless test) draws it.

This README is the **official usage document**. It is written so that an agent/LLM with
no prior context can create or edit a visual component correctly guided only by the
typed signatures below. If something here requires reading the implementation, that is
a defect — report it.

## I want to… → use

| I want to… | Use |
|---|---|
| Create a list/detail view for my model | `view.New(lister, &X{}, opts...)` where `lister` implements `view.Lister` |
| Make my rows appear in the list | Implement `Item() view.Item` on the record type (`view.Itemizer`) |
| Enable saving | Implement `Save(recs ...model.Model) error` — the returned Presenter then satisfies `view.Saver` |
| Enable field patches | Implement `Update(ids []string, rec model.Model, fields []string) error` — the Presenter then satisfies `view.Updater` |
| Enable deleting | Implement `Delete(ids ...string) error` — the returned Presenter then satisfies `view.Deleter` |
| Know if the view can save/delete (renderer side) | `s, ok := p.(view.Saver)` / `d, ok := p.(view.Deleter)` |
| Load / refresh the list | `p.Reload()` (synchronous, returns `error`) |
| Pick a record and get its full model | `m := p.Select(id)` (`nil` if the id is unknown) |
| Clear the selection | `p.Deselect()` |
| Filter the list as the user types | `p.Filter(term)` (local, case-insensitive over Label+Description) |
| Connect over a transport (mcp, http) | `view.NewCallerLister(caller, view.Ops{…}, newList)` then `view.New(l, &X{}, …)` |
| Show an error/success message | Renderer's job: branch on the `error` returned by `Reload`/`Save`/`Delete` |
| Test a renderer implementation | `conformance.Run(t, factory)` — it must pass every clause |
| Simulate a view without a browser | `view/mock.Renderer` |

## Connecting a view to your data

Two cases, side by side.

**In-process** — implement `Lister` (+ the capabilities you have) directly
on your store. There is no operation name to spell, no envelope to unpack:

```go
type deviceStore struct{ db *orm.DB }

func (s *deviceStore) List() ([]model.Model, error) {
	var rows []model.Model
	err := s.db.Query(&Device{}).ReadAll(
		func() model.Model { return &Device{} },
		func(m model.Model) { rows = append(rows, m) },
	)
	return rows, err
}

func (s *deviceStore) Save(recs ...model.Model) error {
	for _, m := range recs {
		if err := s.upsert(m.(*Device)); err != nil {
			return err
		}
	}
	return nil
}

func (s *deviceStore) Update(ids []string, rec model.Model, fields []string) error {
	return s.db.UpdateFields(rec, fields, storage.In("id", anyIDs(ids)))
}

func (s *deviceStore) Delete(ids ...string) error {
	return s.db.Delete(&Device{}, storage.In("id", anyIDs(ids)))
}

view.New(&deviceStore{db: deviceDB}, &Device{}, view.WithTitle("Computadores"))
```

**Remote** — wrap your `router.Caller` transport in the adapter this package
owns, then build the view over it:

```go
l := view.NewCallerLister(caller,
	view.Ops{List: "device.list", Save: "device.save", Delete: "device.delete"},
	func() model.ModelSlice { return &DeviceList{} })
view.New(b, &Device{}, view.WithTitle("Computadores"))
```

Capabilities are **methods, not strings** — the renderer paints `+`/`🗑`/`✏`
from what your lister implements, so a missing method is a compile-time fact.
There is no string to misspell and no silent success-without-write: if the
lister does not implement `Save`, the presenter simply is not a `view.Saver`.
The contract is shared: a lister declares capabilities with
`view.Saver`/`Updater`/`Deleter` — the same interfaces the renderer asserts on
the presenter. One name per capability, used on both sides.

## Migration note (v0.4.0 → v0.5.0)

`Backend` is now `Lister` — the seam finally follows the verb, like
`Saver`/`Updater`/`Deleter`. Same for its derivatives: `NewCallerBackend` →
`NewCallerLister`, `conformance.FakeBackend` → `conformance.FakeLister`.
Pure renames; signatures and bodies are unchanged.

## Migration note (v0.3.0 → v0.4.0)

`BackendSaver`/`BackendUpdater`/`BackendDeleter` are gone; implement
`Saver`/`Updater`/`Deleter` instead:

```go
// v0.3.0
func (s *deviceStore) Save(recs []model.Model) error
func (s *deviceStore) Delete(ids []string) error

// v0.4.0 — add the ellipsis; bodies are unchanged
func (s *deviceStore) Save(recs ...model.Model) error
func (s *deviceStore) Delete(ids ...string) error
```

`Update` is unchanged.

## Migration note (from ≤ v0.2.x)

`New` lost `listOp`/`newList`; `WithSaveOp`/`WithUpdateOp`/`WithDeleteOp` and
`WithArgs` are gone (`WithArgs` had zero call sites). A `router.Caller`
consumer wraps it in `NewCallerLister`:

```go
// before
view.New(caller, &User{}, OpListUsers, func() model.ModelSlice { return &UserList{} },
	view.WithTitle("Usuarios"), view.WithSaveOp(OpUpsertUser), view.WithDeleteOp(OpDeleteUser))

// after
l := view.NewCallerLister(caller,
	view.Ops{List: OpListUsers, Save: OpUpsertUser, Delete: OpDeleteUser},
	func() model.ModelSlice { return &UserList{} })
view.New(b, &User{}, view.WithTitle("Usuarios"))
```

## API Reference

The complete public API of `view`:

```go
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
}

// Itemizer is implemented by a domain record that knows how to project itself as a
// list row. It is the ONLY view-specific code a module writes on its model.
type Itemizer interface {
	Item() Item
}

// Presenter is the UI-agnostic core behind any CRUD view: list, select, reload.
// Always present. Save/Update/Delete are separate capabilities (see below).
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item             // projected list from the last Reload
	Filter(term string) []Item // local case-insensitive match over Label+Description; "" returns all
	Reload() error             // synchronously lists, projects and indexes

	Selected() string             // currently selected id ("" if none)
	Select(id string) model.Model // marks id and returns its record from the internal index; unknown id → nil, selection unchanged
	Deselect()                    // clears the selection
}

// Capabilities. The renderer discovers them by type assertion at the seam.
// They are only present when the lister implements the matching interface:
// no Save method ⇒ the returned value has no Save method ⇒ p.(Saver) fails.
type Saver interface {
	Save(recs ...model.Model) error
}
type Updater interface {
	Update(ids []string, rec model.Model, fields []string) error
}
type Deleter interface {
	Delete(ids ...string) error
}

// Lister is what a view needs from the application: the records to show.
type Lister interface {
	List() ([]model.Model, error)
}

// Optional write capabilities: the SAME interfaces a renderer asserts on the
// Presenter. A lister declares what it can do by implementing them.
type Saver interface {
	Save(recs ...model.Model) error
}
type Updater interface {
	Update(ids []string, rec model.Model, fields []string) error
}
type Deleter interface {
	Delete(ids ...string) error
}

// Ops names the remote operations a CallerLister invokes. An empty name means
// the remote side does not offer that operation.
type Ops struct {
	List   string
	Save   string
	Update string
	Delete string
}

func NewCallerLister(c router.Caller, ops Ops, newList func() model.ModelSlice) Lister

// Option is a functional configuration option for New.
type Option func(*config)

func WithTitle(title string) Option
func WithSearchPlaceholder(placeholder string) Option

// New builds the presenter over a Lister. Mandatory collaborators are
// positional (the compiler enforces their presence); a nil mandatory value
// panics at construction — a loud development diagnostic, never a deferred
// runtime mystery. The Presenter's capabilities MIRROR the lister's.
func New(l Lister, record model.Model, opts ...Option) Presenter
```

## Quick Start

### 1. The domain module declares its view (3 steps)

```go
package catalog

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/view"
)

// Step 1 — the record projects itself as a list row.
func (c *CatalogItem) Item() view.Item {
	return view.Item{ID: c.ID, Label: c.Name, Description: c.SKU}
}

// Step 2 — the store implements the domain seam (List + the writes it supports).
type catalogStore struct{ /* … */ }

func (s *catalogStore) List() ([]model.Model, error) { /* … */ }
func (s *catalogStore) Save(recs ...model.Model) error { /* … */ }

// Step 3 — build the presenter. No projection loop, no cache, no fill:
// the presenter lists through the lister and indexes id → model itself.
func NewCatalogView(store *catalogStore) view.Presenter {
	return view.New(store, &CatalogItem{}, view.WithTitle("Catalog Management"))
}
```

That is the entire module-side code. `CatalogItem` is generated by `ormc` and
already satisfies `model.Model`.

### 2. The renderer wraps the presenter (UI layer)

A renderer never imports the domain module. It draws `Items()`, generates form inputs
from `Record().Schema()`, and discovers capabilities by assertion:

```go
package crudview

import "github.com/tinywasm/view"

type Renderer struct{ p view.Presenter }

func (r *Renderer) Mount() {
	if err := r.p.Reload(); err != nil {
		r.ShowError(err) // messages are the renderer's concern
		return
	}
	r.drawList(r.p.Items())
	if _, ok := r.p.(view.Saver); ok {
		r.drawSaveButton() // only exists if the lister implements Save
	}
	if _, ok := r.p.(view.Deleter); ok {
		r.drawDeleteButton()
	}
}

func (r *Renderer) OnSaveClicked() {
	s := r.p.(view.Saver) // safe: the button only exists if the assertion held
	rec := r.p.Record()
	r.syncFormToRecord(rec) // explicit, unidirectional: form → record → Save
	if err := s.Save(rec); err != nil {
		r.ShowError(err)
		return
	}
	r.ShowSuccess()
}

func (r *Renderer) OnSearchTyped(term string) {
	r.drawList(r.p.Filter(term)) // local filtering
}
```

## Error model

- `Reload`, `Save`, `Update`, `Delete` are **synchronous** and return `error`. The `error` return
  IS the user-message channel: the renderer decides how to present it (toast, inline,
  console). `view` never renders, logs, or swallows messages — there is no `SetLog`.
- `New` and `NewCallerLister` **panic** on nil/empty mandatory collaborators. These are programmer wiring
  bugs, detected deterministically at startup during development (the `template.Must`
  pattern). Logging and continuing would return a half-built presenter that crashes far
  from the cause — a deferred silent failure, which the harness forbids.
- Misuse that cannot be made a compile error is a loud error, never silence:
  `Delete` of an unknown id errors (nothing is sent); a row that does not
  implement `Itemizer` makes `Reload` fail naming the offending
  record (via `model.ModuleNaming.ModelName()` when the row provides it —
  `tinywasm/fmt` has no reflect-based type-name formatter by design).

## Design Goals

1. **Agnostic to UI technology** — the module never imports `dom`, `form` or `html`;
   any renderer can draw the contract.
2. **Agnostic to codec and transport** — in-process consumers import only `model`;
   the transport's codec decodes into the module's typed list (`model.ModelSlice`)
   inside `NewCallerLister`, the single place that knows the wire shape.
3. **Compile-time safety** — mandatory collaborators are positional in `New`;
   capabilities are method sets, so a view whose lister cannot save simply has
   no `Save` method to call.
4. **Synchronous Go idiomatic design** — `Reload`/`Save`/`Update`/`Delete` block and return
   `error`. The async network caller is wrapped inside the adapter with channels. No CPS
   callbacks, no dangling UI states.
5. **Explicit form synchronization** — `Save(recs...)` takes the synchronized records
   explicitly: unidirectional data flow, no hidden shared-pointer mutations.
6. **Glue lives here, once** — projection loop, id→model index, capability wiring
   and the transport adapter are implemented in this package, not repeated in every module.

## WebAssembly/TinyGo Compatibility

To ensure 100% compatibility with WebAssembly (WASM) and TinyGo targets, standard library packages (such as `fmt`, `encoding/json`, or `encoding/binary`) should be avoided in production code. Use the following tech-agnostic, low-allocation alternatives instead:
- `github.com/tinywasm/fmt` for formatting and error creation.
- `github.com/tinywasm/json` for JSON serialization/deserialization.
- `github.com/tinywasm/binary` for binary protocols.

## Reference and Conformance

- **`view/mock`** — headless reference renderer for browser-less simulation and unit
  tests.
- **`view/conformance`** — exports `conformance.Run(t, Factory)` plus the
  `conformance.FakeLister` typed double. A renderer is correct
  only if it passes every clause (list load on mount, label rendering via `Itemizer`,
  select/deselect, save/delete capability assertions, filter semantics, loud errors on
  unknown ids). `conformance.Payload`/`conformance.Has` assert the wire shape a
  `router.Caller` double saw — the tool for testing the transport path.
