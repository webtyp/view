---
PLAN: "refactor!: one capability contract for both sides — drop BackendSaver/Updater/Deleter"
TAG: v0.4.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Do NOT run `gopush` or `codejob`.
>
> **BREAKING** (`view.Backend` implementors change two signatures). Follow-up to
> v0.3.0, see `../VIEW_PAYLOAD_SEAM_MASTER_PLAN.md` §8.

# PLAN — `tinywasm/view`: three capability interfaces, not six

## Why

v0.3.0 introduced the domain seam (`Backend`) and it was the right move. But it
declared a **second set of capability interfaces** next to the ones that already
existed, so the package now names three concepts six times:

| `backend.go` | `view.go` | Difference |
|---|---|---|
| `BackendSaver.Save(recs []model.Model) error` | `Saver.Save(recs ...model.Model) error` | slice vs variadic — no semantic content |
| `BackendUpdater.Update(ids []string, rec model.Model, fields []string) error` | `Updater.Update(ids []string, rec model.Model, fields []string) error` | **none — byte-identical** |
| `BackendDeleter.Delete(ids []string) error` | `Deleter.Delete(ids ...string) error` | slice vs variadic — no semantic content |

The smell announces itself in the code: `BackendUpdater`'s own doc comment says
*"See the Updater doc in view.go"* — a type whose documentation points at its
twin.

They are the same capability. "This backend can save records" and "this
presenter can save records" are one predicate, and after v0.3.0 the presenter's
capability set is *exactly* the backend's (`core.save` validates, asserts the
backend capability, and delegates). Two names for one predicate violates
principle 4 (*one way to do each thing*) and forces every consumer to learn a
mapping that carries no information.

**Fix:** delete the three `Backend*` interfaces. A backend declares what it can
do by implementing `view.Saver` / `view.Updater` / `view.Deleter` — the same
interfaces a renderer already asserts on the presenter. One name per capability,
used on both sides of the presenter.

---

## Repo rules

- Public library → **English** in code, comments, identifiers, error messages.
- `view` compiles to WASM: **no Go stdlib** (use `github.com/tinywasm/fmt`), **no
  `map`**, **no reflection**, **no generics**.
- `gotest`, never `go test`. Stdlib `testing` only.

---

## Step 1 — `view/backend.go`: delete the three `Backend*` interfaces

The file keeps **only** `Backend`, and its doc comment names the real
capability interfaces:

```go
package view

import "github.com/tinywasm/model"

// Backend is what a view needs from the application: the records to show.
// Mandatory — a view with nothing to list is not a view.
//
// Writing is optional and declared by implementing the SAME capability
// interfaces a renderer asserts on the Presenter: Saver, Updater, Deleter (see
// view.go). There is deliberately no separate "BackendSaver" — "this backend
// can save" and "this presenter can save" are one predicate, and view.New
// mirrors the backend's set onto the Presenter it returns. A missing method is
// a compile-time fact, not a configuration string that can be misspelled.
type Backend interface {
	// List returns every record, newest-first or in whatever order the
	// application considers natural. view projects them through Itemizer.
	List() ([]model.Model, error)
}
```

Delete `BackendSaver`, `BackendUpdater`, `BackendDeleter` entirely.

## Step 2 — `view/view.go`: the three interfaces become the shared contract

`Saver`, `Updater`, `Deleter` keep their **current signatures unchanged**
(`Save(recs ...model.Model) error`, `Update(ids []string, rec model.Model,
fields []string) error`, `Delete(ids ...string) error`). Only their docs grow
one sentence each, stating they are implemented on **both** sides:

> Implemented by a Backend to declare the capability, and re-exposed by the
> Presenter view.New returns when the backend has it.

In `New`, change the three assertions:

```go
_, hasS := b.(Saver)
_, hasU := b.(Updater)
_, hasD := b.(Deleter)
```

The 7-way `switch` returning `&crud{}` / `&saveableUpdatable{}` / … stays
byte-for-byte.

Also fix the stale comment on `Presenter.Reload` (line ~41), left over from
v0.3.0 — `ListOp` no longer exists:

```go
	Reload() error             // asks the Backend for the records, then projects and indexes them
```

## Step 3 — `view/presenter.go`: assert the shared interfaces

In `core.save` / `core.update` / `core.delete`, keep **every existing
validation** (empty recs, nil record, empty ids, empty fields, the `lookup`
unknown-id check in `delete`) and change only the assertion + the delegation:

```go
	b, ok := c.backend.(Saver)
	if !ok {
		return fmt.Err("view: Save: backend does not implement view.Saver")
	}
	return b.Save(recs...)
```

```go
	b, ok := c.backend.(Updater)
	if !ok {
		return fmt.Err("view: Update: backend does not implement view.Updater")
	}
	return b.Update(ids, rec, fields)
```

```go
	b, ok := c.backend.(Deleter)
	if !ok {
		return fmt.Err("view: Delete: backend does not implement view.Deleter")
	}
	return b.Delete(ids...)
```

## Step 4 — `view/caller_backend.go`: wrappers go variadic

The eight `caller*` structs keep their names and their `List()` methods. Only
the `Save` and `Delete` wrapper signatures change (the unexported
`callerBackend.save([]model.Model)` / `.delete([]string)` internals stay slices):

```go
func (b *callerSave) Save(recs ...model.Model) error { return b.save(recs) }
func (b *callerDelete) Delete(ids ...string) error   { return b.delete(ids) }
```

