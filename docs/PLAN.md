---
PLAN: "feat: MountSave and MountDelete — the server half of the shapes the screen sends; no module declares {records}/{ids} again"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 3150967760845298565
PR: https://github.com/webtyp/view/pull/11
REVIEWED_COMMIT: 1a2ee8a5ca71010b6e521a6f2803208f2e8479bb
---

> Read AGENTS.md at the repo root before any change.

# PLAN — `view.MountSave` / `view.MountDelete`

## Why

The generic screen saves by sending `{"records":[…]}` (and expects the last saved record back) and
deletes by sending `{"ids":[…]}` (`caller_lister.go:185,205`, shapes in `payload.go`). Those shapes
are unexported here, yet every module has to re-declare them to read them: `service_catalog` did
(`UpsertServiceArgs`, `DeleteServicesArgs` + hand-written loops), `device_manager` and
`staff_manager` did not — so their screens cannot save (finding H3 of
`veltylabs/mjosefa-cms/docs/AUDITORIA_MVP.md`). Decision D27 of
`veltylabs/mjosefa-cms/docs/MVP_DEPLOYMENT_ROADMAP.md`: the framework mounts save and delete with
the same shapes it sends; the module only says how to save one record and how to delete ids.

## Design gate (api-design)

1. **Prior art.** Django REST Framework `ModelViewSet`
   (https://www.django-rest-framework.org/api-guide/viewsets/): the framework provides create,
   update and destroy; the developer gives the model. Here the module gives one function per record.
2. **Names.** `view.MountSave(reg, op, save)` — «mount the save operation»; `view.MountDelete`.
   Same verb as `rpc.Mount`, `erd.Mount`.
3. **Ledger.** Lines per module: −(args type + Encode/Decode + loop) ≈ −40; ways to read the save
   shape: from N (each module) to 1.
4. **Where.** `view`: it owns the shapes (`payload.go`); writing them and reading them live together.
5. **Deletes.** Nothing here; the module plans delete their `Upsert*Args`/`Delete*Args`.

## APIs used

| Symbol | Exact | Where |
|---|---|---|
| `router.OperationRegistry` | `interface { Operation(name string, h HandlerFunc) Route }` | router (`router.go:192`) |
| `router.Route` | has `Requires(model.Resource, model.Action) Route`, `Accepts(model.Fielder) Route` | router (`route.go`) |
| `router.Context` | `Decode(into model.Decodable) error`, `Encode(v model.Encodable) error`, `WriteStatus(int)` | router (`router.go`) |
| `model.FieldReader.Array` | `Array(name string) (ArrayReader, bool)`; `ArrayReader.Len() int`, `ArrayReader.Object(i int, into Decodable) bool`, `ArrayReader.String(i int) string` | model (`codec.go`) |
| `rpc.WriteError` | `func WriteError(ctx router.Context, err error)` | `webtyp.com/rpc` v0.2.7 (add to go.mod; rpc does not import view, no cycle) |
| `loopback.ActingAs` | `func ActingAs(tenantID, userID string, mods ...router.OperationModule) router.Caller` (tests only) | `webtyp.com/router/loopback` |

## Stage 1 — `mount.go` (new, no build tag)

```go
// MountSave registers op as the save operation of a screen: it reads the records
// the screen sends ({"records":[…]}), calls save for each one in order, and
// answers the last one saved (save may fill its id). Errors go through
// rpc.WriteError: no records → 400; a save error → its own status.
//
//	view.MountSave(reg, OpUpsertDevice, func(d *Device) error { return m.SaveDevice(d) }).
//		Requires(ResourceDevice, model.Create|model.Update)
func MountSave[T any, PT interface {
	*T
	model.Model
}](reg router.OperationRegistry, op string, save func(PT) error) router.Route

// MountDelete registers op as the delete operation of a screen: it reads the ids
// the screen sends ({"ids":[…]}) and calls remove with them. No ids → 400.
//
//	view.MountDelete(reg, OpDeleteDevice, m.DeleteDevices).Requires(ResourceDevice, model.Delete)
func MountDelete(reg router.OperationRegistry, op string, remove func(ids []string) error) router.Route
```

- Server-side readers, unexported, next to the writers in `payload.go`:
  `saveIn[T, PT]` with `DecodeFields(r model.FieldReader)` (for each element of `records`:
  `p := PT(new(T)); arr.Object(i, p)`; append) and `deleteIn{ids []string}` reading `ids`.
- Errors (named, `model.Invalid`): `ErrNoRecords model.Invalid = "view: no records to save"`,
  `ErrNoIDs model.Invalid = "view: no ids given"`, `ErrCannotRead model.Invalid = "view: cannot read the request"`.
- `MountSave` returns `reg.Operation(op, handler).Accepts(PT(new(T)))`; `MountDelete` returns
  `reg.Operation(op, handler)`. The caller chains `.Requires(...)`.
- Delete answers 204 with no body (the screen decodes nothing).

## Stage 2 — `tests/mount_test.go` (new) — the real round trip

A test module (`OperationModule` with `ModelName()` and `MountOperations`) that mounts
`view.MountSave` and `view.MountDelete` over an in-memory slice of a test record type that has
`Item()` (reuse the record type the existing caller-lister tests use; grep `NewCallerLister` in
`tests/`). Build the client exactly as an app does:
`caller := view.NewRecords().Caller(loopback.ActingAs("t", "u", mod))` and
`lister := view.NewCallerLister(caller, view.Ops{Module: …, List: …, Save: …, Delete: …}, …)`.

| Test | Does | Expected |
|---|---|---|
| `TestMountSave_RoundTrip` | save two records through the lister | `save` called twice in order; the lister reports no error; the store has both |
| `TestMountSave_EmptyIs400` | call the op with `{"records":[]}` through `caller.Call` | `*rpc.Error` with `Status` 400 |
| `TestMountSave_ErrorKeepsItsStatus` | `save` returns `model.Conflict("dup")` | status 409, `Detail` `dup` |
| `TestMountDelete_RoundTrip` | delete two ids through the lister | `remove` got exactly those ids; no error |
| `TestMountDelete_EmptyIs400` | `{"ids":[]}` | status 400 |

If `loopback` does not translate a status into `*rpc.Error`, assert on whatever error it returns
and say so in the PR; do not add a fake.

## Stage 3 — docs

`README.md`: a section «Serving a screen» with the two calls and «the module never declares
`{records}`/`{ids}`». Update the comments in `payload.go` that say «nothing else in the ecosystem
should need to know it»: «MountSave/MountDelete read them on the server».

## Delivery check

```
gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission: when the check passes, commit, push and open the PR. Stuck 30
  minutes: open the PR listing what fails.
- `git diff --stat main` lists only: `mount.go`, `payload.go`, `tests/mount_test.go`, `README.md`,
  `go.mod`, `go.sum`, this plan.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `mount.go`, `payload.go`, `go.mod`, `go.sum` | compiles (stdlib and wasm) |
| 2 | `tests/mount_test.go` | 5 tests pass |
| 3 | `README.md`, `payload.go` comments | present |

## Executor notes
- `view.Ops` requires `List: "list"` to be instantiated without panicking, which was fixed in the tests.
- Modified tests to use correct function types for `lister.(view.Saver).Save(...)` and `.Call` with appropriate completion callbacks per the updated `webtyp.com/router` API.
