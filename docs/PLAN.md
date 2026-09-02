---
PLAN: "feat!: plural Saver/Deleter and a new Updater for field patches"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.
>
> Forma parte de una ola: `docs/BULK_ACTIONS_MASTER_PLAN.md` en la raíz del
> monorepo. Este plan es **independiente** — no espera a ningún otro. Es
> **puerta** para el plan de `layout`.
>
> **Es un cambio breaking.** Ver §6.

# Plan — Contratos plurales: `Save`, `Update`, `Delete`

## 0. Prerrequisito

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

Los tests se ejecutan con `gotest`. **Nunca `go test`.**

## 1. Por qué

Se añade selección múltiple: borrar tres registros de una, y corregir un campo
que quedó mal en varios a la vez. La regla que fija la ola es **una sola
interfaz por capacidad, siempre plural**: el caso de un solo registro es N=1,
nunca un camino aparte. Dos interfaces (`Deleter` + `BulkDeleter`) duplicarían
el código y el sitio de decisión.

Hoy `view/view.go` declara:

```go
type Saver interface {
	Save(payload model.Model) error
}

type Deleter interface {
	Delete(id string) error
}
```

Ambas singulares. Y no hay nada para "parchea estos campos en estos ids".

## 2. Los contratos nuevos

Fichero: **`view/view.go`**, sustituyendo las dos interfaces actuales.

```go
// Saver creates or replaces WHOLE records. Variadic, not single: the one-record
// case is N=1, so there is exactly one write path to implement, test and reason
// about. This is what the "+" new-record flow and the edit-one-record form use.
type Saver interface {
	Save(recs ...model.Model) error
}

// Updater patches ONLY the named columns across every id, in a single
// statement. It is deliberately NOT Save with a partial record: sending whole
// records to change one column reverts, in silence, every other column that
// someone else changed since this client last reloaded — the classic lost
// update. Naming the columns makes that impossible: what is not in fields is
// never written.
//
// rec carries the values (it is the form's own record, already validated);
// fields names the columns to write. Values stay inside the generated typed
// struct — no name→value bag, no `any`, no reflection.
//
// fields holds Schema() field names and pairs directly with
// form.DirtyFields(). An empty fields slice is an error, not a no-op.
type Updater interface {
	Update(ids []string, rec model.Model, fields []string) error
}

// Deleter removes records. Variadic for the same reason as Saver.
type Deleter interface {
	Delete(ids ...string) error
}
```

Las tres se siguen descubriendo por *type assertion* en el consumidor,
exactamente como hoy. No las metas en `Presenter`.

## 3. Etapa 1 — La opción de configuración del op nuevo

Fichero: **`view/view.go`**.

`Updater` necesita su propio op. Junto a `WithSaveOp` / `WithDeleteOp` (que ya
existen, alrededor de la línea 86) añade:

```go
// WithUpdateOp sets the field-patch operation.
func WithUpdateOp(op string) Option {
```

Guárdalo en `config.updateOp` y propágalo al `core` igual que se propagan
`saveOp` y `deleteOp`. Sigue el patrón que ya está escrito; no inventes otro.

## 4. Etapa 2 — Implementaciones

Fichero: **`view/presenter.go`**.

Hoy hay dos parejas de implementaciones que hacen lo mismo:
`saveable`/`deletable` por un lado, y `crud` por otro (líneas ~123-175). Sus
cuerpos son idénticos. Al pasar a plural, **no dupliques la lógica una tercera
vez**: extrae el cuerpo a un método del `core` y que las tres lo llamen.

```go
// en core
func (c *core) save(recs []model.Model) error   { ... }
func (c *core) update(ids []string, rec model.Model, fields []string) error { ... }
func (c *core) delete(ids []string) error       { ... }
```

Reglas de cada uno:

**`save`**
- `len(recs) == 0` → `fmt.Err("view: Save requires at least one record")`.
- Cualquier `model.IsNil(rec)` → `fmt.Err("view: Save payload is nil")`
  (mismo texto que hoy).
