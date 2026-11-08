---
PLAN: "feat: view owns the module-to-chassis contracts — Notifier, BrowserDeps, ModuleBrowser, EntryScreens, Demo"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally** and is **published as soon as the delivery check passes** (no `replace` anywhere:
> every dependency is a published version). Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: `view` holds every contract between a module and the chassis

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/ddl@v0.0.46 webtyp.com/msgtype@v0.1.0 webtyp.com/time@v0.6.4
```

## Why

A module's `go.mod` lists `webtyp.com/layout` (the chassis, which changes every week) only because the
contracts it implements live there (`layout.Notifier`) or in `appkit`, which imports `layout`
(`appkit.BrowserDeps`). After this plan, those contracts live here; `layout` (plan L2) and `appkit`
(plan L3) import them from `view`. Decisions D15, D16, D17 of `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`.

## Ecosystem search

- `layout/capabilities.go`: `Duration`, `Auto`, `Persistent`, `For`, `Notifier`, `UsesNotifier` — **moved** here
  verbatim (the only ones a module uses: `room_layout/ui/data.go`). `Badged`, `Described`, `Step`, `AlertedModule`,
  `GroupModule`, `UIModule` stay in `layout`: no module implements them.
- `appkit/module_browser.go`: `BrowserDeps`, `ModuleBrowser`, `EntryScreens` — **moved** here; `BrowserDeps` loses
  `Brand` and `EntryScreens` carries data instead of finished elements (D17).
- `Demo`, `Entry`: **new**. Closest existing: each module's hand-written `web/browser/main.go` (replaced by `Demo`)
  and `layout/login.Login` (the chassis keeps it; `Entry` is only what the module fills).

## Design gate

1. **Prior art:** Django admin (`ModelAdmin` declares, the admin site draws — https://docs.djangoproject.com/en/5.1/ref/contrib/admin/),
   Storybook (a component declares stories, the tool supplies the shell — https://storybook.js.org/docs/writing-stories),
   React Admin (`<Resource>` declares, `<Admin>` lays out — https://marmelab.com/react-admin/Resource.html). All three:
   the piece declares data, the host owns the frame. Same here.
2. **Names:** `view.Notifier`, `view.BrowserDeps`, `view.ModuleBrowser`, `view.EntryScreens` keep their current names;
   `view.Entry` (one entry screen), `view.Demo` (a module's demo) read as plain English.
3. **Ledger:** concepts ±0 (moved) +2 (`Entry`, `Demo`); files a module touches for its demo −1 (`web/browser/main.go`
   gone, `seed/demo.go` added); lines at the call site −40 to −100 per demo; ways to do the same thing −1.
4. **Where:** `view` is the repository that already declares itself the view contract (`README.md`).
5. **Deletes:** the copies in `layout` and `appkit` (plans L2, L3).

## APIs used

| Symbol | Signature | Tag |
|---|---|---|
| `msgtype.Type` | `type Type uint8` | `msgtype v0.1.0` |
| `router.Caller` | `interface{ Call(op string, args, reply model.Encodable, done func(error)) }` | `router v0.4.19` |
| `time.Zone` | value returned by `time.LoadZone(name string) (time.Zone, error)` | `time v0.6.4` |
| `crud.Module` | `interface{ model.ModuleNaming; MountOperations(reg crud.Registry) }` | `crud v0.0.3` |
| `ddl.Step` | `struct{ Name string; Run func(conn ddl.Execer, compiler ddl.Compiler) error }` | `ddl v0.0.46` |
| `dom.Component`, `*dom.SignalString` | as in `dom v0.13.29` | |

If a signature in the published tag differs from this table, stop and name the symbol; never adapt it by guessing.

## Stage 1 — `notify.go` (new, no build tag)

Copy from `https://github.com/webtyp/layout/blob/main/capabilities.go` exactly: type `Duration` and its methods
`IsAuto`, `HasTimer`, `Millis`; functions `Auto`, `Persistent`, `For`; interfaces `Notifier`, `UsesNotifier`. Change only
the package name and the doc comments' `layout.` → `view.`. Imports: `webtyp.com/fmt`, `webtyp.com/msgtype`.

```go
type Notifier interface {
	Notify(t msgtype.Type, msg string, d Duration)
}

// UsesNotifier is a screen component that tells the user «saved» or an error through the chassis.
//
//	func (r *RootView) UseNotifier(n view.Notifier) { r.notifier = n }
type UsesNotifier interface {
	UseNotifier(n Notifier)
}
```

## Stage 2 — `browser.go` (new, no build tag)

