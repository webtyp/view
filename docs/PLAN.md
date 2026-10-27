---
PLAN: "feat: view.Screen.Component — a screen that is not a list, in the same menu and with the same permission"
TAG: v0.6.46
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 7046896019275979829
---

> Read AGENTS.md at the repo root before any change.

# Plan — `webtyp.com/view`: screens of their own (`Screen.Component`)

## Why

A module declares its screens as data (`view.Screen`) and the app's renderer draws them, so modules never import
`webtyp/layout`. Today a leaf screen can only be a list with its form (`Presenter`). Some screens are not lists:
the booking wizard of `appointment_booking` (choose service → professional → hour → confirm) and the conversation of
`chat_room`. With no way to say «draw this component of mine», those modules either keep importing `layout` or
lose the screen. A leaf gets a second form: `Component`, exactly one of `Presenter` or `Component`.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/dom@v0.13.29
```

## Ecosystem search

- `dom.Component` (`webtyp.com/dom` v0.13.29, `interface.dom.go:67`) is the contract every drawable piece already
  satisfies (`GetID`, `SetID`, `String`, `Children`) → **reuse**. `view` does not depend on `dom` today; it will
  (`dom` builds for server and browser, so nothing changes for server binaries' platforms).
- `layout.NewUIModule(id, label, icon, view dom.Component, onActivate ...func())` (`webtyp/layout` `factory.go:9`)
  is what the renderer will wrap a `Component` in (layout's own plan).

## Design gate

1. **Prior art.** Django admin (a `ModelAdmin` list plus custom admin views in the same site), ActiveAdmin
   (`register` for resources, `register_page` for custom pages), react-admin (`<Resource>` plus `<CustomRoutes>`),
   Filament (Resources plus Pages): two kinds of screen, one menu, one permission check.
2. **Novice-name test.** `Screen{…, Component: NewBookingView(caller, ids, tenant)}` — «this screen is this
   component». Same word as `dom.Component`.
3. **Complexity ledger.** Concepts +1 (`Screen.Component`); ways to declare a screen −1 (modules stop keeping a
   `Browser()` beside `Screens()` for their non-list screens).
4. **Where it belongs.** `view` owns the screen contract (`Screen`, `Validate`); drawing stays in `layout/app`.
5. **What it deletes.** Nothing in this repo; it lets `appointment_booking`, `chat_room` and `clinical_encounter`
   delete `Browser()` and their `platformd` import.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `dom.Component` | `interface { GetID() string; SetID(id string); String() string; Children() []Component }` | dom v0.13.29 | `interface.dom.go` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — the field and its rules (`screen.go`)

1. Import `"webtyp.com/dom"`. Add the field after `Presenter`:

```go
	Component  dom.Component  // leaf only, instead of Presenter: a screen that is not a list (a wizard, a chat)
```

   Change the comments: `Permission` → `// leaf only: the resource the screen needs`; `Help` → `// group, or a
   Component leaf: what the screen is for`.

2. Rewrite the type comment's rule list:

```go
//   - list leaf: ID, Label, Icon, Permission and Presenter are required. Its help
//     text is the Presenter's Description() (view.WithDescription), never a
//     second copy here.
//   - component leaf: ID, Label, Icon, Permission, Component and Help are required;
//     no Presenter and no Lookups. For a screen that is not a list.
//   - group: ID, Label, Icon, Help and Children are required; it has no
//     Presenter, Component or Permission of its own — the menu shows it only when
//     the user may see at least one child, and opening it opens the first one.
```

3. Error constants — replace `errLeafNoPresenter` and `errGroupHasPresenter`, add three (verbatim):

```go
	errLeafNoPresenter      = "view: screen without Children needs a Presenter or a Component: "
	errLeafBoth             = "view: screen cannot have both a Presenter and a Component: "
	errComponentNoHelp      = "view: screen with a Component needs Help: "
	errComponentHasLookups  = "view: screen with a Component cannot have Lookups: "
	errGroupHasPresenter    = "view: screen with Children cannot have a Presenter, Component, Permission or Lookups: "
```

4. `Validate`, after the group branch (replace from `if s.Presenter == nil {` to the end):

```go
	if s.Presenter != nil && s.Component != nil {
		return fmt.Err(errLeafBoth + s.ID)
	}
	if s.Presenter == nil && s.Component == nil {
		return fmt.Err(errLeafNoPresenter + s.ID)
	}
	if s.Permission == "" {
		return fmt.Err(errLeafNoPermission + s.ID)
	}
	if s.Component != nil {
		if s.Help == "" {
			return fmt.Err(errComponentNoHelp + s.ID)
		}
		if len(s.Lookups) > 0 {
			return fmt.Err(errComponentHasLookups + s.ID)
		}
		return nil
	}
	if s.Help != "" {
		return fmt.Err(errLeafHasHelp + s.ID)
	}
	for _, l := range s.Lookups {
		if l.Field == "" || l.Source == nil {
			return fmt.Err(errLookupIncomplete + s.ID)
		}
	}
	return nil
```

   and the group check becomes `if s.Presenter != nil || s.Component != nil || s.Permission != "" || len(s.Lookups) > 0 {`.

## Stage 2 — tests (`tests/screen_test.go`, extend)

Add a helper in the test file: `type fakeComponent struct{ dom.Element }` (embedded **by value**, per `dom`'s rule)
and `func comp() dom.Component { return &fakeComponent{} }`.

- `TestScreen_ValidComponentLeaf` (new): `view.Screen{ID: "booking", Label: "Booking", Icon: "booking",
  Permission: "reservation", Help: "Book an appointment.", Component: comp()}` → `Validate() == nil`; and the same
  leaf as a child of a valid group → nil.
- `TestScreen_InvalidShapesNameTheScreen` (extend the table; update the two changed messages):

| name | screen | want |
|---|---|---|
| leaf without presenter | `{ID:"x",Label:"X",Icon:"x"}` | `needs a Presenter or a Component: x` |
| both | leaf `x` with `Component: comp()` | `both a Presenter and a Component: x` |
| component without help | component leaf without `Help` | `with a Component needs Help: x` |
| component with lookups | component leaf with one valid `Lookup` | `with a Component cannot have Lookups: x` |
| component without permission | component leaf with `Permission: ""` | `needs a Permission: x` |
| group with component | group `g` with `Component: comp()` | `cannot have a Presenter, Component, Permission or Lookups: g` |

## Stage 3 — docs

- `README.md`, row «Offer my module's screens…»: add «a screen that is not a list sets `Component` (a
  `dom.Component`) and `Help` instead of `Presenter`».
- `docs/SPECS.md`: wherever `Screen` is described, add the component leaf with its rules (same text as Stage 1.2).

## Stage 4 — translations

`langc sync .`, then fill every empty value it reports in `lang.json` (positional, the 8 languages of
`"languages"`), e.g. `"view: screen with a Component needs Help: ": ["view: una pantalla con Component necesita Help: ", …]`.
Zero untranslated keys and zero reuse warnings.

## Delivery

- `gotest` green. A PR that does not compile, or whose tests could not run, is a failed delivery.
  Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `screen.go`, `go.mod`, `go.sum` | `Component` leaf and its rules |
| 2 | `tests/screen_test.go` | valid and invalid shapes |
| 3 | `README.md`, `docs/SPECS.md` | document |
| 4 | `lang.json` | translate |
