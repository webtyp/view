---
PLAN: "refactor!: view.Ops.Allow removed — a screen offers what the model's Allow() lists"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally** and is **published as soon as the delivery check passes** (no `replace` anywhere).
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: one declaration of what a table allows

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/model@latest webtyp.com/crud@latest
```
`model@latest` must contain `model.TableSlice` (plan A6.1). Run after `ormc` A6.4 is published (lists then implement `Allow()`). No `replace`.

## Why

Decision D20 of `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`: the table's `model.Definition.Allow` decides what the server
offers (crud mounts only those actions). `view.Ops.Allow` (`caller_lister.go:16`) declares the same thing a second time and
the two can disagree — a «Delete» button for a table the server does not let anyone delete. Now the screen reads `Allow()`
from the model; `Ops.Allow` is deleted (one intent, one path).

## Stage 1 — `caller_lister.go`

- Delete the field `Allow` from `Ops` and its mention in the `Ops` and `NewCallerLister` comments
  (`grep -rn "Allow" caller_lister.go` → only the new lines below).
- `NewCallerLister(c router.Caller, ops Ops, newList func() model.TableSlice) Lister` (was `func() model.ModelSlice`):
  a list generated before `Allow` existed does not implement `model.TableSlice`, so old code **fails to compile** instead of
  failing when the page opens (and a `gopush` cascade reverts itself on that compile error). After the existing checks:

```go
allow := newList().Allow()
if !allow.Has(model.Read) {
	panic("view: NewCallerLister: " + sample.ModelName() + " does not allow Read — add model.Read to Allow in its model.Definition (and rerun ormc)")
}
hasS := allow.Has(model.Create)
hasU := allow.Has(model.Update)
hasD := allow.Has(model.Delete)
```
  `callerLister.newList` keeps type `func() model.ModelSlice` internally (assign `func() model.ModelSlice { return newList() }`).

## Stage 2 — every other use of `Ops.Allow` in this repo

`grep -rn "Allow:" --include=*.go .` → fix each site (tests, `mock/`, `conformance/`): their test lists implement
`Allow() model.Action` (on the list type, so it is a `model.TableSlice`) returning what the old `Ops.Allow` declared, plus `model.Read`.

## Stage 3 — tests (`tests/`, rows in `docs/TESTS.md`)

| Test | Expected |
|---|---|
| `TestCallerLister_CapabilitiesFromModelAllow` | model `Allow() = Read\|Create` → the Lister implements `Saver`, not `Updater` nor `Deleter` |
| `TestCallerLister_NoReadPanics` | list `Allow() = Create` → panic message contains `does not allow Read` |

## Stage 4 — documentation

`README.md`: the «I want to…» rows about enabling save/update/delete now say «add the action to `Allow` in the model's
`Definition`». `langc sync .` if any text changed.

## Delivery check

```bash
gotest
grep -rn "Ops{[^}]*Allow" --include=*.go .   # empty
```
A tree that does not compile or whose tests did not run is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `caller_lister.go` |
| 2 | `mock/*`, `conformance/*`, tests |
| 3 | `tests/*.go`, `docs/TESTS.md` |
| 4 | `README.md`, `lang.json` |