```go
// BrowserDeps is what every module's screens get in the browser. A new need becomes a field here, never a parameter.
//
//	Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }
type BrowserDeps struct {
	Caller router.Caller
	Zone   time.Zone
}

// ModuleBrowser is what a module adds to the browser. Zero on the server.
type ModuleBrowser struct {
	Screens func(d BrowserDeps) []Screen
	Entry   func(d BrowserDeps) EntryScreens // only the entry module (webtyp/access) sets it
}

// EntryScreens are what the browser shows before the menu. The chassis draws them with the app's name and logo.
// IdleSeconds locks an idle session (0 = 30 minutes).
type EntryScreens struct {
	Login       Entry
	Setup       Entry // shown while the server has no administrator
	IdleSeconds int
}

// Entry is one entry screen as the module fills it: the line under the title, the form, and the message line.
//
//	view.Entry{Subtitle: "Enter your key to continue", Form: loginForm.Render(), Message: loginMessage}
type Entry struct {
	Subtitle lang.Text
	Form     dom.Component
	Message  *dom.SignalString
}
```

## Stage 3 — `demo.go` (new, no build tag)

```go
// Demo is a module's runnable demo: the module built on storage/mem with its seed data, and its screens.
// The module returns it from seed.Demo(); the webtyp daemon draws it with the chassis. The module never imports layout.
//
//	return view.Demo{Label: ui.DefaultLabel, Timezone: "America/Santiago", ActingAs: "u1",
//		Modules: []crud.Module{mod}, Migrations: []ddl.Step{{Name: chatroom.ModelName, Run: migrate.Migrate}},
//		Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }}, nil
type Demo struct {
	Label      string                       // required: the chassis titles the page "<Label> — demo"
	Timezone   string                       // required: IANA zone given to the screens as BrowserDeps.Zone
	ActingAs   string                       // the user id operations see in crud.Caller; "" = signed out
	Modules    []crud.Module                // required: the operations the screens call
	Migrations []ddl.Step                   // tables shown in «Database»; nil = no diagram
	Screens    func(d BrowserDeps) []Screen // required
}

// Validate reports the first missing required field.
func (d Demo) Validate() error {
	switch {
	case d.Label == "":
		return ErrDemoNoLabel
	case d.Timezone == "":
		return ErrDemoNoTimezone
	case len(d.Modules) == 0:
		return ErrDemoNoModules
	case d.Screens == nil:
		return ErrDemoNoScreens
	}
	return nil
}
```

Errors, in the file that already holds this package's errors (`fmt.Err`, translated by `langc`):

```go
var (
	ErrDemoNoLabel    = fmt.Err("view.Demo has no Label: the page would have no title; set Label to the module's name")
	ErrDemoNoTimezone = fmt.Err("view.Demo has no Timezone: dates cannot be shown; set Timezone, e.g. \"America/Santiago\"")
	ErrDemoNoModules  = fmt.Err("view.Demo has no Modules: every screen would fail to load; add the module built in seed.Demo")
	ErrDemoNoScreens  = fmt.Err("view.Demo has no Screens: the menu would be empty; set Screens to the module's ui.Screens")
)
```

## Stage 4 — tests (`tests/`, rows in `docs/TESTS.md`)

| Test | File | Setup → call → expected |
|---|---|---|
| `TestNotify_AutoSizesToWords` | `tests/notify_test.go` | `Auto().Millis("ok")` → `4000`; 40 words → `10000`; `Auto().IsAuto()` → true |
| `TestNotify_ForAndPersistent` | `tests/notify_test.go` | `For(1500).Millis("x")` → `1500`; `Persistent().HasTimer()` → false, `.Millis("x")` → `0` |
| `TestDemo_ValidateNamesFirstMissingField` | `tests/demo_test.go` | table: zero `Demo` → `ErrDemoNoLabel`; Label only → `ErrDemoNoTimezone`; + Timezone → `ErrDemoNoModules`; + one fake `crud.Module` → `ErrDemoNoScreens`; complete → `nil` |

The fake `crud.Module` is a struct in the test file with `ModelName() string` and an empty `MountOperations(crud.Registry)`.

## Stage 5 — translations and documentation

`langc sync .`, fill every empty value. `README.md`: add rows to «I want to… → use»: «Tell the user saved/error from a
custom screen → `view.UsesNotifier`», «Give my module a runnable demo → `seed.Demo() (view.Demo, error)`», «Be the entry
module (sign-in) → `ModuleBrowser.Entry` returning `view.EntryScreens`». `docs/ARCHITECTURE.md`: one paragraph «Module ↔
chassis contracts» naming the five types and stating that `view` never imports `layout` or `appkit`.

## Delivery check

```bash
gotest
langc sync .   # zero untranslated, zero reuse warnings
grep -rn "webtyp.com/layout\|webtyp.com/appkit" --include=*.go .   # empty
```

A PR that does not compile, or whose tests could not run, is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `notify.go` |
| 2 | `browser.go` |
| 3 | `demo.go`, errors file |
| 4 | `tests/notify_test.go`, `tests/demo_test.go`, `docs/TESTS.md` |
| 5 | `lang.json`, `README.md`, `docs/ARCHITECTURE.md` |
