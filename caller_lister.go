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
	Module string
	List   string
	Save   string
	Update string
	Delete string
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
	if ops.Module == "" {
		panic("view: NewCallerLister: Ops.Module is required — it must match the server module's ModelName()")
	}
	if ops.List == "" {
		panic("view: NewCallerLister: Ops.List is required")
	}
	if newList == nil {
		panic("view: NewCallerLister: newList is required")
	}
	cb := &callerLister{caller: c, ops: ops.qualified(), newList: newList}
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
	caller  router.Caller
	ops     Ops
	newList func() model.ModelSlice
}

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

func (b *callerLister) save(recs []model.Model, done func(error)) {
	b.caller.Call(b.ops.Save, &saveArgs{recs: recs}, nil, done)
}

func (b *callerLister) update(ids []string, rec model.Model, fields []string, done func(error)) {
	b.caller.Call(b.ops.Update, &updateArgs{ids: ids, fields: fields, rec: rec}, nil, done)
}

func (b *callerLister) delete(ids []string, done func(error)) {
	b.caller.Call(b.ops.Delete, &deleteArgs{ids: ids}, nil, done)
}

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
