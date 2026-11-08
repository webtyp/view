---
PLAN: "refactor!: wire contract from webtyp.com/crud; view.Ops on model.Action"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: Extract Wire Contract to `webtyp.com/crud` and Standardize `view.Ops` on `model.Action`

## Tools and local dependencies

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```
`go.mod`: `replace` for `webtyp.com/model`, `webtyp.com/router`, `webtyp.com/crud`, `webtyp.com/rpc` to `../model`, `../router`, `../crud`, `../rpc` (local, unpublished); then `go mod tidy`.
Since master D13, `crud.ReadArgs` has a last field `Filter model.Model`: `view` never sets it (it stays nil); do not add filter UI here.
Every `webtyp.com/router/loopback` import in `tests/` becomes `crud.Loopback(mods...)` / `crud.LoopbackActingAs(userID, mods...)` (the router package no longer exists), and every test module becomes a `crud.Module` (`MountOperations(reg crud.Registry)`); server-side helpers that called `view.MountSave/MountUpdate/MountDelete` or `crud.MountCreate/...` use `reg.Create/Read/Update/Delete`.


## Context & Goal

`webtyp.com/crud` now owns the wire contract between client screens and server handlers (`ReadArgs`, `CreateArgs`, `UpdateArgs`, `DeleteArgs`, `Cap`, `MaxPage`, `Op`, and the closed `crud.Registry` modules mount on). `webtyp.com/view` remains strictly the browser-side presenter and screen library (`Lister` and `List` represent the visual UI list component, while the wire payload is `crud.ReadArgs`).

This plan removes the server/wire duplicates from `view` and updates `view.Ops` / `NewCallerLister` to derive the 4 standard CRUD operation names via `crud.Op(sample, action)` and `model.Action` instead of accepting `List`, `Save`, `Update`, `Delete` string fields.

## Explicit Deletions

Delete these files and symbols completely (no aliases, no shims):
- Delete `mount.go` (`MountSave`, `MountUpdate`, `MountDelete`).
- Delete `payload.go` (`saveArgs`, `updateArgs`, `deleteArgs`, `saveIn`, `updateIn`, `deleteIn`, `ErrNoRecords`, `ErrNoIDs`, `ErrNoFields`, `ErrCannotRead`).
- Delete `tests/mount_test.go` (moved to `webtyp.com/crud/tests/mount_test.go`).
- In `page.go`: delete `MaxPage`, `Cap`, `ListArgs`, `ListArgsModel`, and all `(*ListArgs)` methods (`Schema`, `Pointers`, `IsNil`, `EncodeFields`, `DecodeFields`). Keep `Page`, `All`, `DefaultPage`, and `Page.declared()`.
- In `tests/page_test.go`: delete `TestCap` and `TestListArgs_RoundTrip` (moved to `webtyp.com/crud/tests/read_test.go`); keep `TestCallerLister_SendsSearchAndPage`.

## Stage 1: Replace `ListArgs` with `crud.ReadArgs` Across `view`

### Files to Modify
- `go.mod`: require `webtyp.com/crud` (local `replace`, see above).
- `lister.go`:
  - Import `"webtyp.com/crud"`.
  - Change `Lister.List` signature to `List(args crud.ReadArgs, done func(rows []Record, err error))`.
- `presenter.go`:
  - Import `"webtyp.com/crud"`.
  - In `Reload`: replace `ListArgs{Limit: MaxPage}` with `crud.ReadArgs{Limit: crud.MaxPage}` and `ListArgs{Limit: int64(page.First)}` with `crud.ReadArgs{Limit: int64(page.First)}`.
  - In `Search`: replace `ListArgs{Search: term, Limit: int64(page.Size)}` with `crud.ReadArgs{Search: term, Limit: int64(page.Size)}`.
  - In `More`: replace `ListArgs{...}` with `crud.ReadArgs{Search: p.term, After: p.order[len(p.order)-1], Limit: int64(page.Size)}`.
  - In `load`: change parameter `args ListArgs` to `args crud.ReadArgs`.
- `conformance/conformance.go`:
  - Import `"webtyp.com/crud"`.
  - Change `FakeLister.LastArgs` and `FakeLister.List` (plus all minimal `Lister` test doubles in `conformance.go`) from `view.ListArgs` to `crud.ReadArgs`, and `view.MaxPage` to `crud.MaxPage`.

## Stage 2: Standardize `view.Ops` and `NewCallerLister` on `model.Action` and `crud.*Args`

### Files to Modify
- `caller_lister.go`:
  - Import `"webtyp.com/crud"`.
  - Replace `List`, `Save`, `Update`, `Delete` string fields on `Ops` with `Allow model.Action`:
    ```go
    type Ops struct {
    	Module  string
    	Allow   model.Action // write capabilities (model.Create, model.Update, model.Delete; model.Read is always enabled)
    	Actions []Action
    	Page    Page // how much the list loads on open and per search or scroll step; zero = DefaultPage
    }
    ```
  - In `NewCallerLister(c router.Caller, ops Ops, newList func() model.ModelSlice) Lister`:
    - Validate `c != nil`, `c.(*recordsCaller)`, `ops.Module != ""`, `newList != nil`, and `ops.Actions`.
    - Obtain `sample := newList().Append().(model.Model)`.
    - Compute the 4 qualified CRUD operation names using `crud.Op(sample, action)`:
      - `opRead: ops.Module + "." + crud.Op(sample, model.Read)`
      - `opCreate: ops.Module + "." + crud.Op(sample, model.Create)`
      - `opUpdate: ops.Module + "." + crud.Op(sample, model.Update)`
      - `opDelete: ops.Module + "." + crud.Op(sample, model.Delete)`
    - Determine capabilities from `ops.Allow`:
      - `hasS := ops.Allow.Has(model.Create)`
      - `hasU := ops.Allow.Has(model.Update)`
      - `hasD := ops.Allow.Has(model.Delete)`
  - In `(*callerLister).list(args crud.ReadArgs, done func([]Record, error))`: call `b.caller.Call(b.opRead, &args, dec, ...)`.
  - In `(*callerLister).save(recs []model.Model, done func(error))`: call `b.caller.Call(b.opCreate, &crud.CreateArgs{Records: recs}, saved, ...)`.
  - In `(*callerLister).update(ids []string, rec model.Model, fields []string, done func(error))`: call `b.caller.Call(b.opUpdate, &crud.UpdateArgs{IDs: ids, Fields: fields, Record: rec}, nil, done)`.
  - In `(*callerLister).delete(ids []string, done func(error))`: call `b.caller.Call(b.opDelete, &crud.DeleteArgs{IDs: ids}, nil, ...)`.
  - Update all 8 capability wrapper `List` methods (`callerList`, `callerSave`, `callerUpdate`, `callerDelete`, `callerSaveUpdate`, `callerSaveDelete`, `callerUpdateDelete`, `callerCRUD`) to take `args crud.ReadArgs`.
- Update `tests/action_test.go`, `tests/backend_test.go`, `tests/caller_backend_test.go`, `tests/harvest_consumer_test.go`, `tests/module_test.go`, `tests/page_test.go`, `tests/payload_json_test.go`, `tests/plural_test.go`, `tests/records_test.go`, `README.md`, and `docs/SPECS.md` to use `view.Ops{Module: ..., Allow: ...}` and `crud.ReadArgs`.

## Delivery Check

```bash
langc sync
gotest
```
