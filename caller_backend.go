package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Ops names the remote operations a CallerLister invokes. An empty name means
// the remote side does not offer that operation, and the returned Lister will
// not carry the matching capability.
type Ops struct {
	List   string
	Save   string
	Update string
	Delete string
}

// NewCallerLister adapts a router.Caller (mcp, http, any transport) to the
// domain seam. It is the single place that translates a typed call into an
// operation name plus a wire envelope — no consumer writes that mapping again.
//
// newList builds the empty slice the transport decodes into; that is a codec
// concern, which is why it lives here and not in New.
//
// Ops.List is required. The returned Lister implements exactly the write
// capabilities whose op names are non-empty, so view.New's assertions stay
// honest for a remote backend too.
func NewCallerLister(c router.Caller, ops Ops, newList func() model.ModelSlice) Lister {
	if c == nil {
		panic("view: NewCallerLister: caller is required")
	}
	if ops.List == "" {
		panic("view: NewCallerLister: Ops.List is required")
	}
	if newList == nil {
		panic("view: NewCallerLister: newList is required")
	}
	cb := &callerLister{caller: c, ops: ops, newList: newList}
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

func (b *callerLister) list() ([]model.Model, error) {
	list := b.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		return nil, fmt.Err("view: list returned by newList does not implement model.Decodable")
	}
	ch := make(chan error, 1)
	b.caller.Call(b.ops.List, nil, dec, func(err error) { ch <- err })
	if err := <-ch; err != nil {
		return nil, err
	}
	rows := make([]model.Model, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		row := list.At(i)
		m, ok := row.(model.Model)
		if !ok {
			return nil, fmt.Err("view: Reload: row type", rowName(row), "does not implement model.Model")
		}
		rows = append(rows, m)
	}
	return rows, nil
}

func (b *callerLister) save(recs []model.Model) error {
	ch := make(chan error, 1)
	b.caller.Call(b.ops.Save, &saveArgs{recs: recs}, nil, func(err error) { ch <- err })
	return <-ch
}

func (b *callerLister) update(ids []string, rec model.Model, fields []string) error {
	ch := make(chan error, 1)
	b.caller.Call(b.ops.Update, &updateArgs{ids: ids, fields: fields, rec: rec}, nil, func(err error) { ch <- err })
	return <-ch
}

func (b *callerLister) delete(ids []string) error {
	ch := make(chan error, 1)
	b.caller.Call(b.ops.Delete, &deleteArgs{ids: ids}, nil, func(err error) { ch <- err })
	return <-ch
}

// The capability wrappers below follow the pattern documented in lister.go:
// thin structs whose only difference is the method SET. See it before editing.
type callerList struct {
	*callerLister
}

func (b *callerList) List() ([]model.Model, error) {
	return b.list()
}

type callerSave struct {
	*callerLister
}

func (b *callerSave) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerSave) Save(recs ...model.Model) error {
	return b.save(recs)
}

type callerUpdate struct {
	*callerLister
}

func (b *callerUpdate) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerUpdate) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

type callerDelete struct {
	*callerLister
}

func (b *callerDelete) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerDelete) Delete(ids ...string) error {
	return b.delete(ids)
}

type callerSaveUpdate struct {
	*callerLister
}

func (b *callerSaveUpdate) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerSaveUpdate) Save(recs ...model.Model) error {
	return b.save(recs)
}

func (b *callerSaveUpdate) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

type callerSaveDelete struct {
	*callerLister
}

func (b *callerSaveDelete) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerSaveDelete) Save(recs ...model.Model) error {
	return b.save(recs)
}

func (b *callerSaveDelete) Delete(ids ...string) error {
	return b.delete(ids)
}

type callerUpdateDelete struct {
	*callerLister
}

func (b *callerUpdateDelete) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerUpdateDelete) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

func (b *callerUpdateDelete) Delete(ids ...string) error {
	return b.delete(ids)
}

type callerCRUD struct {
	*callerLister
}

func (b *callerCRUD) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerCRUD) Save(recs ...model.Model) error {
	return b.save(recs)
}

func (b *callerCRUD) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

func (b *callerCRUD) Delete(ids ...string) error {
	return b.delete(ids)
}