- Envía **una sola llamada** con todos los registros.

**`update`**
- `len(ids) == 0` → `fmt.Err("view: Update requires at least one id")`.
- `len(fields) == 0` → `fmt.Err("view: Update requires at least one field")`.
- `model.IsNil(rec)` → `fmt.Err("view: Update record is nil")`.
- Envía `{ids, fields, record}` en **una sola llamada** a `updateOp`.

**`delete`**
- `len(ids) == 0` → `fmt.Err("view: Delete requires at least one id")`.
- Cada id debe existir en `c.index`; si alguno no,
  `fmt.Errf("view: Delete: unknown id %q", id)` (hoy el mensaje es
  `view: Delete: unknown id`, sin el id — añádelo, ayuda a depurar N ids).
- Comprueba **todos** los ids ANTES de enviar nada. Enviar y fallar a mitad es
  justo lo que la operación plural existe para evitar.
- Envía **una sola llamada** con todos los registros indexados.

## 4.1 Etapa 2b — Quitar el mapa del índice (obligatoria)

`view/presenter.go` declara:

```go
index    map[string]model.Model
```

y **no lleva build tag**, así que compila a WASM. Un mapa arrastra la
maquinaria de hashing y el runtime de TinyGo al binario del navegador, que es
justo lo que este ecosistema no puede permitirse.

Esto no es un extra opcional: la Etapa 2 reescribe `delete` para validar N ids
**contra ese mismo índice** antes de enviar nada. Dejar el mapa significaría
ampliar el uso de una estructura que no debería estar ahí.

Sustitúyelo por un slice de un par tipado, declarado en `presenter.go`:

```go
// indexEntry pairs a record with its id. A slice of these replaces what used
// to be a map[string]model.Model: this file compiles to WASM, and a map pulls
// TinyGo's hashing machinery into the browser binary.
//
// Not fmt.KeyValue — that is {Key, Value string}, and the value here is a
// model.Model. fmt.KeyValue is the ecosystem's map-free shape for string→string
// pairs; anything else gets a typed pair like this one.
type indexEntry struct {
	id  string
	rec model.Model
}
```

Sitios de llamada a migrar (todos en `view/presenter.go`, líneas de hoy):

| Hoy | Pasa a ser |
|---|---|
| `index map[string]model.Model` (l. 22) | `index []indexEntry` |
| `p.index = make(map[string]model.Model, list.Len())` (l. 64) | `p.index = make([]indexEntry, 0, list.Len())` |
| `p.index[it.ID] = m` (l. 77) | `p.index = append(p.index, indexEntry{id: it.ID, rec: m})` |
| `m, ok := p.index[id]` (l. 96) | un método `lookup(id)` privado |
| `rec, ok := d.index[id]` (l. 142) | el mismo `lookup(id)` |

Escribe **un solo** buscador y que los dos sitios lo llamen:

```go
// lookup finds the record for id. Linear, and that is fine: a projected list
// holds tens to low hundreds of rows, so the scan costs microseconds — the map
// was never buying anything measurable, and it cost binary size on every page
// load.
func (c *core) lookup(id string) (model.Model, bool)
```

Dos búsquedas idénticas escritas por separado son exactamente la duplicación
que se desincroniza en el siguiente refactor.

**Ojo con `Reload()`:** el índice se reconstruye en cada recarga. Con un mapa,
`make(...)` lo vaciaba; con un slice, asegúrate de reasignar (no de hacer
`append` sobre el anterior), o los registros viejos sobreviven a la recarga y
`Delete` aceptaría ids que ya no existen.

## 5. Etapa 3 — El sobre que viaja por el cable

Fichero nuevo: **`view/payload.go`**.

`router.Caller.Call` toma un único `model.Encodable`:

```go
Call(op string, args model.Encodable, into model.Decodable, done func(err error))
```

