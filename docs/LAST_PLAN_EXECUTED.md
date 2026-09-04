---
PLAN: "refactor!: view takes a typed Backend instead of a router.Caller + op strings"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Do NOT run `gopush` or `codejob`.
>
> **This is a BREAKING change** (`view.New` changes signature). It is Fase A of
> `../VIEW_PAYLOAD_SEAM_MASTER_PLAN.md`. Consumers (`app-demo`, `auth`,
> `layout/crudview` tests) are migrated in later phases with their own plans —
> **do not touch other repos from here.**

# PLAN — `tinywasm/view`: seam de dominio (`Backend`) en vez de seam de transporte

## Why (read this before writing code)

`view` is a **domain** layer but asks its consumer for a **transport**:

```go
// today
func New(caller router.Caller, record model.Model, listOp string,
         newList func() model.ModelSlice, opts ...Option) Presenter
```

`router.Caller` is `Call(op string, args model.Encodable, into model.Decodable,
done func(error))`. That is right for a real transport — `mcp/caller.go`
implements it correctly by serialising the payload to JSON without ever looking
inside. But it forces **every** consumer, including in-process ones that never
serialise anything, to behave like a transport:

1. It must **unpack an unexported envelope** (`saveArgs{records}`,
   `updateArgs{ids,fields,record}`, `deleteArgs{ids}` in `payload.go`) by
   hand-writing a `model.FieldWriter`. That ~80-line block is currently copied
   in five places across the monorepo, including this repo's own
   `conformance.go`.
2. It must **route on a magic string** that the compiler never checks. The
   in-process stores' `switch op` have no `default`, so a typo'd op falls
   through, `err` stays nil, and `done(nil)` reports **success having written
   nothing** — a silent failure, the worst category under principle 6.
3. Its **capability is declared by a string**: the presenter is a `view.Saver`
   because `saveOp != ""`. With a typo the presenter *claims* it can save, the
   renderer paints the button, and nothing is written.

The fix is not a nicer unpacker — that would leave (2) and (3) untouched. The
fix is for `view` to declare its own seam in its own vocabulary, and to keep the
transport as an adapter it owns.

---

## Repo rules

- Public library → **English** in code, comments, identifiers, error messages.
- `view` compiles to WASM: **no Go stdlib** (use `github.com/tinywasm/fmt`), **no
  `map`** (linear scan over typed slices), **no reflection**, **no generics**.
- Minimal public surface; internal machinery unexported.
- `gotest`, never `go test`. Stdlib `testing` only.
- The harness rule that gates this: *"An API is not published until a
  consumer-shaped test, inside the library itself, proves it."*

---

## Step 1 — `view/backend.go` (new file)

The domain seam. Mandatory contract plus optional capabilities discovered by
type assertion — the same bag-of-capabilities pattern `view` already exposes
upward with `Saver`/`Updater`/`Deleter`.

```go
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
```

---

## Step 2 — `view/caller_backend.go` (new file): the transport adapter

The **only** place in the ecosystem that knows the envelopes and the operation
names. `payload.go` (`saveArgs`/`updateArgs`/`deleteArgs`) stays exactly as it
is and becomes this file's private detail.

```go
package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Ops names the remote operations a CallerBackend invokes. An empty name means
// the remote side does not offer that operation, and the returned Backend will
// not carry the matching capability.
type Ops struct {
	List   string
	Save   string
	Update string
	Delete string
}

// NewCallerBackend adapts a router.Caller (mcp, http, any transport) to the
// domain seam. It is the single place that translates a typed call into an
// operation name plus a wire envelope — no consumer writes that mapping again.
//
// newList builds the empty slice the transport decodes into; that is a codec
// concern, which is why it lives here and not in New.
//
// Ops.List is required. The returned Backend implements exactly the write
// capabilities whose op names are non-empty, so view.New's assertions stay
// honest for a remote backend too.
func NewCallerBackend(c router.Caller, ops Ops, newList func() model.ModelSlice) Backend {
	if c == nil {
		panic("view: NewCallerBackend: caller is required")
	}
	if ops.List == "" {
		panic("view: NewCallerBackend: Ops.List is required")
	}
	if newList == nil {
		panic("view: NewCallerBackend: newList is required")
	}
	cb := &callerBackend{caller: c, ops: ops, newList: newList}
	// … return the wrapper matching the non-empty write ops (see below)
}
```

