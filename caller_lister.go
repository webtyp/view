package view

import (
	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Ops names the module a CallerLister talks to. The four CRUD operations are derived from the model:
// router.Operation{Module: Module, Entity: <the model's name>, Action: …}. Write capabilities are governed by the model's Allow().
type Ops struct {
	Module  string
	Actions []Action
	Page    Page // how much the list loads on open and per search or scroll step; zero = DefaultPage
}

// NewCallerLister adapts a router.Caller (mcp, http, any transport) to the
// domain seam. It is the single place that translates a typed call into an
// operation name plus a wire envelope — no consumer writes that mapping again.
//
// newList builds the empty slice the transport decodes into; that is a codec
// concern, which is why it lives here and not in New.
//
// Ops.Module is required. The returned Lister implements
// exactly the write capabilities declared in the model's Allow(), so view.New's
// assertions stay honest for a remote backend too.
func NewCallerLister(c router.Caller, ops Ops, newList func() model.TableSlice) Lister {
	if c == nil {
		panic("view: NewCallerLister: caller is required")
	}
	rc, ok := c.(*recordsCaller)
	if !ok {
		panic("view: NewCallerLister: caller must come from view.Records.Caller — build it once per page: view.NewRecords().Caller(c)")
	}
	if ops.Module == "" {
		panic("view: NewCallerLister: Ops.Module is required — it must match the server module's ModelName()")
	}
	if newList == nil {
		panic("view: NewCallerLister: newList is required")
	}
	for _, act := range ops.Actions {
		if !act.Operation.Valid() {
			panic("view: NewCallerLister: Action.Operation must be a valid router.Operation")
		}
		if act.Label == "" {
			panic("view: NewCallerLister: Action.Label is required")
		}
	}

	sample := newList().Append().(model.Model)
	allow := newList().Allow()
	if !allow.Has(model.Read) {
		panic("view: NewCallerLister: " + sample.ModelName() + " does not allow Read — add model.Read to Allow in its model.Definition (and rerun ormc)")
	}

	entity := sample.ModelName()
	cb := &callerLister{
		caller:     c,
		ops:        ops,
		rawActions: ops.Actions,
		newList:    func() model.ModelSlice { return newList() },
		records:    rc.records,
		modelName:  entity,
		opRead:     router.Operation{Module: ops.Module, Entity: entity, Action: model.Read},
		opCreate:   router.Operation{Module: ops.Module, Entity: entity, Action: model.Create},
		opUpdate:   router.Operation{Module: ops.Module, Entity: entity, Action: model.Update},
		opDelete:   router.Operation{Module: ops.Module, Entity: entity, Action: model.Delete},
	}

	hasS := allow.Has(model.Create)
	hasU := allow.Has(model.Update)
	hasD := allow.Has(model.Delete)

	switch {
	case hasS && hasU && hasD:
		return &callerCRUD{callerLister: cb}
	case hasS && hasU:
		return &callerSaveUpdate{callerLister: cb}
	case hasS && hasD:
		return &callerSaveDelete{callerLister: cb}
	case hasU && hasD:
		return &callerUpdateDelete{callerLister: cb}
	case hasS:
		return &callerSave{callerLister: cb}
	case hasU:
		return &callerUpdate{callerLister: cb}
	case hasD:
		return &callerDelete{callerLister: cb}
	default:
		return &callerList{callerLister: cb}
	}
}

type callerLister struct {
	caller     router.Caller
	ops        Ops
	rawActions []Action
	newList    func() model.ModelSlice
	records    *Records
	modelName  string
	opRead     router.Operation
	opCreate   router.Operation
	opUpdate   router.Operation
	opDelete   router.Operation
}

func (b *callerLister) Records() *Records { return b.records }

func (b *callerLister) Page() Page { return b.ops.Page }

func (b *callerLister) Actions() []Action {
	return b.rawActions
}

func (b *callerLister) RunAction(op router.Operation, args model.Encodable, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	for _, act := range b.rawActions {
		if act.Operation == op {
			b.caller.Call(op, args, nil, done)
			return
		}
	}
	done(errUnknownAction(op))
}

var (
	opReloadPrefix     = "view: Reload:"
	opSaveListerPrefix = "view: Save:"
)

func (b *callerLister) list(args crud.ReadArgs, done func([]Record, error)) {
	list := b.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		done(nil, fmt.Err("view: list returned by newList", "does not implement", typeName("model.Decodable")))
		return
	}
	b.caller.Call(b.opRead, &args, dec, func(err error) {
		if err != nil {
			done(nil, err)
			return
		}
		rows := make([]Record, 0, list.Len())
		for i := 0; i < list.Len(); i++ {
			rec, ok := list.At(i).(Record)
			if !ok {
				done(nil, fmt.Err(opReloadPrefix, "row type", rowName(list.At(i)), "has no Item(): mark its fields with Item: model.ItemLabel and run ormc"))
				return
			}
			rows = append(rows, rec)
		}
		done(rows, nil)
	})
}