Así que hacen falta tipos que empaqueten varios valores en un `Encodable`. Son
privados de `view` y **no requieren tocar `model` ni `ormc`**: todo lo que hace
falta ya existe en `model.FieldWriter`.

```go
// updateArgs is the wire shape of a field patch: which rows, which columns,
// and a record carrying the values.
type updateArgs struct {
	ids    []string
	fields []string
	rec    model.Model
}

func (a *updateArgs) IsNil() bool { return a == nil }

func (a *updateArgs) EncodeFields(w model.FieldWriter) {
	ids := w.Array("ids", len(a.ids))
	for _, id := range a.ids {
		ids.String(id)
	}
	fields := w.Array("fields", len(a.fields))
	for _, f := range a.fields {
		fields.String(f)
	}
	w.Object("record", a.rec)
}
```

Y el equivalente para `save` (array de objetos) y `delete` (array de objetos o
de ids, según lo que el handler necesite — decídelo y documéntalo en el
fichero):

```go
// saveArgs carries N whole records.
type saveArgs struct{ recs []model.Model }

func (a *saveArgs) EncodeFields(w model.FieldWriter) {
	arr := w.Array("records", len(a.recs))
	for _, r := range a.recs {
		arr.Object(r)
	}
}
```

**Verificado antes de escribir este plan:** `model.FieldWriter` ya tiene
`Array(name string, n int) ArrayWriter` y `Object(name string, val Encodable)`,
y `model.ArrayWriter` ya tiene `String(val string)` y `Object(val Encodable)`.
No hace falta añadir nada a `model`.

**Anti-footgun:** NO uses el tipo `XList` que genera `ormc` (p. ej.
`RoleList`). Su `EncodeFields` es hoy un **stub vacío**
(`func (s *RoleList) EncodeFields(_ model.FieldWriter) {}`), así que
compilaría y enviaría un objeto vacío, en silencio. Rellenarlo es otro plan y
regeneraría todos los `*_orm.go` del ecosistema. Usa los tipos de este fichero.

## 6. Etapa 4 — Arreglar los consumidores internos

Estos dos están **dentro de este repositorio** y son parte de este plan:

- **`view/mock/renderer.go`** — implementa `Saver`/`Deleter`. Cambia las
  firmas a plural. Si acumula llamadas para que los tests las inspeccionen,
  guarda ahora el slice completo, no un solo valor.
- **`view/conformance/conformance.go`** — el rail de conformidad. Cambia las
  firmas y **añade cobertura de los casos plurales**: borrar 2 ids de una,
  parchear 1 campo sobre 2 ids. Un rail de conformidad que sólo ejercita N=1
  no protege nada de lo que este plan añade.

Consumidores **fuera** de este repositorio (NO son parte de este plan; van en
la fase C de la ola):

```
layout/crudview/crudview.go
crudp/handlers.go
app-demo/modules/medicalhistory/medicalhistory.go
auth/authority/users.go
auth/authority/credentials_lan.go
```

La firma variádica está elegida para minimizar el daño: `Delete(id)` sigue
compilando tal cual en los **sitios de llamada**; sólo cambian las
**implementaciones**.

## 7. Etapa 5 — Tests

Fichero: **`view/plural_test.go`**.

| Test | Comprueba |
|---|---|
| `TestSaveRejectsAnEmptyBatch` | Error con `at least one record` |
| `TestSaveShipsEveryRecordInOneCall` | 3 registros → el caller doble recibe **1** llamada |
| `TestUpdateRejectsAnEmptyIDList` | Error con `at least one id` |
| `TestUpdateRejectsAnEmptyFieldList` | Error con `at least one field` |
| `TestUpdateShipsIDsFieldsAndRecord` | El `Encodable` escribe `ids`, `fields` y `record` |
| `TestDeleteRejectsAnEmptyIDList` | Error con `at least one id` |
| `TestDeleteValidatesEveryIDBeforeShipping` | Con un id desconocido entre válidos → error, y el caller doble recibe **cero** llamadas |
| `TestDeleteShipsEveryIDInOneCall` | 3 ids → **1** llamada |

