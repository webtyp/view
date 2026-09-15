# Specification — `webtyp/view`

Strict functional requirements for the tech-agnostic CRUD view contract: the
exact public surface, the asynchronous callback semantics, the exact error
messages, and the exact wire shapes the transport adapter ships. A renderer, a
domain module, or a conformance double is correct when it matches these tables
— every table below is a test assertion.

You reach this document when you need the contract **exactly**: which method
takes which callback, what error a validation produces word for word, or what
bytes cross the `router.Caller`. For how to *use* the library read
[README.md](../README.md); for the rules a contributor must not violate read
[AGENTS.md](../AGENTS.md). This page is the single, permanent home of the
contract — the working plan documents that drove past changes are replaced or
deleted every cycle, so nothing here depends on them.

---

## 1. Public surface

```go
package view

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

// Item is ONE projected row of the list.
type Item struct {
	ID          string // selection key
	Label       string // primary text
	Description string // secondary text
	LeadTop     string // optional three-line badge, leading slot
	LeadMain    string
	LeadBottom  string
}

// Itemizer is implemented by a domain record that knows how to project itself
// as a list row. It is the ONLY view-specific code a module writes.
type Itemizer interface {
	Item() Item
}

// Lister is what a view needs from the application: the records to show.
type Lister interface {
	List(done func(rows []model.Model, err error))
}

// Presenter is the UI-agnostic core behind any CRUD view.
type Presenter interface {
	Title() string
	SearchPlaceholder() string
	Record() model.Model

	Items() []Item
	Filter(term string) []Item
	Reload(done func(error))

	Selected() string
	Select(id string) model.Model
	Deselect()
}

// Capabilities, discovered by type assertion. A Presenter carries a method
// set that mirrors the lister's (see §4).
type Saver interface {
	Save(recs []model.Model, done func(error))
}
type Updater interface {
	Update(ids []string, rec model.Model, fields []string, done func(error))
}
type Deleter interface {
	Delete(ids []string, done func(error))
}

type Ops struct {
	List   string
	Save   string
	Update string
	Delete string
}

func NewCallerLister(c router.Caller, ops Ops, newList func() model.ModelSlice) Lister
func New(l Lister, record model.Model, opts ...Option) Presenter

type Option func(*config) // unexported body
func WithTitle(title string) Option
func WithSearchPlaceholder(placeholder string) Option
```

Everything else is unexported: `core`, `indexEntry`, `lookup`, `rowName`, the
capability wrappers, and the wire envelopes `saveArgs` / `updateArgs` /
`deleteArgs`.

## 2. The asynchronous contract

Applies to `List`, `Save`, `Update`, `Delete` and `Reload` alike.

1. `done` is the **last** parameter of every call.
2. The method **returns nothing**. The result, or the error, is delivered to
   `done` exactly once.
3. `done` is **always non-nil** for the implementer. `view` normalizes a nil
   consumer callback to a no-op once at entry; an implementer may assume
   `done != nil`.
4. The implementer **must not block** and must not assume the caller has the
   result when the call returns. A transport invokes `done` from its own
   callback (a later event-loop turn); an in-process store may invoke it
   immediately. `view` must behave identically either way — see the
   `no_blocking_in_list` clause (§8).
5. **One channel of error.** Programming errors validated by `view` (an empty
   batch, an unknown id) and transport errors both arrive through `done`.
   There is no error return value to ignore.
6. A consumer may pass `nil` for `done` to fire-and-forget (e.g.
   `p.Reload(nil)`).

## 3. Presenter behaviour

### 3.1 `Reload(done func(error))`

- Normalizes `done` (§2.3), then calls `p.lister.List(cb)`.
- Inside `cb`:
  - `err != nil` → `done(err)`; `items`/`index` are left unchanged.
  - otherwise resets `items = items[:0]` and rebuilds `index` from the rows,
    appending one `Item` and one `indexEntry{id, rec}` per row.
- A row that does not implement `view.Itemizer` aborts the projection with
  `view: Reload: row type <name> does not implement view.Itemizer`, where
  `<name>` is `model.ModuleNaming.ModelName()` when the row provides it and
  `<unnamed type>` otherwise. Rows projected before the offending row stay in
  `items` (partial projection), matching the pre-callback behaviour.
- On success `done(nil)`.
- `Items()` returns the projected list; before the first successful Reload it
  is empty.

### 3.2 `Filter(term string) []Item`

- `term == ""` → a **copy** of all items (the caller may mutate it freely).
- Otherwise the items whose `Label` or `Description` matches `term` with
  `fmt.Matches` (case-insensitive substring). `Lead*` fields are never
  searched. Result order follows `items`.

### 3.3 Selection

| Call | Result |
|---|---|
| `Select(id)` with `id` in the index | sets `selected = id`, returns the record |
| `Select(id)` with unknown `id` | returns `nil`, `selected` unchanged |
| `Deselect()` | `selected = ""` |
| `Selected()` | the current selection (`""` if none) |

`lookup` scans `index` linearly — a projected list holds tens to low hundreds
of rows, so the scan is microseconds; do not reintroduce a `map`.

## 4. Capability mirroring

`view.New` returns a concrete type whose method set mirrors the lister's. A
renderer discovers capabilities by type assertion; the assertion is true on the
presenter iff the lister implements the matching interface.

