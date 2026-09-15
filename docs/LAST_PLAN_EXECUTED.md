---
PLAN: "refactor!: Lister/Saver/Updater/Deleter/Reload take a callback instead of blocking on a channel"
TAG: v0.6.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `view` vuelve a callbacks

## Prerrequisito

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Toda verificación de este plan usa `gotest` (nunca `go test` directo): corre
`vet`, `race`, cobertura y la suite WASM en un navegador real.

## Por qué (leer antes de tocar nada)

`view` expone cinco operaciones como si fueran síncronas, implementadas todas
bloqueando un canal sobre un transporte que es asíncrono:

```go
ch := make(chan error, 1)
b.caller.Call(b.ops.List, nil, dec, func(err error) { ch <- err })
if err := <-ch; err != nil { ... }      // ← el bloqueo
```

**Esto no es implementable en WebAssembly con el toolchain de Go.** Bloquear una
goroutine en un canal que solo un callback JS posterior puede llenar, estando ya
dentro de un callback JS (una respuesta de `fetch`, un handler de clic), no puede
resolverse nunca: el callback que llenaría el canal no corre hasta que el actual
retorne, y el actual no retorna porque está bloqueado. Medido en Chrome con el
paquete real:

| Runtime | Resultado |
|---|---|
| Go plano (`GOOS=js`, modo de build `L`, el usado por las apps) | `fatal error: all goroutines are asleep - deadlock!` · `exit code: 2` · el programa entero muere en silencio |
| TinyGo (`-target wasm`, modos `M`/`S`) | funciona — *asyncify* desenrolla y rebobina la pila |

Que dependa del runtime es lo que lo vuelve inaceptable: la misma librería
funciona o mata la aplicación según con qué toolchain se compiló. Hoy, en modo
`L`, **toda escritura de toda pantalla construida sobre `view` está muerta**:
guardar al salir de un campo, borrar, recargar.

La capa de abajo ya declara el contrato correcto. `router.Caller`, en su propia
interfaz:

```go
// done reports the outcome; result/err arrive asynchronously (works for wasm
// fetch and for in-process test doubles alike).
Call(op string, args model.Encodable, into model.Decodable, done func(err error))
```

`view` envolvió un contrato explícitamente asíncrono para fingir que no lo era.
Este plan deja de fingirlo. **No es una idea nueva: es volver a la forma que
`view` tenía** antes del commit `d468f09` (*"synchronous presenter methods"*),
donde era `Reload(done func(error))`, `Save(done func(error))`,
`Delete(id string, done func(error))`.

## Design gate

**1 · Prior art.** Ningún framework de UI bloquea esperando I/O: React
(`useEffect` + `setState`), Elm (`Cmd` → mensaje a `update`), Flutter (`Future`
+ `setState`). En los tres el efecto se pide y el resultado vuelve como
callback/mensaje que actualiza estado; una lectura síncrona de red no es
expresable. Nos diferenciábamos de los tres justo en lo que se rompe.

**2 · Novice-name test.** `lister.List(func(rows, err) { … })` se lee *"pedime la
lista y cuando llegue, llamame"*. `rows, err := lister.List()` se lee *"esto
vuelve ya"* — y miente.

**3 · Complexity ledger.** Conceptos **−1** (desaparece el puente
sync-sobre-async; `view` deja de usar canales). Archivos **0** nuevos. Líneas en
`view` **−~25** (se van cinco bloques de canal). Formas de hacerlo **2 → 1**
(hoy: callback en `Caller`, bloqueo en `view`). La última fila no queda positiva.

**4 · Dónde vive.** En `view`, dueño de "cómo se le pide un registro a la
aplicación". No en `dom` (no sabe de transportes) ni en cada consumidor.

**5 · Qué borra.** Los cinco `ch := make(chan error, 1)` de `caller_lister.go`
y `presenter.go`.

## Las firmas nuevas

`done` va **último**, como en todo lo asíncrono del ecosistema (`Caller.Call`,
`fetch…Send`, y el propio `view` anterior a `d468f09`).

```go
// lister.go
type Lister interface {
	List(done func(rows []model.Model, err error))
}

// view.go
type Saver interface {
	Save(recs []model.Model, done func(error))
}
type Updater interface {
	Update(ids []string, rec model.Model, fields []string, done func(error))
}
type Deleter interface {
	Delete(ids []string, done func(error))
}

// view.go — dentro de Presenter
Reload(done func(error))
```

