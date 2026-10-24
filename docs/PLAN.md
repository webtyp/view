---
PLAN: "feat(lang): view ships its dictionary — lang.json with every language of lang.Supported() filled"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 12274797605227864188
PR: https://github.com/webtyp/view/pull/8
REVIEWED_COMMIT: 0b98c0978da69aa46ab207441360c8215303ab0c
---

> Read AGENTS.md at the repo root before any change.

# PLAN — `view` dictionary

## Why

`view` returns user-facing errors (`fmt.Err`) but has no `lang.json`, so they always show in
English. Ecosystem rule (LANG_SUPPORT, closed): every library seeds `lang.Supported()` minus `en`.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

## Stage 1 — `lang.json`

1. Run `langc sync .` at the repo root. It creates `lang.json` with every key and one column per
   language of `lang.Supported()` except `en`. Format sample (keep exactly this shape):

   ```json
   {
     "languages": ["es", "zh", "hi", "ar", "pt", "fr", "de", "ru"],
     "keys": {
       "view: screen has no ID": ["view: la pantalla no tiene ID", "…", "…"],
   ```
2. Fill **every** empty value in every column with a correct translation. Keep the prefix
   `view:` untranslated, and keep `%s`/`%v` placeholders and trailing `: ` exactly as in the key.
3. `langc sync .` also prints reuse warnings (two keys sharing a phrase, e.g. `view: Update
   requires at least one field` / `… id`). Do **not** change any Go string for them in this plan;
   list them in the PR description.
4. Run `langc sync .` again: it must report no empty value.

## Delivery check

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission or report progress: when the check passes, commit, push and open
  the PR. Stuck for 30 minutes: open the PR anyway listing what fails.
- `git diff --stat main` lists only `lang.json` (new) and this plan.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `lang.json` (new) | no empty value in any column |

## Executor notes

Completed translation of `lang.json` with all required values for `es`, `zh`, `hi`, `ar`, `pt`, `fr`, `de`, `ru`.

`langc sync .` generated the following reuse warnings which are acceptable per instructions:
```
lang: does not implement model.Model and does not implement view.Itemizer share does not implement — extract it as its own key and reuse it, see lang.json
lang: view: Delete requires at least one id and view: Update requires at least one id share requires at least one id — extract it as its own key and reuse it, see lang.json
lang: view: Save payload is nil and view: Save requires at least one record share view: Save — extract it as its own key and reuse it, see lang.json
lang: view: Save payload is nil and view: Update record is nil share is nil — extract it as its own key and reuse it, see lang.json
lang: view: Update record is nil and view: Update requires at least one field share view: Update — extract it as its own key and reuse it, see lang.json
lang: view: Update requires at least one field and view: Update requires at least one id share view: Update requires at least one — extract it as its own key and reuse it, see lang.json
```
