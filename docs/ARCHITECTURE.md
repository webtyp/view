# Architecture: webtyp.com/view

## 1. Overview in One Minute

`view` defines the UI-agnostic contract between domain models, storage or network backends, and rendering engines.
Every operation a view calls is a typed `router.Operation`:
- The four CRUD operations (`Read`, `Create`, `Update`, `Delete`) are derived automatically from the model: `router.Operation{Module: Module, Entity: ModelName, Action: Action}`.
- Custom list-wide actions (`view.Action`) carry their own typed `router.Operation`, representing a CRUD operation on another model (a different permission is its own model).

`view` also holds the layout contract (`Brand`, `Session`, `Layout`): app and layout exchange only `view` types, so neither imports the other and the application picks its layout at runtime.

## 2. Repository Pieces & On-Disk Layout

| File | Responsibility | Owner |
|---|---|---|
| `presenter.go` | UI-agnostic core presenter (`Presenter`, `Saver`, `Updater`, `Deleter`) | view |
| `caller_lister.go` | Remote backend adapter (`NewCallerLister`) invoking `router.Operation` | view |
| `action.go` | List-wide action definitions (`Action`, `Actioner`, `ActionRunner`) | view |
| `session.go` | Layout contract between app and renderer (`Brand`, `Session`, `Layout`) | view |
| `screen.go`, `browser.go` | Navigation menu items and entry screen definitions (`Screen`, `EntryScreens`) | view |
| `records.go` | Shared client-side reactive store for loaded records | view |
| `demo.go` | Standalone module demo contract | view |

## 3. Step-by-Step Flows

### Operations Invocation Flow
1. A user interaction triggers a read, save, update, delete, or list-wide action.
2. The presenter validates permissions/records and dispatches a typed `router.Operation` to the backend.
3. The backend (`callerLister`) sends the wire request through `router.Caller`.
4. Asynchronous completion runs the `done(error)` callback; on success, `Reload` refreshes the list in place.

## 4. Security Boundaries & Contracts

- **No string operations**: Callers address operations strictly through `router.Operation{Module, Entity, Action}`.
- **Capabilities mirror model permissions**: Write capabilities (`Saver`, `Updater`, `Deleter`) exist only if the model's `Allow()` explicitly permits `Create`, `Update`, or `Delete`.
- **Zero transport imports**: Modules never import `layout` or `appkit`.