**`Save` y `Delete` dejan de ser variádicos.** Un parámetro variádico debe ir
último y ahí va `done`. La semántica plural se conserva: se pasa un slice, y el
caso de un registro es un slice de uno. Actualizá el comentario de doc de
`Saver`/`Deleter` en `view.go`, que hoy justifica el variádico
(*"Variadic, not single: the one-record case is N=1"*) — la razón de fondo
(**un solo camino de escritura**) sigue valiendo y debe seguir escrita; lo que
cambia es cómo se pasa la pluralidad.

## Regla que NO se puede violar: un solo canal de error

Hoy `core.save/update/delete` validan y devuelven error *antes* de llamar al
lister (`"view: Save requires at least one record"`, `"view: Save payload is
nil"`, `"view: Save: lister does not implement view.Saver"`, `"view: Update
requires at least one field"`, `"view: Delete: unknown id %q"`, etc.).

Al pasar a callbacks, **esos errores de validación también viajan por `done`**.
Ninguno de los cinco métodos devuelve `error`. Tener a la vez un `error` de
retorno para errores de programación y un `done` para los de transporte serían
dos formas de reportar lo mismo (viola *one way to do each thing*) y garantiza
que un consumidor ignore una de las dos.

```go
func (c *core) save(recs []model.Model, done func(error)) {
	if len(recs) == 0 {
		done(fmt.Err("view: Save requires at least one record"))
		return
	}
	// … el resto de las validaciones, cada una con done(...) + return …
	b, ok := c.lister.(Saver)
	if !ok {
		done(fmt.Err("view: Save: lister does not implement view.Saver"))
		return
	}
	b.Save(recs, done)
}
```

Los mensajes de error se conservan **palabra por palabra**: hay tests que los
comparan.

`done` nunca es `nil` para el que implementa: si un consumidor no pasa nada,
`view` es quien debe protegerse. Normalizá una sola vez al entrar a cada método
público (`if done == nil { done = func(error) {} }`) en lugar de sembrar
`if done != nil` por todo el archivo.

## Etapas

### 1 · `lister.go`

`Lister.List` toma el callback. Actualizá el comentario de doc: describe que el
resultado llega de forma asíncrona y que el implementador **no debe bloquear**.

### 2 · `view.go`

`Saver`, `Updater`, `Deleter` y el método `Reload` de `Presenter`, con las firmas
de arriba. `New(...)` no cambia de firma. El bloque de doc del "capability-wrapper
pattern" sigue siendo válido tal cual.

### 3 · `presenter.go`

- `core.Reload(done func(error))`: pide al lister con callback; dentro del
  callback proyecta `items`/`index` exactamente como hoy (incluida la validación
  de `Itemizer` con su mensaje actual) y termina llamando `done(nil)` o
  `done(err)`.
- `core.save/update/delete`: la forma de arriba.
- Los 7 wrappers de capacidad (`saveable`, `updatable`, `deletable`,
  `saveableUpdatable`, `saveableDeletable`, `updatableDeletable`, y el de las
  tres) siguen siendo delegaciones finas: solo cambian de firma. **No muevas
  lógica a los wrappers** — el patrón está documentado en `lister.go` y debe
  seguir siendo "una sola implementación en `core`, N conjuntos de métodos".

### 4 · `caller_lister.go`

Los cuatro métodos de `callerLister` pierden el canal:

```go
func (b *callerLister) list(done func([]model.Model, error)) {
	list := b.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		done(nil, fmt.Err("view: list returned by newList does not implement model.Decodable"))
		return
	}
	b.caller.Call(b.ops.List, nil, dec, func(err error) {
		if err != nil {
			done(nil, err)
			return
		}
		rows := make([]model.Model, 0, list.Len())
		for i := 0; i < list.Len(); i++ {
			row := list.At(i)
			m, ok := row.(model.Model)
			if !ok {
				done(nil, fmt.Err("view: Reload: row type", rowName(row), "does not implement model.Model"))
				return
			}
			rows = append(rows, m)
		}
		done(rows, nil)
	})
}
```

`save`, `update` y `delete` quedan como un solo `b.caller.Call(..., done)` —
el callback del `Caller` ya tiene exactamente la firma `func(error)`.

