# Architecture: webtyp.com/view

## Module ↔ chassis contracts

`view` holds every contract between a domain module and the application chassis:
`Notifier` / `UsesNotifier` (and `Duration`), `BrowserDeps`, `ModuleBrowser`, `EntryScreens` (with `Entry`), and `Demo`.
Domain modules implement and declare these contracts without importing any concrete chassis or renderer.
`view` never imports `layout` or `appkit`.