func (b *callerLister) save(recs []model.Model, done func(error)) {
	raw := b.newList().Append()
	saved, ok := raw.(Record)
	if !ok {
		done(fmt.Err(opSaveListerPrefix, "row type", rowName(raw), "has no Item(): mark its fields with Item: model.ItemLabel and run ormc"))
		return
	}
	b.caller.Call(b.opCreate, &crud.CreateArgs{Records: recs}, saved, func(err error) {
		if err == nil {
			b.records.Put(saved)
		}
		done(err)
	})
}

func rowName(v any) string {
	if m, ok := v.(model.Model); ok {
		return m.ModelName()
	}
	return "<unnamed type>"
}

func (b *callerLister) update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.caller.Call(b.opUpdate, &crud.UpdateArgs{IDs: ids, Fields: fields, Record: rec}, nil, done)
}

func (b *callerLister) delete(ids []string, done func(error)) {
	b.caller.Call(b.opDelete, &crud.DeleteArgs{IDs: ids}, nil, func(err error) {
		if err == nil {
			b.records.Remove(b.modelName, ids...)
		}
		done(err)
	})
}

// Compile-time assertions for ActionRunner on all capability wrappers
var (
	_ ActionRunner = (*callerList)(nil)
	_ ActionRunner = (*callerSave)(nil)
	_ ActionRunner = (*callerUpdate)(nil)
	_ ActionRunner = (*callerDelete)(nil)
	_ ActionRunner = (*callerSaveUpdate)(nil)
	_ ActionRunner = (*callerSaveDelete)(nil)
	_ ActionRunner = (*callerUpdateDelete)(nil)
	_ ActionRunner = (*callerCRUD)(nil)
)

// The capability wrappers below follow the pattern documented in lister.go:
// thin structs whose only difference is the method SET. See it before editing.
type callerList struct {
	*callerLister
}

func (b *callerList) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

type callerSave struct {
	*callerLister
}

func (b *callerSave) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerSave) Save(recs []model.Model, done func(error)) {
	b.save(recs, done)
}

type callerUpdate struct {
	*callerLister
}

func (b *callerUpdate) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerUpdate) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.update(ids, rec, fields, done)
}

type callerDelete struct {
	*callerLister
}

func (b *callerDelete) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerDelete) Delete(ids []string, done func(error)) {
	b.delete(ids, done)
}

type callerSaveUpdate struct {
	*callerLister
}

func (b *callerSaveUpdate) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerSaveUpdate) Save(recs []model.Model, done func(error)) {
	b.save(recs, done)
}

func (b *callerSaveUpdate) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.update(ids, rec, fields, done)
}

type callerSaveDelete struct {
	*callerLister
}

func (b *callerSaveDelete) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerSaveDelete) Save(recs []model.Model, done func(error)) {
	b.save(recs, done)
}

func (b *callerSaveDelete) Delete(ids []string, done func(error)) {
	b.delete(ids, done)
}

type callerUpdateDelete struct {
	*callerLister
}

func (b *callerUpdateDelete) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerUpdateDelete) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.update(ids, rec, fields, done)
}

func (b *callerUpdateDelete) Delete(ids []string, done func(error)) {
	b.delete(ids, done)
}

type callerCRUD struct {
	*callerLister
}

func (b *callerCRUD) List(args crud.ReadArgs, done func([]Record, error)) {
	b.list(args, done)
}

func (b *callerCRUD) Save(recs []model.Model, done func(error)) {
	b.save(recs, done)
}

func (b *callerCRUD) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.update(ids, rec, fields, done)
}

func (b *callerCRUD) Delete(ids []string, done func(error)) {
	b.delete(ids, done)
}