`callerBackend` holds the unexported methods `list`, `save`, `update`, `delete`,
each doing what `presenter.go` does today (build the envelope, `Call`, block on
a `chan error`). `list` additionally decodes: `newList()` → assert
`model.Decodable` → `Call(ops.List, nil, dec, …)` → walk `list.Len()`/`At(i)`
into `[]model.Model`.

**Capability wrappers:** return one of 8 thin structs embedding `*callerBackend`
— `callerList`, `callerSave`, `callerUpdate`, `callerDelete`, `callerSaveUpdate`,
`callerSaveDelete`, `callerUpdateDelete`, `callerCRUD` — each declaring only the
methods for its non-empty ops. This is **the same 2^n−1 pattern already
documented and implemented in `presenter.go`** (see the block comment above
`type saveable struct`); copy that shape exactly, including the comment
explaining the count. Do not invent a different mechanism.

`WithArgs`'s old list-arguments hole is **not** carried over: pass `nil` args to
`Call` for the list op.

---

## Step 3 — `view/view.go`: new `New`, options pruned

```go
// New builds the presenter over a Backend. Mandatory collaborators are
// positional; a nil/empty mandatory value panics — a loud development
// diagnostic.
//
// The Presenter's capabilities MIRROR the backend's: it is a Saver iff b
// implements BackendSaver, and so on. There is no configuration that can claim
// a capability the backend does not have.
func New(b Backend, record model.Model, opts ...Option) Presenter
```

- Panic when `b == nil` (`"view: New: backend is required"`) or
  `model.IsNil(record)` (message unchanged).
- **Delete** the `listOp` and `newList` parameters.
- **Delete** the options `WithSaveOp`, `WithUpdateOp`, `WithDeleteOp`,
  `WithArgs`, and the `config` fields `saveOp`, `updateOp`, `deleteOp`, `args`.
  (`WithArgs` has **zero** call sites in the monorepo — verified — so nothing is
  lost.)
- **Keep** `WithTitle` and `WithSearchPlaceholder` unchanged.
- Capability switch: replace `hasS := cfg.saveOp != ""` (and siblings) with

  ```go
  _, hasS := b.(BackendSaver)
  _, hasU := b.(BackendUpdater)
  _, hasD := b.(BackendDeleter)
  ```

  The `switch` returning `&crud{}` / `&saveableUpdatable{}` / … stays byte-for-byte
  as it is. Only the source of the three booleans changes.
- Update `New`'s doc comment to state the mirror rule above.

---

## Step 4 — `view/presenter.go`: `core` talks to the Backend

- `core` fields: replace `caller router.Caller`, `listOp string`,
  `newList func() model.ModelSlice`, `saveOp/updateOp/deleteOp string`,
  `args func() model.Encodable` with a single `backend Backend`. Keep `record`,
  `title`, `searchPlaceholder`, `items`, `selected`, `index`.
- `Reload()`:

  ```go
  func (p *core) Reload() error {
      rows, err := p.backend.List()
      if err != nil {
          return err
      }
      p.items = p.items[:0]
      p.index = make([]indexEntry, 0, len(rows))
      for _, row := range rows {
          iz, ok := row.(Itemizer)
          if !ok {
              return fmt.Err("view: Reload: row type", rowName(row), "does not implement view.Itemizer")
          }
          it := iz.Item()
          p.items = append(p.items, it)
          p.index = append(p.index, indexEntry{id: it.ID, rec: row})
      }
      return nil
  }
  ```

  The `model.Model` assertion disappears — `List` already returns `[]model.Model`.
  `rowName` currently takes a `model.Fielder`; widen it to `model.Model` (or keep
  `model.Fielder`, which `model.Model` satisfies) so it still compiles. Keep the
  Itemizer error message and `rowName` as they are.