| Lister implements | `view.New` returns | Presenter satisfies |
|---|---|---|
| `Lister` only | `*core` | `Presenter` |
| + `Saver` | `*saveable` | `Presenter, Saver` |
| + `Updater` | `*updatable` | `Presenter, Updater` |
| + `Deleter` | `*deletable` | `Presenter, Deleter` |
| + `Saver, Updater` | `*saveableUpdatable` | `Presenter, Saver, Updater` |
| + `Saver, Deleter` | `*saveableDeletable` | `Presenter, Saver, Deleter` |
| + `Updater, Deleter` | `*updatableDeletable` | `Presenter, Updater, Deleter` |
| + `Saver, Updater, Deleter` | `*crud` | `Presenter, Saver, Updater, Deleter` |

The wrappers are one-line delegations to `core`; no logic lives in them. There
are `2^n-1` of them for `n` optional capabilities. A fourth capability is a
design review trigger, not another eight structs.

## 5. Failure conditions — exact messages

Compared word for word by tests. Do not reword.

| Call | Condition | `done` receives |
|---|---|---|
| `Save(recs, done)` | `len(recs) == 0` | `view: Save requires at least one record` |
| `Save(recs, done)` | any record `model.IsNil` | `view: Save payload is nil` |
| `Save(recs, done)` | lister is not a `view.Saver` | `view: Save: lister does not implement view.Saver` |
| `Update(ids, rec, fields, done)` | `len(ids) == 0` | `view: Update requires at least one id` |
| `Update(ids, rec, fields, done)` | `len(fields) == 0` | `view: Update requires at least one field` |
| `Update(ids, rec, fields, done)` | `model.IsNil(rec)` | `view: Update record is nil` |
| `Update(ids, rec, fields, done)` | lister is not a `view.Updater` | `view: Update: lister does not implement view.Updater` |
| `Delete(ids, done)` | `len(ids) == 0` | `view: Delete requires at least one id` |
| `Delete(ids, done)` | any id not in the index | `view: Delete: unknown id "<id>"` — **nothing is sent** |
| `Delete(ids, done)` | lister is not a `view.Deleter` | `view: Delete: lister does not implement view.Deleter` |
| `Reload(done)` | a row is not a `view.Itemizer` | `view: Reload: row type <name> does not implement view.Itemizer` |
| `list(done)` (callerLister) | a row is not a `model.Model` | `view: Reload: row type <name> does not implement model.Model` |
| `list(done)` (callerLister) | `newList()` is not a `model.Decodable` | `view: list returned by newList does not implement model.Decodable` |

In every case `done` is invoked exactly once and the underlying lister /
caller is **not** reached.

## 6. Constructors — panic conditions

Panics are programmer wiring bugs, detected at startup; they are not routed
through `done`.

| Constructor | Condition | Panic |
|---|---|---|
| `New` | `l == nil` | `view: New: lister is required` |
| `New` | `model.IsNil(record)` | `view: New: record is required` |
| `NewCallerLister` | `c == nil` | `view: NewCallerLister: caller is required` |
| `NewCallerLister` | `ops.List == ""` | `view: NewCallerLister: Ops.List is required` |
| `NewCallerLister` | `newList == nil` | `view: NewCallerLister: newList is required` |

`New` does not change the lister's signature; `NewCallerLister` neither changes
its signature nor its panics.

## 7. The transport adapter (`NewCallerLister`)

`callerLister` is the single place a typed call becomes an operation name plus
a wire envelope.

- The returned `Lister` implements `Saver`/`Updater`/`Deleter` iff the
  matching `ops.*` name is non-empty — the same mirroring `view.New` applies,
  so a remote backend's capabilities are honest by construction.
- `list` builds `newList()`, requires it to be a `model.Decodable`, then calls
  `c.caller.Call(ops.List, nil, dec, cb)`. Inside `cb` it converts `Len()`/`At(i)`
  rows to `[]model.Model` and delivers them via `done`.
- `save` / `update` / `delete` are a single `c.caller.Call(op, envelope, nil, done)`
  — the `Caller`'s callback already has the `func(error)` shape.

Wire envelopes (`model.Encodable`):

| Envelope | Fields written |
|---|---|
| `saveArgs` | `records` array, one object per record |
| `updateArgs` | `ids` array of strings, `fields` array of strings, `record` object |
| `deleteArgs` | `ids` array of strings |

## 8. Renderer conformance — `conformance.Run(t, factory)`

A renderer is correct only if every clause passes. The clause names are the
contract:

```
mount_triggers_list_load          list_renders_item_labels
select_fills_form                 save_ships_form_values
unchanged_save_does_not_ship      revert_edit_is_not_dirty
delete_ships_selected_record      no_save_capability_without_saver
deselect_clears_selection         select_unknown_id_returns_nil
filter_matches_label_and_description
delete_unknown_id_errors          new_focuses_first_field
edit_focuses_first_field          cancel_clears_focus
no_delete_capability_without_deleter
no_update_capability_without_updater
saver_capability_mirrors_backend  plural_save_delete_and_update
no_blocking_in_list
```

`no_blocking_in_list` is the invariant the callback contract exists to
guarantee: a `Lister` whose `done` fires in a later event-loop turn must work
identically — nothing may be assumed available when `List` returns. Clauses are
never weakened or deleted; if one stops compiling, rewrite it with callbacks.

The package-level tests also pin two behaviours directly:
- `TestListDoneRunsLater` — `Items()` is empty before a deferred `done` runs,
  then correctly projected.
- `TestValidationErrorsTravelThroughDone` — every §5 failure arrives through
  `done` with the exact message, and no return value exists to ignore.

---

## Related documents

- [README.md](../README.md) — usage guide; indexes every `docs/` file.
- [AGENTS.md](../AGENTS.md) — contributor rules (async contract, WASM
  restrictions, capability-wrapper pattern, testing).