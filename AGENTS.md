# AGENTS.md — webtyp/view

Working notes for agents changing this library. Read this before touching any
file. For the usage guide read [README.md](README.md); for the exact contract
read [docs/SPECS.md](docs/SPECS.md).

## Mission

`view` is the **tech-agnostic CRUD view contract**: a domain module declares its
list, record and write operations once, and any renderer — DOM, HTMX, SSR,
native, headless test — draws it. The library owns the seam between "what the
application can do" and "what the UI shows".

The goal that decides every argument here: **someone who does not know the
domain builds a correct view guided only by the signatures.** Correctness lives
in the compiler and the signatures, not in a manual you must remember. A
signature that does not say what it does is a defect, even if it compiles.

Consumers: `webtyp/layout/crudview`, the WebTyp app modules, and any renderer
that asserts the capability interfaces. Depends on `webtyp/model` for records
and on `webtyp/router` only inside the transport adapter.

---

## 1. The async contract — the rule this library exists to hold

Every read and write is **asynchronous**: the result arrives through a `done`
callback delivered as the **last** parameter. No method returns `error`.

```go
List(done func(rows []model.Model, err error))
Save(recs []model.Model, done func(error))
Update(ids []string, rec model.Model, fields []string, done func(error))
Delete(ids []string, done func(error))
Reload(done func(error))
```

- **Never block, never use a channel.** The old shape wrapped a channel around
  an inherently asynchronous transport. Inside a JS callback (a `fetch`
  response, a click handler), blocking a goroutine on a channel that only a
  later callback can fill is **unresolvable**: the callback never runs because
  the current one never returns. Measured on Go WASM (`GOOS=js`, build mode
  `L`, the mode the apps use): `fatal error: all goroutines are asleep -
  deadlock!`, exit code 2, the whole program dies silently. TinyGo's asyncify
  hides it, which is worse — the same library works or kills the app depending
  on the toolchain. Do not reintroduce it.
- **`done` goes last**, like every other asynchronous call in the ecosystem
  (`router.Caller.Call`, `fetch…Send`).
- **`done` is always non-nil for the implementer.** A consumer may pass `nil`;
  normalize it once at the top of each core method (`if done == nil { done =
  func(error) {} }`), never with scattered `if done != nil` guards.
- **One channel of error.** Validation errors and transport errors both travel
  through `done`. A method that both returned an `error` and took a `done`
  would be two ways to report the same thing, and a consumer would ignore one.
- **`done` is called exactly once.**

## 2. Error messages are contract

The messages below are compared word for word by tests. Do not reword them; if
one must change, change the test in the same commit and say why. The full table
lives in [docs/SPECS.md](docs/SPECS.md).

```
view: Save requires at least one record
view: Save payload is nil
view: Save: lister does not implement view.Saver
view: Update requires at least one id
view: Update requires at least one field
view: Update record is nil
view: Update: lister does not implement view.Updater
view: Delete requires at least one id
view: Delete: unknown id %q
view: Delete: lister does not implement view.Deleter
view: Reload: row type %s does not implement view.Itemizer
view: Reload: row type %s does not implement model.Model
view: list returned by newList does not implement model.Decodable
```

## 3. WASM / TinyGo restrictions (do NOT violate)

- **No Go stdlib in production code.** Use `webtyp.com/fmt` for formatting and
  errors, `webtyp.com/json` for JSON, `webtyp.com/binary` for binary. Never
  import `fmt`, `strings`, `strconv`, `errors`, `encoding/json` or
  `encoding/binary` in a file that ships to the browser.
- **No `map`.** TinyGo pulls in the hashmap runtime, inflating the binary. The
  id index is a `[]indexEntry` slice (`presenter.go`), scanned linearly — a
  projected list holds tens to low hundreds of rows, so the scan costs
  microseconds. Do not "optimize" it back into a map.
- **No `reflect`.** `webtyp/fmt` has no reflect-based type-name formatter by
  design; `rowName` reads `model.ModuleNaming.ModelName()` instead.
- **No generics / no `any` in the data.** Typed fields only.

## 4. The capability-wrapper pattern — one implementation, N method sets

`core` (`presenter.go`) is the **single** place each operation is written.
`view.New` returns one of `saveable`, `updatable`, `deletable`,
`saveableUpdatable`, `saveableDeletable`, `updatableDeletable`, `crud` — thin
structs that embed `*core` and differ **only** in which methods they expose.
`caller_lister.go` mirrors the same set for the transport adapter.

- **Never move logic into a wrapper.** A wrapper body is a one-line delegation
  (`s.save(recs, done)`); the validation and the call live in `core`.
- The count is `2^n-1` for `n` optional capabilities (7 for three). That is the
  price of discovering capabilities by type assertion, which is what lets a
  renderer decide at wiring time not to paint a control it could never run.
- **A fourth capability is the moment to stop and reconsider**, not to type out
  eight more structs.
- Capabilities are **method sets, not strings**: a missing method is a
  compile-time fact. `view.New` mirrors the lister's set onto the presenter, so
  `p.(view.Saver)` is true iff the lister implements `view.Saver`.

## 5. Minimal public surface

Export exactly what a consumer uses. Everything else stays unexported — the
index, `lookup`, the wrapper bodies, the wire envelopes (`saveArgs`,
`updateArgs`, `deleteArgs`), `rowName`. You cannot misuse what you cannot see.

## 6. The only view-specific code a module writes

`Item() view.Item` on the domain record. If a change would require a module to
write more view code, the seam is in the wrong place.

## 7. Testing

```bash
go install webtyp.com/devflow/cmd/gotest@latest   # once
gotest        # vet + race + coverage + wasm + badges — NOT `go test`
```

- **`gotest`, never `go test`.** Stdlib assertions only (`testing`), no
  testify.
- **All tests live in `tests/` (package `tests`).** Unit tests, transport
  tests and the conformance wiring all go there — never a `*_test.go` at the
  module root. The full suite is `gotest`, which compiles the whole `./...`
  tree, so the location is what keeps the root package free of test files.
- **`conformance.Run(t, factory)` must pass every clause.** The clauses are the
  contract: list load on mount, label rendering via `Itemizer`, select/deselect,
  dirty-check on save, save/delete capability assertions, filter semantics,
  loud errors on unknown ids, and `no_blocking_in_list` — a lister whose `done`
  fires in a later event-loop turn must work identically. **Never weaken or
  delete a clause**; if one no longer compiles, rewrite it with callbacks.
- Doubles shared across packages live in `conformance` (`FakeLister`,
  `MockRecord`, `MockList`, `Driver`); a double used only by the `tests/`
  package stays there. Add a conformance clause there, not a private test.
- `gotest` enables the WASM suite only when the package has js/wasm-only files;
  for this repo it does not, so also run the real browser target by hand when a
  change touches the async path:
  ```bash
  GOOS=js GOARCH=wasm go test -exec "$(go env GOROOT)/lib/wasm/go_js_wasm_exec" ./...
  ```

## 8. Publishing and documentation

- `gopush 'message'`, never a raw `git commit`/`push`.
- Documentation is updated **before** publishing, in the same commit as the code
  it describes. `README.md` is the usage document and indexes every file in
  `docs/` except the ephemeral `PLAN*.md` / `LAST_PLAN_EXECUTED.md`.
- Permanent docs must **never** link to `PLAN.md` / `LAST_PLAN_EXECUTED.md` —
  they are replaced or deleted, so every such reference is a dead link.
- English only. Diagrams: `flowchart TD`, no `subgraph`, `<br/>` for breaks.
