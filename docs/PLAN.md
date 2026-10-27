---
PLAN: "feat!: one shared copy of each record (view.Records); rows from model.RowOf; Itemizer and Lookup removed"
TAG: v0.6.47
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 13760651529966002640
PR: https://github.com/webtyp/view/pull/10
REVIEWED_COMMIT: 13e85c20c27a7d203bdc057a6eadfd110f941b6c
---

> Read AGENTS.md at the repo root before any change.

<!-- PARKED: becomes docs/PLAN.md when the running plan (Screen.Component, v0.6.46) closes. Dispatch only after
     webtyp.com/view v0.6.46 and webtyp.com/model v0.2.7 are published. -->

# Plan — `webtyp.com/view`: what is saved shows everywhere (shared records)

## Why

Every presenter keeps its own copy of the records (`core.items`/`core.index`, `presenter.go:26-37`). Two screens
that show the same table — or a screen and a select fed by that table — hold two copies, and saving through one
never changes the other: the person saves, sees «Guardado», and the other place keeps the old text. From now on a
page has **one** copy of each record, `view.Records`, keyed by model name and id; every presenter reads it and is
told when it changes. The list row comes from the fields (`model.RowOf`, `webtyp.com/model` v0.2.7), not from a
hand-written `Item()`. Breaking change, no compatibility layer (the framework has no external consumers).

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/model@v0.2.7
```

If `model@v0.2.7` lacks `model.RowOf`, `model.Row`, `Field.Row` or `Field.Options`, stop and say so in the PR.

## Ecosystem search

- `model.RowOf(f model.Fielder) model.Row` and `model.Row{ID, Title, Detail, Badge}` (model v0.2.7) → **reuse**:
  replaces `Itemizer`/`Item()`.
- `model.FielderSlice.Append() model.Fielder` (model `interface.go:26-30`) → **reuse** to make a fresh record for a
  save response.
- `dbsync.Client.Caller()` wraps a `router.Caller` the same way `Records.Caller` does (precedent for the shape).
- No in-memory record store exists in `webtyp/*` (`storage` is the DB port, `kvdb` string KV, `dbsync` offline
  persistence) → **new** type here, replacing the per-presenter index this repo already owns.

## Design gate

1. **Prior art.** Apollo Client `InMemoryCache` (normalised by type + id; every query reading an entity re-renders
   when it changes), Ember Data `store`, Redux Toolkit `createEntityAdapter`, Relay store.
2. **Novice-name test.** `records := view.NewRecords()`; `caller := records.Caller(rpc.NewCaller(""))` — «the
   records of this page», «a caller that keeps them». `presenter.OnChange(fn)` — «tell me when it changes».
3. **Complexity ledger.** Concepts +2 (`Records`, `OnChange`) / −3 (`Itemizer`, `Lookup`, the per-presenter index);
   module code −1 method per listed model (`Item()`); ways a row is built −1.
4. **Where it belongs.** `view` owns «what the screens show»; the store is that same concern, shared.
5. **What it deletes.** `Itemizer`, `Item.LeadTop/LeadMain/LeadBottom`, `Screen.Lookups`, `Lookup`,
   `indexEntry`, `core.items`, `core.index`, the `Itemizer` error path of `Reload`.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `model.RowOf` | `func RowOf(f Fielder) Row` | model v0.2.7 | `row.go` |
| `model.Row` | `struct { ID, Title, Detail, Badge string }` | model v0.2.7 | `row.go` |
| `model.Model` | `Fielder + ModuleNaming + Encodable + Decodable` | model v0.2.7 | `interface.go` |
| `model.FielderSlice` | `Len() int; At(i int) Fielder; Append() Fielder` | model v0.2.7 | `interface.go` |
| `router.Caller` | `Call(op string, args model.Encodable, into model.Decodable, done func(error))` (read it in `webtyp.com/router` `caller.go` at the version in `go.mod`) | router | `caller.go` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — `records.go` (new): the shared copy

```go
package view

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

// Records is the one in-memory copy of every record the screens of a page have
// loaded, keyed by model name and id. Every presenter built over the same Records
// reads it, so what is saved through one screen shows at once in every other place
// that displays it (a list row, a select). Build ONE per page (appkit does it):
//
//	records := view.NewRecords()
//	caller := records.Caller(rpc.NewCaller(""))
type Records struct {
	tables []*recordTable // a slice, not a map: this file compiles to WASM
}

type recordTable struct {
	name string
	rows []model.Model
	subs []func()
}

// NewRecords returns an empty copy for one page.
func NewRecords() *Records { return &Records{} }

// Caller wraps c so every lister built over it (NewCallerLister) shares these Records.
func (r *Records) Caller(c router.Caller) router.Caller { return &recordsCaller{Caller: c, records: r} }

// Put inserts or replaces rows by model name and id (model.RowOf(row).ID), then
// tells each touched model's subscribers once.
func (r *Records) Put(rows ...model.Model) {
	var touched []*recordTable
	for _, row := range rows {
		t := r.table(row.ModelName())
		id := model.RowOf(row).ID
		replaced := false
		for i, cur := range t.rows {
			if model.RowOf(cur).ID == id {
				t.rows[i] = row
				replaced = true
				break
			}
		}
		if !replaced {
			t.rows = append(t.rows, row)
		}
		touched = appendTable(touched, t)
	}
	notify(touched)
}

// Remove drops the rows of modelName with the given ids and tells its subscribers.
func (r *Records) Remove(modelName string, ids ...string) {
	t := r.table(modelName)
	kept := t.rows[:0]
	for _, cur := range t.rows {
		if !containsID(ids, model.RowOf(cur).ID) {
			kept = append(kept, cur)
		}
	}
	t.rows = kept
	notify([]*recordTable{t})
}

// Get returns the record of modelName with id.
func (r *Records) Get(modelName, id string) (model.Model, bool) {
	for _, cur := range r.table(modelName).rows {
		if model.RowOf(cur).ID == id {
			return cur, true
		}
	}
	return nil, false
}

// All returns every record of modelName, in the order they were first put.
func (r *Records) All(modelName string) []model.Model { return r.table(modelName).rows }

// OnChange registers fn to run after any Put or Remove that touches modelName.
func (r *Records) OnChange(modelName string, fn func()) {
	t := r.table(modelName)
	t.subs = append(t.subs, fn)
}

func (r *Records) table(name string) *recordTable {
	for _, t := range r.tables {
		if t.name == name {
			return t
		}
	}
	t := &recordTable{name: name}
	r.tables = append(r.tables, t)
	return t
}

func appendTable(ts []*recordTable, t *recordTable) []*recordTable {
	for _, cur := range ts {
		if cur == t {
			return ts
		}
	}
	return append(ts, t)
}

func notify(ts []*recordTable) {
	for _, t := range ts {
		for _, fn := range t.subs {
			fn()
		}
	}
}

func containsID(ids []string, id string) bool {
	for _, cur := range ids {
		if cur == id {
			return true
		}
	}
	return false
}

// recordsCaller is the router.Caller Records.Caller returns; NewCallerLister reads
// the Records from it.
type recordsCaller struct {
	router.Caller
	records *Records
}
```

## Stage 2 — `Lister` and `Item` (`lister.go`, `view.go`)

1. `Lister` gains the shared copy (every lister, also test doubles):

```go
type Lister interface {
	List(done func(rows []model.Model, err error))
	// Records is the page's shared copy this lister's rows live in. A lister puts
	// the record a Save answered with and removes deleted ids itself (Stage 3);
	// view puts the rows of List.
	Records() *Records
}
```

   Rewrite its doc: «view projects them through `model.RowOf`» (not Itemizer).

2. `Item` (`view.go`): delete `LeadTop`, `LeadMain`, `LeadBottom` and their comment; add
   `Badge string // the leading badge (model.RowBadge field), e.g. a date or an hour; "" = none`. Delete the
   `Itemizer` interface. Add the projection:

```go
// itemOf projects a record as a list row from its fields (model.RowOf).
func itemOf(rec model.Model) Item {
	r := model.RowOf(rec)
	return Item{ID: r.ID, Label: r.Title, Description: r.Detail, Badge: r.Badge}
}
```

3. `Presenter` gains:

```go
	// OnChange registers fn to run after any record of this presenter's model
	// changes in the shared Records (a save from any screen, a sync). A renderer
	// re-reads Items() inside fn.
	OnChange(fn func())
```

   and `Items()`' comment becomes `// the rows of this presenter's model, newest first`.

## Stage 3 — `NewCallerLister` (`caller_lister.go`)

1. `NewCallerLister` requires a caller from `Records.Caller` (verbatim panic):

```go
	rc, ok := c.(*recordsCaller)
	if !ok {
		panic("view: NewCallerLister: caller must come from view.Records.Caller — build it once per page: view.NewRecords().Caller(c)")
	}
```

   Store `records: rc.records` in `callerLister` and add `func (b *callerLister) Records() *Records { return b.records }`.

2. `save` decodes the saved record the server answers and puts it:

```go
func (b *callerLister) save(recs []model.Model, done func(error)) {
	saved, ok := b.newList().Append().(model.Model)
	if !ok {
		done(fmt.Err("view: list returned by newList", "does not implement", typeName("model.Model")))
		return
	}
	b.caller.Call(b.ops.Save, &saveArgs{recs: recs}, saved, func(err error) {
		if err == nil {
			b.records.Put(saved)
		}
		done(err)
	})
}
```

3. `delete` removes on success: inside the callback, `if err == nil { b.records.Remove(b.ops.Module …) }` — use the
   model name of the presenter's records: add a `modelName string` to `callerLister`, filled in `NewCallerLister`
   from `newList().Append().(model.Model).ModelName()`, and call `b.records.Remove(b.modelName, ids...)`.

4. `update` (bulk patch) is unchanged: crudview reloads after a bulk edit.

## Stage 4 — the presenter reads the shared copy (`presenter.go`)

Replace `items`, `index` and `indexEntry` with an id order:

```go
type core struct {
	lister            Lister
	record            model.Model
	title             string
	searchPlaceholder lang.Text
	description       lang.Text
	emptyMessage      lang.Text

	order    []string // ids of the last List, in its order; ids put later go first
	selected string
}
```

```go
func (p *core) Reload(done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	p.lister.List(func(rows []model.Model, err error) {
		if err != nil {
			done(err)
			return
		}
		p.order = p.order[:0]
		for _, row := range rows {
			p.order = append(p.order, model.RowOf(row).ID)
		}
		p.lister.Records().Put(rows...)
		done(nil)
	})
}

// Items: the rows of this model in the shared copy — those this presenter never
// listed (saved here or elsewhere, synced) first, then the listed ones in List order.
func (p *core) Items() []Item {
	all := p.lister.Records().All(p.record.ModelName())
	var fresh, listed []Item
	for _, rec := range all {
		if !containsID(p.order, model.RowOf(rec).ID) {
			fresh = append(fresh, itemOf(rec))
		}
	}
	for _, id := range p.order {
		if rec, ok := p.lister.Records().Get(p.record.ModelName(), id); ok {
			listed = append(listed, itemOf(rec))
		}
	}
	return append(fresh, listed...)
}

func (p *core) OnChange(fn func()) { p.lister.Records().OnChange(p.record.ModelName(), fn) }

func (c *core) lookup(id string) (model.Model, bool) {
	return c.lister.Records().Get(c.record.ModelName(), id)
}
```

`Filter` iterates `p.Items()` instead of `p.items`. `rowName` and the `Itemizer` error key go (check
`grep -rn "Itemizer\|indexEntry\|p.items\|c.index" --include=*.go .` → empty). Add `OnChange` to every capability
wrapper type that needs it (they embed `*core`, so it is promoted — verify every wrapper still satisfies `Presenter`).

## Stage 5 — `Screen` without `Lookups` (`screen.go`)

Delete `Screen.Lookups`, the `Lookup` type, `errLookupIncomplete`, the Lookups checks in `Validate`
(`errGroupHasPresenter` becomes `"view: screen with Children cannot have a Presenter, Component or Permission: "`;
`errComponentHasLookups` goes), and the `Lookups` line of the `Screen` doc example. A select is filled from the
field's `Ref`/`Options` (model v0.2.7) by the renderer.

## Stage 6 — doubles and tests

- `conformance/conformance.go`: `MockRecord` loses `Item()`; its `Schema()` marks `name` with `Row: model.RowTitle`
  and `id` as PK (`DB: &model.FieldDB{PK: true}`); every double (`FakeLister`, `listOnlyLister`, `listSaveLister`,
  `deferredLister`) gets a `records *view.Records` field defaulting to `view.NewRecords()` when nil and a
  `Records()` method; `FakeLister.Save` puts the saved records (`b.Records().Put(recs...)`) and `Delete` removes
  them, like a real lister. Any clause that asserted `Description == "Desc of …"` asserts `""` (no RowDetail).
- New `tests/records_test.go`:

| Test | Setup → call | Expected |
|---|---|---|
| `TestRecords_PutReplacesByID` | put `{id a, name A}`, then `{id a, name B}` | `All` has 1 row, `name B` |
| `TestRecords_OnChangeOncePerPut` | subscribe; `Put(r1, r2)` same model | fn ran once |
| `TestRecords_Remove` | put a, b; `Remove(model, "a")` | only b; subscribers told |
| `TestPresenters_ShareRecords` | two presenters over two listers sharing one `Records`; Reload both; save a renamed record through the first | the second presenter's `Items()` shows the new label without reloading, and its `OnChange` fn ran |
| `TestPresenter_SavedRowFirst` | Reload [a, b]; save new c | `Items()` = [c, a, b] |
| `TestNewCallerLister_RequiresRecordsCaller` | `NewCallerLister(plainMockCaller, …)` | panics with the message of Stage 3 |
| `TestCallerLister_SavePutsServerAnswer` | caller answers the save with `{id x, name "Server name"}` | `Records().Get(model, "x")` has `"Server name"` |

Existing tests that build listers or presenters are updated to the new shapes (never deleted unless they pinned a
removed symbol: `Itemizer`, `Lookup`, `Lead*`); list which ones in the PR.

## Stage 7 — docs and translations

- `README.md` and `docs/SPECS.md`: the row comes from `Row:` on the model's fields (`model.RowOf`); one
  `view.NewRecords()` per page, `records.Caller(c)` for every lister; `Presenter.OnChange`; a select fills from the
  field's `Ref`/`Options`. Remove every mention of `Itemizer`, `Item()`, `Lookup`, `Lead*`.
- `langc sync .` and fill every empty value; zero untranslated keys, zero reuse warnings.

## Delivery

- `gotest` green. A PR that does not compile, or whose tests could not run, is a failed delivery.
  Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.
- No `map`, no stdlib `fmt`/`strings`/`errors` in non-test code (WASM).
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `records.go` | `Records`, `Put`, `Remove`, `Get`, `All`, `OnChange`, `Caller` |
| 2 | `lister.go`, `view.go` | `Lister.Records`; `Item.Badge`; `Itemizer` gone; `Presenter.OnChange` |
| 3 | `caller_lister.go` | requires `Records.Caller`; save puts the answer; delete removes |
| 4 | `presenter.go` | reads the shared copy |
| 5 | `screen.go` | `Lookups` gone |
| 6 | `conformance/*`, `tests/*` | doubles and behaviour tests |
| 7 | `README.md`, `docs/SPECS.md`, `lang.json` | document, translate |