Apply to every wrapper that has the method: `callerSave`, `callerDelete`,
`callerSaveUpdate`, `callerSaveDelete`, `callerUpdateDelete`, `callerCRUD`.
`Update` wrappers are unchanged.

## Step 5 — the block comment is written once

The ~12-line comment starting *"The capability wrappers below are thin on
purpose… The count is 2^n-1…"* is currently **verbatim in both**
`presenter.go` and `caller_backend.go`. Move the canonical text to
`backend.go`, under a heading comment such as:

```go
// --- The capability-wrapper pattern -----------------------------------------
//
// <the existing 12-line explanation, written once>
```

and in each of the two zoos leave a two-line pointer:

```go
// The capability wrappers below follow the pattern documented in backend.go:
// thin structs whose only difference is the method SET. See it before editing.
```

---

## What this plan deliberately does NOT change

The executor must not "improve" these. Each is a considered decision:

- **The two capability zoos stay** — 7 structs in `presenter.go`, 8 in
  `caller_backend.go`. They wrap different concrete types (`*core` vs
  `*callerBackend`); Go without generics cannot express one in terms of the
  other, and this package forbids generics. Deleting them would mean abandoning
  capability-discovery-by-type-assertion, which is the pattern that lets a
  renderer decide at wiring time not to paint a control it cannot run. After
  Steps 1–4 the two zoos at least select over the *same three interfaces*, which
  is what makes them symmetric instead of parallel-but-subtly-different.
  **Do not merge them. Do not introduce generics. Do not replace the assertion
  with a `Can(...)` accessor.**
- **`NewCallerBackend` selecting capabilities from non-empty op strings stays.**
  It is confined to one function inside `view`, at the transport boundary where
  operation names legitimately live, and its *result* is a properly typed
  Backend. No string reaches a consumer's code.
- **`payload.go` and `conformance/payload.go` stay as they are.**

---

## Step 6 — tests and conformance

- `view/conformance/conformance.go`: `FakeBackend`'s `Save` and `Delete` become
  variadic. Any clause asserting `view.BackendSaver` / `BackendUpdater` /
  `BackendDeleter` asserts `view.Saver` / `Updater` / `Deleter` instead. The
  minimal negative-case doubles (`listOnlyBackend` and siblings) keep working;
  update their method signatures if they declare the write methods.
- Add one conformance clause pinning the new invariant: a backend implementing
  `List`+`Save` yields a Presenter that IS a `view.Saver` and is NOT an
  `Updater`/`Deleter` — **and the backend itself satisfies `view.Saver`**
  (`var _ view.Saver = theBackend`), proving the contract is genuinely shared.
- `view/tests/backend_test.go`, `view/tests/caller_backend_test.go`,
  `view/plural_test.go`, `view/tests/module_test.go`: update the in-memory
  backend's `Save`/`Delete` signatures to variadic. Every existing assertion
  must still pass unchanged.

## Step 7 — docs

`view/README.md`:

- Every example implementing a backend: `Save(recs ...model.Model) error`,
  `Delete(ids ...string) error`.
- The "Connecting a view to your data" section must state the shared contract:
  a backend declares capabilities with `view.Saver`/`Updater`/`Deleter` — the
  same interfaces the renderer asserts on the presenter.
- Migration note for v0.3.0 → v0.4.0:

  ```go
  // v0.3.0
  func (s *deviceStore) Save(recs []model.Model) error
  func (s *deviceStore) Delete(ids []string) error

  // v0.4.0 — add the ellipsis; bodies are unchanged
  func (s *deviceStore) Save(recs ...model.Model) error
  func (s *deviceStore) Delete(ids ...string) error
  ```

  `Update` is unchanged. `BackendSaver`/`BackendUpdater`/`BackendDeleter` are
  gone; use `Saver`/`Updater`/`Deleter`.

---

## Acceptance

- `grep -rn "BackendSaver\|BackendUpdater\|BackendDeleter" view/` → empty.
- `grep -c "type .* interface" view/backend.go` → `1`.
- `grep -rn "recs \[\]model.Model\|ids \[\]string) error" view/backend.go view/view.go` → empty
  (no slice-shaped capability method survives in the public contract).
- `grep -c "The count is 2\^n-1" view/presenter.go view/caller_backend.go view/backend.go`
  → `0`, `0`, `1`.
- `grep -n "ListOp" view/` → empty.
- The 7 presenter wrappers and the 8 caller wrappers still exist:
  `grep -c "^type saveable\|^type updatable\|^type deletable\|^type saveableUpdatable\|^type saveableDeletable\|^type updatableDeletable\|^type crud" view/presenter.go` → `7`.
- `gotest ./...` green. `GOOS=js GOARCH=wasm go build ./...` succeeds.

## Stages

| # | Scope | Files |
|---|---|---|
| 1 | drop the twin interfaces | `view/backend.go` |
| 2 | shared contract + stale doc | `view/view.go` |
| 3 | core asserts the shared interfaces | `view/presenter.go` |
| 4 | caller wrappers variadic | `view/caller_backend.go` |
| 5 | block comment written once | `view/backend.go`, `view/presenter.go`, `view/caller_backend.go` |
| 6 | tests + conformance | `view/conformance/conformance.go`, `view/tests/*.go`, `view/plural_test.go` |
| 7 | docs | `view/README.md` |

Final: `gotest ./...` green, WASM build clean.

---

## NOT in this plan

Consumer migration. Each backend implementor adds `...` to two signatures:
`app-demo` (3 stores), `auth` (uses `NewCallerBackend`, likely untouched), and
`layout/crudview` test doubles. Their plans are written after this publishes.
Do not edit those repos here.
