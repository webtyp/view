# Paged lists — what a module changes for view v0.6.63

`webtyp.com/view` v0.6.63 replaced "list everything, filter in the browser" with **paged lists declared by the
module and searched on the server**. A module that shows a list with `view.NewCallerLister` or implements
`view.Lister` makes the changes below. Contract: [page.go](../page.go), [lister.go](../lister.go),
[view.go](../view.go).

## What changed in view

| Before | Now |
|---|---|
| `Lister.List(done func(rows []view.Record, err error))` | `Lister.List(args view.ListArgs, done func(rows []view.Record, err error))` and `Lister.Page() view.Page` |
| The list operation got no body | It gets `view.ListArgs{Search, After, Limit}` and **must accept it** (otherwise it answers 400 and the screen lists nothing) |
| `Presenter.Filter(term) []model.Item` (local) | `Presenter.Search(term, done)`, `Presenter.HasMore()`, `Presenter.More(done)` (server) |
| — | `view.Ops.Page` (`view.Page{First, Size, More}`), `view.All`, `view.DefaultPage`, `view.MaxPage`, `view.Cap` |

## 1. The list operation accepts `view.ListArgs`

Every operation named in `view.Ops.List` declares and decodes `view.ListArgs`, caps the limit, searches on the
server and continues after the last row shown. Rows come in **ascending id order** (ids are unixid strings of equal
length, so string order is time order), the same order every call, so `After` continues it.

```go
reg.Operation(OpListThings, m.opListThings).Requires(model.ResourceOf(&ThingModel), model.Read).Accepts(&view.ListArgs{})

func (m *Module) opListThings(ctx router.Context) {
	var args view.ListArgs
	if err := ctx.Decode(&args); err != nil {
		rpc.WriteError(ctx, ErrCannotRead) // the module's existing "cannot read" error
		return
	}
	rows, err := m.listThings(args.Search, args.After, view.Cap(args.Limit))
	if err != nil {
		rpc.WriteError(ctx, err)
		return
	}
	var list ThingList
	for i := range rows {
		list = append(list, &rows[i])
	}
	_ = ctx.Encode(&list)
}

// listThings: tenant from the module (m.tenantID), never from the request.
func (m *Module) listThings(search, after string, limit int64) ([]Thing, error) {
	var t Thing
	qb := m.db.Query(&t).Where(Thing_.TenantId).Eq(m.tenantID)
	if search != "" {
		qb = qb.Where(Thing_.Name).Like("%" + search + "%") // the field marked Item: model.ItemLabel
	}
	if after != "" {
		qb = qb.Where(Thing_.Id).Gt(after)
	}
	qb = qb.OrderBy(Thing_.Id).Asc().Limit(int(limit))
	rows, err := ReadAllThing(qb)
	…
}
```

- **Never use `qb.Or()`**: `orm` does not group OR conditions, so `tenant = X AND a LIKE s OR b LIKE s` would leak
  rows of other tenants. Search one field (the `model.ItemLabel` one); a module that needs several fields picks the
  field by the shape of the term (e.g. a valid RUT → the RUT column, compared with `input.NormalizeRUT`).
- A model without a tenant column drops that `Where`. A model without `Id` orders by its primary key.
- If the operation already had arguments of its own (filters), those stay in the module's other operations; the
  view's list operation receives only `view.ListArgs`.

## 2. The module declares its Page

In `view.Ops`:

```go
view.Ops{Module: ModelName, List: OpListThings, Save: OpUpsertThing, Page: view.Page{First: view.All}}
```

- Small or bounded lists (catalogs, configuration, staff, devices, zones): `view.Page{First: view.All}` (every
  row up to `view.MaxPage` = 500, as today).
- Lists that grow without bound (patients, payments, messages, audit): `view.Page{First: 0, Size: 20, More: true}`
  (nothing until a search) or `view.Page{First: 50, Size: 50, More: true}`. The plan of each repo says which.

## 3. A hand-written Lister

```go
func (l *thingLister) Page() view.Page { return view.Page{First: view.All} }

func (l *thingLister) List(args view.ListArgs, done func([]view.Record, error)) { … same body, args may be ignored when Page is All … }
```

## 4. Tests

- Replace every `p.Filter(term)` with `p.Search(term, nil)` and then read `p.Items()`.
- A test double that implements `view.Lister` gets `Page()` and the new `List(args, done)`.
- Add `Test<Area>_ListAcceptsListArgs`: call the list operation with `&view.ListArgs{Limit: 1}` → status 200 and
  exactly 1 row; with `&view.ListArgs{Search: "<label of one seeded row>"}` → that row; with
  `&view.ListArgs{After: <id of the first row>}` → the first row is not in the result.
