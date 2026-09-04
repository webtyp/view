package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// Ops names the remote operations a CallerBackend invokes. An empty name means
// the remote side does not offer that operation, and the returned Backend will
// not carry the matching capability.
type Ops struct {
	List   string
	Save   string
	Update string
	Delete string
}

// NewCallerBackend adapts a router.Caller (mcp, http, any transport) to the
// domain seam. It is the single place that translates a typed call into an
// operation name plus a wire envelope — no consumer writes that mapping again.
//
// newList builds the empty slice the transport decodes into; that is a codec
// concern, which is why it lives here and not in New.
//
// Ops.List is required. The returned Backend implements exactly the write
// capabilities whose op names are non-empty, so view.New's assertions stay
// honest for a remote backend too.
func NewCallerBackend(c router.Caller, ops Ops, newList func() model.ModelSlice) Backend {
	if c == nil {
		panic("view: NewCallerBackend: caller is required")
	}
	if ops.List == "" {
		panic("view: NewCallerBackend: Ops.List is required")
	}
	if newList == nil {
		panic("view: NewCallerBackend: newList is required")
	}
	cb := &callerBackend{caller: c, ops: ops, newList: newList}
	hasS := ops.Save != ""
	hasU := ops.Update != ""
	hasD := ops.Delete != ""
	switch {
	case hasS && hasU && hasD:
		return &callerCRUD{callerBackend: cb}
	case hasS && hasU:
		return &callerSaveUpdate{callerBackend: cb}
	case hasS && hasD:
		return &callerSaveDelete{callerBackend: cb}
	case hasU && hasD:
		return &callerUpdateDelete{callerBackend: cb}
	case hasS:
		return &callerSave{callerBackend: cb}
	case hasU:
		return &callerUpdate{callerBackend: cb}
	case hasD:
		return &callerDelete{callerBackend: cb}
	default:
		return &callerList{callerBackend: cb}
	}
}

type callerBackend struct {
	caller  router.Caller
	ops     Ops
	newList func() model.ModelSlice
}

func (b *callerBackend) list() ([]model.Model, error) {
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

func (b *callerBackend) save(recs []model.Model) error {
	ch := make(chan error, 1)
	b.caller.Call(b.ops.Save, &saveArgs{recs: recs}, nil, func(err error) { ch <- err })
	return <-ch
}

func (b *callerBackend) update(ids []string, rec model.Model, fields []string) error {
	ch := make(chan error, 1)
	b.caller.Call(b.ops.Update, &updateArgs{ids: ids, fields: fields, rec: rec}, nil, func(err error) { ch <- err })
	return <-ch
}

func (b *callerBackend) delete(ids []string) error {
	ch := make(chan error, 1)
	b.caller.Call(b.ops.Delete, &deleteArgs{ids: ids}, nil, func(err error) { ch <- err })
	return <-ch
}

// The capability wrappers below are thin on purpose: every method delegates to
// the matching callerBackend method, which is the ONE place each operation is
// written. What varies between them is only the method SET, because that is
// what view.New's `b.(BackendSaver)` assertion reads.
//
// The count is 2^n-1 for n optional capabilities: 3 types for two, 7 for three,
// and 15 if a fourth is ever added. That is the price of discovering
// capabilities by type assertion rather than by a runtime `Can(...)` check —
// and it is the right price here, because the assertion is what lets view.New
// return a Presenter carrying exactly the capabilities the remote side offers.
// A fourth capability is the moment to stop and reconsider, not to type out
// eight more structs.
type callerList struct {
	*callerBackend
}

func (b *callerList) List() ([]model.Model, error) {
	return b.list()
}

type callerSave struct {
	*callerBackend
}

func (b *callerSave) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerSave) Save(recs []model.Model) error {
	return b.save(recs)
}

type callerUpdate struct {
	*callerBackend
}

func (b *callerUpdate) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerUpdate) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

type callerDelete struct {
	*callerBackend
}

func (b *callerDelete) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerDelete) Delete(ids []string) error {
	return b.delete(ids)
}

type callerSaveUpdate struct {
	*callerBackend
}

func (b *callerSaveUpdate) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerSaveUpdate) Save(recs []model.Model) error {
	return b.save(recs)
}

func (b *callerSaveUpdate) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

type callerSaveDelete struct {
	*callerBackend
}

func (b *callerSaveDelete) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerSaveDelete) Save(recs []model.Model) error {
	return b.save(recs)
}

func (b *callerSaveDelete) Delete(ids []string) error {
	return b.delete(ids)
}

type callerUpdateDelete struct {
	*callerBackend
}

func (b *callerUpdateDelete) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerUpdateDelete) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

func (b *callerUpdateDelete) Delete(ids []string) error {
	return b.delete(ids)
}

type callerCRUD struct {
	*callerBackend
}

func (b *callerCRUD) List() ([]model.Model, error) {
	return b.list()
}

func (b *callerCRUD) Save(recs []model.Model) error {
	return b.save(recs)
}

func (b *callerCRUD) Update(ids []string, rec model.Model, fields []string) error {
	return b.update(ids, rec, fields)
}

func (b *callerCRUD) Delete(ids []string) error {
	return b.delete(ids)
}