- `save` / `update` / `delete`: keep **every existing validation** (empty recs,
  nil record, empty ids, empty fields, the `lookup` unknown-id check in
  `delete`), then delegate:

  ```go
  func (c *core) save(recs ...model.Model) error {
      // … existing validations …
      b, ok := c.backend.(BackendSaver)
      if !ok {
          return fmt.Err("view: Save: backend does not implement view.BackendSaver")
      }
      return b.Save(recs)
  }
  ```

  Same shape for `update` (`BackendUpdater`) and `delete` (`BackendDeleter`).
  The `ok` branch is unreachable through `New` (the wrapper only exists when the
  capability does) but is the loud diagnostic for direct `core` construction.
- **Delete the `chan error` synchronisation** in all four operations — the
  Backend is synchronous. The `router` import leaves `presenter.go` (it stays
  only in `caller_backend.go`).
- The capability wrapper structs (`saveable` … `crud`) and their block comment
  stay unchanged.

---

## Step 5 — `view/payload.go`

No behavioural change. Add one line to each struct's doc comment: these are the
**wire** shapes, written and read only by `caller_backend.go`; nothing else in
the ecosystem should need to know them.

---

## Step 6 — `view/conformance`: typed doubles, and the walk copy dies

`conformance.go`:

- Replace `FakeCaller` with **`FakeBackend`** — a full-capability double
  recording typed calls:

  ```go
  type FakeBackend struct {
      Rows []model.Model // what List returns

      Calls        int
      SavedRecords []model.Model
      UpdatedIDs   []string
      UpdatedFields []string
      UpdatedRecord model.Model
      DeletedIDs   []string
      Err          error // returned by every write, for error-path clauses
  }
  ```

  implementing `List`/`Save`/`Update`/`Delete`.
- **Delete `recordInspectWriter` and `inspectArrayWriter` (~lines 757–800)** and
  every `args.EncodeFields(w)` dance at their call sites (~406, ~726–744). The
  assertions there (`2 saved records in 1 call`, `2 ids and 1 field in update`,
  `2 deleted ids`) now read `fb.SavedRecords`, `fb.UpdatedIDs`,
  `fb.UpdatedFields`, `fb.DeletedIDs` directly.
- The 18 `view.New(` call sites become `view.New(fb, &MockRecord{}, …)`.
- **Negative capability clauses** (`no_delete_capability_when_deleteop_empty`
  and any sibling): rename to the new fact and rewrite with purpose-built
  minimal doubles — e.g. `type listOnlyBackend struct{ rows []model.Model }`
  implementing only `List`, then assert `_, ok := p.(view.Deleter); !ok`. Add
  the equivalent clause for `Saver` and `Updater` so all three are pinned.
- Add a clause proving the mirror rule: a backend implementing `List`+`Save`
  yields a Presenter that IS a `Saver` and is NOT an `Updater`/`Deleter`.

`conformance/payload.go` (`Payload` / `Has`): **keep unchanged.** It asserts the
wire shape, which is now exactly what `NewCallerBackend` produces — it becomes
the tool for Step 7's transport test.

---

## Step 7 — tests in `view`

- `view/tests/backend_test.go` (new, `package view_test`) — the consumer-shaped
  proof. Define a small **real** in-memory backend (a slice of `MockRecord`, not
  an opaque stub) implementing all four interfaces, and drive
  `view.New(store, &conformance.MockRecord{}, view.WithTitle("t"))` through:
  `Reload` → `Items()` projected; `Select(id)`; `Save` adds/replaces;
  `Update(ids, rec, fields)` writes only the named columns; `Delete(ids)`
  removes; `Reload` again reflects all of it.
- `view/tests/caller_backend_test.go` (new) — the transport path still works.
  A `router.Caller` double records `(op, args)`. Build
  `view.NewCallerBackend(dbl, view.Ops{List: "l", Save: "s", Update: "u", Delete: "d"}, newMockList)`,
  wrap it in `view.New`, then:
  - `Save(r1, r2)` → double saw op `"s"`, and `conformance.Payload(args)` +
    `conformance.Has` show both records' fields;
  - `Update` → op `"u"`, payload has `ids`, `fields`, and the record's fields;
  - `Delete` → op `"d"`, payload has the ids;
  - a `view.Ops` with `Delete: ""` → the returned Backend is **not** a
    `view.BackendDeleter`, and the Presenter is **not** a `view.Deleter`.
