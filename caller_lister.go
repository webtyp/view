package view

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Ops names the remote operations a CallerLister invokes, scoped to Module —
// the same string the server's router.OperationModule.ModelName() returns
// for the module that harvests these operations (mcp.HarvestOps qualifies
// every Tool.Name as "<ModelName>.<name>"; see that package's own doc).
// NewCallerLister composes the identical qualified name here, so no caller
// ever writes "module.op" as a hand-assembled literal — the two sides read
// it off the same two strings (Module + the bare op name) instead of a
// string neither owns.
//
// An empty op name means the remote side does not offer that operation, and
// the returned Lister will not carry the matching capability.
type Ops struct {
	Module  string
	List    string
	Save    string
	Update  string
	Delete  string
	Actions []Action
}

// qualified returns a copy of o with every non-empty op name prefixed by
// Module — the exact transformation mcp.HarvestOps applies server-side.
func (o Ops) qualified() Ops {
	q := o
	q.List = o.Module + "." + o.List
	if o.Save != "" {
		q.Save = o.Module + "." + o.Save
	}
	if o.Update != "" {
		q.Update = o.Module + "." + o.Update
	}
	if o.Delete != "" {
		q.Delete = o.Module + "." + o.Delete
	}
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
// Ops.Module and Ops.List are required. The returned Lister implements
// exactly the write capabilities whose op names are non-empty, so view.New's
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
	if ops.List == "" {
		panic("view: NewCallerLister: Ops.List is required")
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
	cb := &callerLister{
		caller:     c,
		ops:        ops.qualified(),
		rawActions: ops.Actions,
		newList:    newList,
		records:    rc.records,
		modelName:  newList().Append().(model.Model).ModelName(),
	}
	hasS := ops.Save != ""
	hasU := ops.Update != ""
	hasD := ops.Delete != ""
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
}

func (b *callerLister) Records() *Records { return b.records }

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

func (b *callerLister) list(done func([]model.Model, error)) {
	list := b.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		done(nil, fmt.Err("view: list returned by newList", "does not implement", typeName("model.Decodable")))
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
				done(nil, fmt.Err("view: Reload: row type", "<unnamed type>", "does not implement", typeName("model.Model")))
				return
			}
			rows = append(rows, m)
		}
		done(rows, nil)
	})
}

func (b *callerLister) save(recs []model.Model, done func(error)) {
	saved, ok := b.newList().Append().(model.Model)
	if !ok {
		done(fmt.Err("view: list returned by newList", "does not implement", typeName("model.Model")))
		return
	}
	b.caller.Call(b.ops.Save, &saveArgs{recs: recs}, saved, func(err error) {
		if err == nil {
			b.records.Put(saved)
		}
		done(err)
	})
}

func (b *callerLister) update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.caller.Call(b.ops.Update, &updateArgs{ids: ids, fields: fields, rec: rec}, nil, done)
}

func (b *callerLister) delete(ids []string, done func(error)) {
	b.caller.Call(b.ops.Delete, &deleteArgs{ids: ids}, nil, func(err error) {
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

func (b *callerList) List(done func([]model.Model, error)) {
	b.list(done)
}

type callerSave struct {
	*callerLister
}

func (b *callerSave) List(done func([]model.Model, error)) {
	b.list(done)
}

func (b *callerSave) Save(recs []model.Model, done func(error)) {
	b.save(recs, done)
}

type callerUpdate struct {
	*callerLister
}

func (b *callerUpdate) List(done func([]model.Model, error)) {
	b.list(done)
}

func (b *callerUpdate) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.update(ids, rec, fields, done)
}

type callerDelete struct {
	*callerLister
}

func (b *callerDelete) List(done func([]model.Model, error)) {
	b.list(done)
}

func (b *callerDelete) Delete(ids []string, done func(error)) {
	b.delete(ids, done)
}

type callerSaveUpdate struct {
	*callerLister
}

func (b *callerSaveUpdate) List(done func([]model.Model, error)) {
	b.list(done)
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

func (b *callerSaveDelete) List(done func([]model.Model, error)) {
	b.list(done)
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

func (b *callerUpdateDelete) List(done func([]model.Model, error)) {
	b.list(done)
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

func (b *callerCRUD) List(done func([]model.Model, error)) {
	b.list(done)
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
