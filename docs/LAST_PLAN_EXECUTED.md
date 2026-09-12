---
PLAN: "chore: drop the Schema()/Pointers() stubs from list types"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **Phase C** of
> [`LIST_CONTRACT_MASTER_PLAN.md`](https://github.com/webtyp/docs/blob/main/LIST_CONTRACT_MASTER_PLAN.md).
> Runs in parallel with the other phase-C repos.
>
> **Depends on phase A** (`webtyp.com/model`) and **phase B** (`webtyp.com/ormc`).
> As the first line of work: `go get webtyp.com/model@latest`. Never add a
> `replace`, never invent a version.

# Plan — `webtyp.com/view`: a list stops claiming it has columns

## 0. Context (verified against the repo — do not re-diagnose)

`model.FielderSlice` used to embed `model.Fielder`, so every list type had to
answer "what are your columns?" — a question a sequence of rows cannot have.
`ormc` therefore emitted, on every generated list:

```go
func (s *XList) Schema() []model.Field { return nil }
func (s *XList) Pointers() []any       { return nil }
```

Nothing ever called them: the json codec reaches rows through
`Len()`/`At()`/`Append()` and type-asserts the **element**, never the list.

The harm is that having them made the lie true for the compiler. A list
satisfies `model.Fielder`, so `Accepts(&XList{})` compiles and
`mcp/tool_schema.go` believes it, publishing the tool **advertising that it
takes no arguments** — no error, no log.

Phase A narrowed `FielderSlice` to `Len`/`At`/`Append`; phase B stopped `ormc`
emitting the two stubs. This repo now carries them as dead weight. Removing them
is what closes the hole **here**: until it regenerates, its list types still
satisfy `model.Fielder`.

**This is not a size optimization.** Measured: ~27 bytes per list type, 0,02 %
of a real WASM client. Do not justify or scope this change by binary size.

**Anti-footgun.** Do NOT remove the `EncodeFields`/`DecodeFields` no-ops from
list types. `json.Encode` takes a `model.Encodable`, so deleting those breaks
every call that serializes a list. That alternative was measured and rejected.
`Len`, `At` and `Append` are the whole slice contract now and must survive
untouched.

## Quality rules

```
RULE: never hand-edit a generated *_orm.go — run the generator.
RULE: every repeated string is a named constant; string literals forbidden in logic.
RULE: this repo's behaviour must not change; only dead methods disappear.
```

## Stage 1 — the hand-written list

**File:** `conformance/conformance.go` — `MockList` is written by hand, so `ormc` never touches it.

Delete its two stub methods:

```go
func (m *MockList) Schema() []model.Field { return nil }
func (m *MockList) Pointers() []any { return nil }
```

They sit at `conformance.go:191` and `:194` at the time of writing, each under a
`// Schema implements model.Fielder.` / `// Pointers implements model.Fielder.`
comment — delete those comments too. Match on content, not line number, and note
the receiver here is `m`, not `s`.

Keep `Len`, `At`, `Append`, `IsNil`, `EncodeFields` and `DecodeFields` exactly
as they are. If the type is declared to satisfy an interface via a
`var _ model.X = (*MockList)(nil)` line, leave that line alone — it must still
compile, and that is the check.

## Acceptance criteria

1. `go build ./...`, `go vet ./...`, `go test ./...` green.
2. `grep -rn "List) Schema() \[\]model.Field" --include='*.go' .` → empty.
3. `grep -rn "List) Pointers()" --include='*.go' .` → empty.
4. `grep -rnc "Append() model.Fielder" --include='*.go' .` → unchanged from
   before the change: the traversal contract survived.
5. `go.mod` requires the phase A tag of `webtyp.com/model`; no `replace`.
6. `grep -rn "TODO\|FIXME\|Deprecated" --include='*.go' .` → only hits that
   predate this change.

## Out of scope

- Changing `model.FielderSlice` itself — phase A, already shipped.
- Changing what `ormc` emits — phase B, already shipped.
- Removing the `EncodeFields`/`DecodeFields` no-ops — measured and rejected.
- Any behaviour change in this repo. If a test fails, the cause is upstream:
  report it, do not paper over it here.

| Stage | Files | Action |
|---|---|---|
| 1 | `conformance/conformance.go` | delete `MockList`'s two stub methods by hand |
