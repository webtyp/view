package view

import (
	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Ops names the remote operations a CallerLister invokes, scoped to Module.
// The 4 CRUD operations are derived automatically via crud.Op(sample, action).
// Write capabilities are governed by Allow (model.Create, model.Update, model.Delete;
// model.Read is always enabled).
type Ops struct {
	Module  string
	Allow   model.Action // write capabilities (model.Create, model.Update, model.Delete; model.Read is always enabled)
	Actions []Action
	Page    Page // how much the list loads on open and per search or scroll step; zero = DefaultPage
}

// qualified returns a copy of o with every non-empty action op prefixed by Module.
func (o Ops) qualified() Ops {
	q := o
	if len(o.Actions) > 0 {
		q.Actions = make([]Action, len(o.Actions))
		for i, a := range o.Actions {
			q.Actions[i] = a
			q.Actions[i].Op = o.Module + "." + a.Op
		}
	}
	return q
}

// NewCallerLister adapts a router.Caller (mcp, http, any transport) to the
// domain seam. It is the single place that translates a typed call into an
// operation name plus a wire envelope — no consumer writes that mapping again.
//
// newList builds the empty slice the transport decodes into; that is a codec
// concern, which is why it lives here and not in New.
//
// Ops.Module is required. The returned Lister implements
// exactly the write capabilities declared in Ops.Allow, so view.New's
// assertions stay honest for a remote backend too.
func NewCallerLister(c router.Caller, ops Ops, newList func() model.ModelSlice) Lister {
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
		if act.Op == "" {
			panic("view: NewCallerLister: Action.Op is required")
		}
		if act.Label == "" {
			panic("view: NewCallerLister: Action.Label is required")
		}
	}

	sample := newList().Append().(model.Model)
	opRead := ops.Module + "." + crud.Op(sample, model.Read)
	opCreate := ops.Module + "." + crud.Op(sample, model.Create)
	opUpdate := ops.Module + "." + crud.Op(sample, model.Update)
	opDelete := ops.Module + "." + crud.Op(sample, model.Delete)

	cb := &callerLister{
		caller:     c,
		ops:        ops.qualified(),
		rawActions: ops.Actions,
		newList:    newList,
		records:    rc.records,
		modelName:  sample.ModelName(),
		opRead:     opRead,
		opCreate:   opCreate,
		opUpdate:   opUpdate,
		opDelete:   opDelete,
	}

	hasS := ops.Allow.Has(model.Create)
	hasU := ops.Allow.Has(model.Update)
	hasD := ops.Allow.Has(model.Delete)

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
	rawActions []Action // The UNPREFIXED bare operations to match p.Run(op)
	newList    func() model.ModelSlice
	records    *Records
	modelName  string
	opRead     string
	opCreate   string
	opUpdate   string
	opDelete   string
}

func (b *callerLister) Records() *Records { return b.records }

func (b *callerLister) Page() Page { return b.ops.Page }

func (b *callerLister) Actions() []Action {
	return b.rawActions
}

func (b *callerLister) RunAction(op string, args model.Encodable, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	// rawActions and ops.Actions share indexes: ops.Actions[i] is rawActions[i] qualified.
	for i, act := range b.rawActions {
		if act.Op == op {
			b.caller.Call(b.ops.Actions[i].Op, args, nil, done)
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