`TestDeleteValidatesEveryIDBeforeShipping` es el que da sentido a la operación
plural: o va todo, o no va nada.

### La regla que decide CÓMO se escriben estos tests

> **An API is not published until a consumer-shaped test, inside the library
> itself, proves it.** A library tested only in isolation — with opaque doubles
> standing in for its real collaborators — hides its gaps until a consumer hits
> them.

Aquí eso significa: **modelos reales** (structs generados por `ormc`, no un
`model.Model` de mentira) y **un `router.Caller` falso** que registre las
llamadas. El caller es el borde de E/S, y es lo único que se finge.

Por eso la Etapa 4 no es opcional: `view/conformance` es el rail que ejercita
la forma que un consumidor usará de verdad, y **hoy sólo cubre N=1**. Un rail
que no ejercita el camino plural no protege nada de lo que este plan añade.

**Anti-footgun:** sólo librería estándar de testing. Nada de `testify` ni
`gomega`.

## 8. Criterios de aceptación

- [ ] `gotest` en verde (vet, race, cover, wasm).
- [ ] `grep -n "Save(payload model.Model)\|Delete(id string)" view/*.go` →
      **sin resultados**. Las firmas singulares desaparecen; no se quedan como
      capa de compatibilidad.
- [ ] `grep -rn "BulkSaver\|BulkDeleter\|SaveMany\|DeleteMany" .` →
      **sin resultados**. Una sola interfaz por capacidad, no dos.
- [ ] `grep -n "func WithUpdateOp" view/view.go` → una línea.
- [ ] `grep -rn "RoleList\|List{}" view/` → sin resultados: no se usan los
      tipos `XList` de `ormc`.
- [ ] Los cuerpos de `saveable`/`deletable`/`crud` no duplican la lógica:
      `grep -c "caller.Call" view/presenter.go` no aumenta respecto a hoy.
- [ ] `view/conformance/conformance.go` ejercita el camino plural con N>1, no
      sólo N=1.
- [ ] **Ni un mapa en el binario WASM:** `grep -rn "map\[" view/*.go | grep -v _test`
      → **sin resultados**. (Un mapa dentro de un fichero `//go:build !wasm`
      sería legítimo; en `view` no hay ninguno.)
- [ ] Hay **un solo** buscador por id:
      `grep -c "func (c \*core) lookup" view/presenter.go` → 1.

## 8.1 Nota de diseño: por qué `fields []string` y no un tipo cerrado

El principio 6 del harness pide fallar en compilación antes que en ejecución.
Aquí no se puede: los campos que el usuario tocó se conocen **en tiempo de
ejecución** (salen de `form.DirtyFields()`, que depende de lo que se escribió
en pantalla). No existe un tipo que haga imposible un nombre inválido.

El harness da la alternativa para ese caso: *"compile error → loud development
diagnostic → (never) silent failure"*. Por eso `Update` **rechaza con error**
la lista vacía y `orm.UpdateFields` **rechaza con error** un nombre que no está
en el esquema (`orm: UpdateFields: unknown field %q`). Lo que nunca puede pasar
es que un nombre mal escrito produzca un `UPDATE` silencioso que no toque nada.

## 9. Etapas

| # | Etapa | Ficheros | Depende de |
|---|---|---|---|
| 1 | Interfaces plurales + `WithUpdateOp` | `view/view.go` | — |
| 2 | Implementaciones compartidas en `core` | `view/presenter.go` | 1 |
| 2b | Índice sin mapa (slice + `lookup`) | `view/presenter.go` | — |
| 3 | Sobres `Encodable` | `view/payload.go` | 1 |
| 4 | Mock y rail de conformidad | `view/mock/renderer.go`, `view/conformance/conformance.go` | 2, 3 |
| 5 | Tests | `view/plural_test.go` | 2, 3 |
