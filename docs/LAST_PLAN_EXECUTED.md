---
PLAN: "feat!: listed records are model.Itemizer (generated Item()); Records keeps each item computed once; view.Item removed"
TAG: v0.6.48
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

<!-- GATE PLAN: executed LOCALLY, third of three (model v0.2.8 → ormc v0.1.26 → view). Needs webtyp.com/model v0.2.8. -->

# Plan — `webtyp.com/view`: one item type, computed once, checked by the compiler

## Why

v0.6.47 projects every row at runtime with `model.RowOf` and calls it to read ids inside loops
(`records.go:37,40,59,70`, `presenter.go:56,85`): O(N²) projections per list, re-run on every filter keystroke,
and a second type (`view.Item`) translated from `model.Row` (`view.go:18-21`). `webtyp.com/model` v0.2.8 has ONE
type, `model.Item`, and `ormc` v0.1.26 generates a typed `Item()` per listed model. Here:

- a listed record is `view.Record` = `model.Model` + `model.Itemizer` — `view.New` with a model that has no
  generated `Item()` does not compile;
- `Records` computes each record's item **once**, when it is put, and every read uses that cached item;
- `view.Item` and `itemOf` are deleted: everything speaks `model.Item`.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/model@v0.2.8
```

## Ecosystem search

`model.Item`, `model.Itemizer` (model v0.2.8) → reuse; this repo's `view.Item`, `itemOf` and every `model.RowOf`
call → deleted. Precedent: Apollo/Ember Data normalise on write and read cached entities.

## Design gate

1. **Prior art.** Apollo `InMemoryCache` and Ember Data compute an entity's identity and fields on write and serve
   reads from the cache; Go's own generated accessors (sqlc, protoc) over reflection.
2. **Novice-name test.** `view.Record` — «a record a list can show»; `Presenter.Record() view.Record`.
3. **Complexity ledger.** Types for one idea 2 → 1 (`view.Item` gone); projections per list O(N²) → N (on put);
   a model without `Item()` fails at compile time instead of at runtime.
4. **Where it belongs.** `view` owns the shared copy and the presenters; the item comes from `model` + `ormc`.
5. **What it deletes.** `view.Item`, `itemOf`, every `model.RowOf` call, `model.Model` as the presenter's record type.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `model.Item` | `struct { ID, Label, Description, Badge string }` | model v0.2.8 | `item.go` |
| `model.Itemizer` | `interface { Item() Item }` | model v0.2.8 | `item.go` |

## Stage 1 — the listed record (`view.go`, `lister.go`)

```go
// Record is a record a list can show: a model whose Item() ormc generates from the
// fields marked Item:… (model.ItemLabel, …). A model without marks has no Item()
// and cannot be listed — the compiler says so.
type Record interface {
	model.Model
	model.Itemizer
}
```

- Delete `type Item struct{…}` and `itemOf`. Every `Item` in this repo becomes `model.Item`
  (`Presenter.Items() []model.Item`, `Filter(term string) []model.Item`, `Action` payloads if any).
- `Presenter.Record() Record`; `New(l Lister, record Record, opts ...Option) Presenter`.
- `Lister.List(done func(rows []Record, err error))`.

## Stage 2 — `Records` keeps the item (`records.go`)

```go
type recordEntry struct {
	item model.Item // computed once, in Put
	rec  Record
}

type recordTable struct {
	name    string
	entries []recordEntry
	subs    []func()
}

// Put inserts or replaces rows by model name and Item().ID, computing each item once.
func (r *Records) Put(rows ...Record) {
	var touched []*recordTable
	for _, row := range rows {
		t := r.table(row.ModelName())
		e := recordEntry{item: row.Item(), rec: row}
		replaced := false
		for i := range t.entries {
			if t.entries[i].item.ID == e.item.ID {
				t.entries[i] = e
				replaced = true
				break
			}
		}
		if !replaced {
			t.entries = append(t.entries, e)
		}
		touched = appendTable(touched, t)
	}
	notify(touched)
}
```

`Remove`, `Get(modelName, id string) (Record, bool)` and `All(modelName string) []Record` compare
`entries[i].item.ID` (a string compare, never a projection). Add the unexported
`func (r *Records) items(modelName string) []recordEntry` for the presenter.

## Stage 3 — presenter and lister read cached items (`presenter.go`, `caller_lister.go`)

```go
func (p *core) Items() []model.Item {
	entries := p.lister.Records().items(p.record.ModelName())
	var fresh, listed []model.Item
	for _, e := range entries {
		if !containsID(p.order, e.item.ID) {
			fresh = append(fresh, e.item)
		}
	}
	for _, id := range p.order {
		for _, e := range entries {
			if e.item.ID == id {
				listed = append(listed, e.item)
				break
			}
		}
	}
	return append(fresh, listed...)
}
```

`Reload`: `p.order = append(p.order, row.Item().ID)` — or better, put first and read the ids from the entries
just put; either way `Item()` runs once per row per list. `callerLister.list` asserts each decoded row:

```go
		rec, ok := list.At(i).(Record)
		if !ok {
			done(nil, fmt.Err("view: Reload: row type", rowName(list.At(i)), "has no Item(): mark its fields with Item: model.ItemLabel and run ormc"))
			return
		}
```

`callerLister.save` asserts the decoded answer the same way before `Put`. Check:
`grep -rn "RowOf\|itemOf\|view.Item\b" --include=*.go .` → empty.

## Stage 4 — doubles and tests

- `conformance/conformance.go`: `MockRecord` implements `Item() model.Item` by hand (a test double, not generated):
  `return model.Item{ID: m.ID, Label: m.Name}`; drop its `Row:` marks. Every double's `List` delivers `[]view.Record`.
- Update every test to `model.Item`, `[]view.Record`.
- Add `TestRecords_ItemComputedOncePerPut`: a record type whose `Item()` counts its calls; `Put` 3 rows then call
  `Get`, `All`, `Items()` many times → the count is 3.
- Add `TestNew_ListedRecordMustBeItemizer` as a compile-level check in `conformance` (a `var _ view.Record =
  (*MockRecord)(nil)` line) — no runtime test needed.

## Stage 5 — docs and translations

`README.md`, `docs/SPECS.md`: `view.Record`, `model.Item`, items computed once on put; no `RowOf`, no `view.Item`.
`langc sync .`; fill every empty value; zero untranslated keys, zero reuse warnings.

## Delivery

- `gotest` green. No `map`, no stdlib `fmt`/`strings`/`errors` in non-test code (WASM).
- Commit after every stage; never `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `view.go`, `lister.go`, `go.mod`, `go.sum` | `view.Record`; `view.Item` gone |
| 2 | `records.go` | entries with the cached item |
| 3 | `presenter.go`, `caller_lister.go` | read cached items; assert `Record` |
| 4 | `conformance/*`, `tests/*` | doubles, once-per-put test |
| 5 | `README.md`, `docs/SPECS.md`, `lang.json` | document, translate |