Los 8 wrappers (`callerList`, `callerSave`, `callerUpdate`, `callerDelete`,
`callerSaveUpdate`, `callerSaveDelete`, `callerUpdateDelete`, `callerCRUD`)
solo cambian de firma. `NewCallerLister` **no** cambia de firma ni de panics.

### 5 · `conformance/` y `mock/`

`conformance/conformance.go` (`FakeLister`, `listOnlyLister`, `listSaveLister`
y las cláusulas que ejercitan guardar/borrar/recargar) y `mock/renderer.go` se
adaptan a las firmas nuevas. Las cláusulas de conformidad no se debilitan ni se
borran: si una cláusula ya no compila, se reescribe con callbacks, no se elimina.

Agregá **una cláusula nueva** a la conformidad, porque es el invariante que este
plan existe para garantizar y ningún test lo cubre hoy:

- **`no_blocking_in_list`** — un `Lister` cuyo `done` se invoca en un turno
  posterior del event loop (no inline) debe funcionar igual. En backend se
  simula con `go func(){ done(...) }()` + sincronización; el punto es que la
  conformidad falle si algún renderer vuelve a asumir que el resultado ya está
  disponible cuando `List` retorna.

### 6 · Tests

`plural_test.go`, `tests/module_test.go`, `tests/conformance_test.go`,
`tests/caller_backend_test.go`, `tests/backend_test.go` se adaptan. Ninguno se
borra. Agregá:

- **`TestListDoneRunsLater`** — el `Lister` doble invoca `done` de forma diferida
  (no inline). `Reload` debe proyectar los items correctamente cuando ese `done`
  finalmente corre, y `Items()` debe estar vacío antes.
- **`TestValidationErrorsTravelThroughDone`** — para cada validación de
  `save/update/delete`, el error llega por `done` y **no** hay valor de retorno.
  Compará los mensajes exactos.

### 7 · Documentación

- `README.md` — todos los ejemplos pasan a callbacks. Es el archivo que un dev
  sin contexto lee primero: el ejemplo principal debe mostrar la forma completa
  *pedir → pintar*, incluyendo que la actualización de UI va en el callback.
- Si `docs/` tiene algún documento que describa el contrato síncrono, corregilo.
  **No** cites `docs/PLAN.md` desde documentación permanente: este archivo se
  borra al publicar.

## Criterios de aceptación

- [ ] `grep -rn "make(chan" .` (excluyendo `_test.go`) → **vacío**.
- [ ] Ninguno de `Lister.List`, `Saver.Save`, `Updater.Update`, `Deleter.Delete`,
      `Presenter.Reload` devuelve `error`; los cinco toman `done` como último
      parámetro.
- [ ] `grep -rn "recs \.\.\.model.Model\|ids \.\.\.string" .` → **vacío**
      (el variádico se fue de las firmas públicas).
- [ ] Los mensajes de error existentes se conservan literalmente
      (`grep -c "view: Save requires at least one record" .` ≥ 1, y así con el
      resto).
- [ ] La cláusula `no_blocking_in_list` existe en `conformance/`.
- [ ] `gotest` → todo verde (`vet`, `race`, `tests`, `wasm`).
- [ ] `go build ./...` y `GOOS=js GOARCH=wasm go build ./...` compilan.
- [ ] `README.md` sin ejemplos síncronos.

## Lo que este plan NO hace

No toca ningún consumidor. `webtyp/layout/crudview`,
`veltylabs/modules/appointment_booking` y `webtyp/app-demo` se migran en fases
propias, cada una con su plan, **después** de que este publique su tag. Los
módulos que solo declaran `view.Ops{...}` y llaman `view.New(...)`
(`staff_manager`, `item_catalog`, `device_manager`, `patient_directory`,
`business_calendar`, `clinical_encounter`, `webtyp/auth`) no cambian.

| Etapa | Tarea |
|---|---|
| 1 | `lister.go` — `List(done)` |
| 2 | `view.go` — `Saver`/`Updater`/`Deleter`/`Presenter.Reload` |
| 3 | `presenter.go` — `core` + 7 wrappers, validaciones por `done` |
| 4 | `caller_lister.go` — sin canales, 8 wrappers |
| 5 | `conformance/` + `mock/` + cláusula `no_blocking_in_list` |
| 6 | Tests adaptados + los dos nuevos |
| 7 | `README.md` y docs |