- `view/plural_test.go` — migrate `dummyCaller` to the new seam (either a typed
  backend, or `NewCallerBackend` over the existing double if the test is about
  the wire). Keep every assertion about plural payloads: they are now assertions
  about what `NewCallerBackend` emits.
- `view/tests/module_test.go`, `view/tests/conformance_test.go` — update to the
  new `New` signature / `FakeBackend`.
- `view/mock/renderer.go` works on `view.Presenter` only — expected to need no
  change; verify it compiles.

---

## Step 8 — docs

`view/README.md` — this is the main deliverable of the step, not an afterthought:

- Update every `view.New(...)` example to the new signature.
- New section **"Connecting a view to your data"** with the two cases side by
  side:
  - **in-process** — implement `Backend` (+ the capabilities you have) directly
    on your store; show a ~15-line example with `List`/`Save`/`Update`/`Delete`;
  - **remote** — `view.NewCallerBackend(caller, view.Ops{…}, newList)`.
- One paragraph: capabilities are **methods, not strings** — the renderer paints
  `+`/`🗑`/`✏` from what your backend implements, so a missing method is a
  compile-time fact.
- A short **migration note** for consumers coming from ≤ v0.2.x: `New` lost
  `listOp`/`newList`; `WithSaveOp`/`WithUpdateOp`/`WithDeleteOp`/`WithArgs` are
  gone; a `router.Caller` consumer wraps it in `NewCallerBackend`. Include the
  `auth` before/after as the worked example:

  ```go
  // before
  view.New(caller, &User{}, OpListUsers, func() model.ModelSlice { return &UserList{} },
      view.WithTitle("Usuarios"), view.WithSaveOp(OpUpsertUser), view.WithDeleteOp(OpDeleteUser))

  // after
  b := view.NewCallerBackend(caller,
      view.Ops{List: OpListUsers, Save: OpUpsertUser, Delete: OpDeleteUser},
      func() model.ModelSlice { return &UserList{} })
  view.New(b, &User{}, view.WithTitle("Usuarios"))
  ```

---

## Acceptance

- `grep -rn "recordInspectWriter\|inspectArrayWriter" view/` → empty.
- `grep -rn "WithSaveOp\|WithUpdateOp\|WithDeleteOp\|WithArgs" view/` → empty.
- `grep -rn "router\." view/*.go` → only `caller_backend.go`.
- `grep -rn "chan error" view/presenter.go` → empty.
- `grep -rn "map\[" view/backend.go view/caller_backend.go` → empty; no Go
  stdlib import anywhere in `view`.
- `view.Backend`, `view.BackendSaver`, `view.BackendUpdater`,
  `view.BackendDeleter`, `view.Ops`, `view.NewCallerBackend` exported.
- `view/conformance/payload.go` unchanged (`git diff` empty for it).
- `gotest ./...` green. `GOOS=js GOARCH=wasm go build ./...` succeeds.

## Stages

| # | Scope | Files |
|---|---|---|
| 1 | domain seam | `view/backend.go` (new) |
| 2 | transport adapter | `view/caller_backend.go` (new), `view/payload.go` (comments) |
| 3 | New + core | `view/view.go`, `view/presenter.go` |
| 4 | conformance to typed doubles | `view/conformance/conformance.go` |
| 5 | tests | `view/tests/backend_test.go` (new), `view/tests/caller_backend_test.go` (new), `view/plural_test.go`, `view/tests/*.go` |
| 6 | docs | `view/README.md` |

Final: `gotest ./...` green, WASM build clean.

---

## NOT in this plan

Migrating consumers. Each gets its own plan once this is published:
`app-demo` (3 `memCaller` → typed stores), `auth` (one line in `NewView` +
delete `tests/payloadwalk_test.go`), `layout/crudview` (test doubles). Do not
edit those repos here.
