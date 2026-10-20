---
PLAN: "feat: WithDescription and WithEmpty — a view explains itself and its empty state"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change. This plan is complete: do not guess; if something does
> not fit, stop and ask in the PR. Design history: `webtyp/docs/MODULE_GUIDANCE_MASTER_PLAN.md` (D4, closes
> P4) and the rule in `devskills/skills/ui-consistency/SKILL.md` ("A module explains itself").

## Goal

A screen must be usable by someone never told how it works. Today `Presenter` has no way to say **what this
list is for** when nothing is selected, or **what to do** when the list is empty. Add two `Option`s mirroring
`WithTitle`'s exact shape, and two matching `Presenter` methods.

## Git rules (strict)

- Work on the branch codejob created. Never `git reset`/`checkout -- <file>`/`clean`/`stash`. Commit often.
- **Never pause to ask permission or to report progress.** Delivery check passes → commit, push, open the PR.
  Still failing after 30 minutes → open the PR anyway, list exactly what fails.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Stage 1 — `view.go`

Add to `config` (line ~113):

```go
type config struct {
	title             string
	searchPlaceholder lang.Text
	description       lang.Text
	emptyMessage      lang.Text
}
```

Add two options, immediately after `WithSearchPlaceholder`:

```go
// WithDescription sets what this view is for, shown when no record is selected.
func WithDescription(d lang.Text) Option {
	return func(c *config) { c.description = d }
}

// WithEmpty sets what to show, and what to do, when the list has no records yet.
func WithEmpty(msg lang.Text) Option {
	return func(c *config) { c.emptyMessage = msg }
}
```

Add to `Presenter` (after `SearchPlaceholder() lang.Text`):

```go
// Description is what this view is for, shown when no record is selected. "" when WithDescription was not used.
Description() lang.Text

// EmptyMessage is what to show, and what to do, when Items() is empty. "" when WithEmpty was not used.
EmptyMessage() lang.Text
```

In `New`, copy both into `core` (same line as `title`/`searchPlaceholder`), and add the two methods on `*core`
(next to the existing `Title()`/`SearchPlaceholder()` methods — find them by grepping for `func (c *core)
Title`):

```go
func (c *core) Description() lang.Text  { return c.description }
func (c *core) EmptyMessage() lang.Text { return c.emptyMessage }
```

Every capability wrapper (`crud`, `saveable`, `updatable`, `deletable`, and their combinations) embeds `*core`
— no further change needed there; they inherit both methods for free, exactly like `Title()` today.

## Stage 2 — tests

Through the exported API (`tests/`, package `view_test`, a minimal fake `Lister`):

- `WithDescription`/`WithEmpty` unset → both methods return the empty `lang.Text`.
- Set → both return exactly what was passed, for a plain `Presenter` and for one with every capability
  (`Saver`+`Updater`+`Deleter`) — confirms the embedding path on every wrapper type, not just `core` alone.

## Stage 3 — docs

`README.md`: add both options next to `WithTitle`'s own documentation, with one line each on when a module
should set them (a list that has a real "why it's useful" story, a list that can start truly empty).

## Delivery check

- `gotest` green (vet, race, tests, wasm).
- `git diff --stat main --diff-filter=D` empty — nothing deleted.
- No behaviour change for any existing caller that does not use the two new options.

## Stages

| # | File(s) | What |
|---|---|---|
| 1 | `view.go` | `config` fields, `WithDescription`/`WithEmpty`, `Presenter` methods, `*core` implementation |
| 2 | `tests/*` | unset/set behaviour across capability combinations |
| 3 | `README.md` | document the two options |
