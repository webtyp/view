package view

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
)

// indexEntry pairs a record with its id. A slice of these replaces what used
// to be a map[string]model.Model: this file compiles to WASM, and a map pulls
// TinyGo's hashing machinery into the browser binary.
type indexEntry struct {
	id  string
	rec model.Model
}

type core struct {
	caller            router.Caller
	record            model.Model
	listOp            string
	newList           func() model.ModelSlice
	title             string
	searchPlaceholder string
	saveOp            string
	updateOp          string
	deleteOp          string
	args              func() model.Encodable

	items    []Item
	selected string
	index    []indexEntry
}

func (p *core) Title() string {
	return p.title
}

func (p *core) SearchPlaceholder() string {
	return p.searchPlaceholder
}

func (p *core) Record() model.Model {
	return p.record
}

func (p *core) Items() []Item {
	return p.items
}

func (p *core) Selected() string {
	return p.selected
}

func (p *core) Reload() error {
	var listArgs model.Encodable
	if p.args != nil {
		listArgs = p.args()
	}

	list := p.newList()
	dec, ok := list.(model.Decodable)
	if !ok {
		return fmt.Err("view: list returned by newList does not implement model.Decodable")
	}

	ch := make(chan error, 1)
	p.caller.Call(p.listOp, listArgs, dec, func(err error) { ch <- err })
	if err := <-ch; err != nil {
		return err
	}

	p.items = p.items[:0]
	p.index = make([]indexEntry, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		row := list.At(i)
		iz, ok := row.(Itemizer)
		if !ok {
			return fmt.Err("view: Reload: row type", rowName(row), "does not implement view.Itemizer")
		}
		m, ok := row.(model.Model)
		if !ok {
			return fmt.Err("view: Reload: row type", rowName(row), "does not implement model.Model")
		}
		it := iz.Item()
		p.items = append(p.items, it)
		p.index = append(p.index, indexEntry{id: it.ID, rec: m})
	}

	return nil
}

// rowName names the row that failed an Itemizer/model.Model assertion in Reload,
// so the error points at the offending record. tinywasm/fmt has no reflect-based
// type-name formatter (WASM-size discipline), so this uses model.ModuleNaming —
// the stable name ormc already generates for every domain record — when the row
// provides it.
func rowName(row model.Fielder) string {
	if mn, ok := row.(model.ModuleNaming); ok {
		return mn.ModelName()
	}
	return "<unnamed type>"
}

// lookup finds the record for id. Linear, and that is fine: a projected list
// holds tens to low hundreds of rows, so the scan costs microseconds — the map
// was never buying anything measurable, and it cost binary size on every page
// load.
func (c *core) lookup(id string) (model.Model, bool) {
	for _, entry := range c.index {
		if entry.id == id {
			return entry.rec, true
		}
	}
	return nil, false
}

func (p *core) Select(id string) model.Model {
	m, ok := p.lookup(id)
	if !ok {
		return nil
	}
	p.selected = id
	return m
}

func (p *core) Deselect() {
	p.selected = ""
}

func (p *core) Filter(term string) []Item {
	if term == "" {
		res := make([]Item, len(p.items))
		copy(res, p.items)
		return res
	}
	var filtered []Item
	for _, it := range p.items {
		if fmt.Matches(it.Label, term) || fmt.Matches(it.Description, term) {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

func (c *core) save(recs ...model.Model) error {
	if len(recs) == 0 {
		return fmt.Err("view: Save requires at least one record")
	}
	for _, rec := range recs {
		if model.IsNil(rec) {
			return fmt.Err("view: Save payload is nil")
		}
	}

	ch := make(chan error, 1)
	c.caller.Call(c.saveOp, &saveArgs{recs: recs}, nil, func(err error) { ch <- err })
	return <-ch
}

func (c *core) update(ids []string, rec model.Model, fields []string) error {
	if len(ids) == 0 {
		return fmt.Err("view: Update requires at least one id")
	}
	if len(fields) == 0 {
		return fmt.Err("view: Update requires at least one field")
	}
	if model.IsNil(rec) {
		return fmt.Err("view: Update record is nil")
	}

	ch := make(chan error, 1)
	c.caller.Call(c.updateOp, &updateArgs{ids: ids, fields: fields, rec: rec}, nil, func(err error) { ch <- err })
	return <-ch
}

func (c *core) delete(ids ...string) error {
	if len(ids) == 0 {
		return fmt.Err("view: Delete requires at least one id")
	}
	for _, id := range ids {
		if _, ok := c.lookup(id); !ok {
			return fmt.Errf("view: Delete: unknown id %q", id)
		}
	}

	ch := make(chan error, 1)
	c.caller.Call(c.deleteOp, &deleteArgs{ids: ids}, nil, func(err error) { ch <- err })
	return <-ch
}

// The capability wrappers below are thin on purpose: every method delegates to
// the matching core method, which is the ONE place each operation is written.
// What varies between them is only the method SET, because that is what a
// consumer's `if u, ok := p.(view.Updater); ok` reads.
//
// The count is 2^n-1 for n optional capabilities: 3 types for two, 7 for three,
// and 15 if a fourth is ever added. That is the price of discovering
// capabilities by type assertion rather than by a runtime `Can(...)` check —
// and it is the right price here, because the assertion is what lets a
// renderer decide at wiring time not to paint a control it could never run.
// A fourth capability is the moment to stop and reconsider, not to type out
// eight more structs.
type saveable struct {
	*core
}

func (s *saveable) Save(recs ...model.Model) error {
	return s.save(recs...)
}

type updatable struct {
	*core
}

func (u *updatable) Update(ids []string, rec model.Model, fields []string) error {
	return u.update(ids, rec, fields)
}

type deletable struct {
	*core
}

func (d *deletable) Delete(ids ...string) error {
	return d.delete(ids...)
}

type saveableUpdatable struct {
	*core
}

func (su *saveableUpdatable) Save(recs ...model.Model) error {
	return su.save(recs...)
}

func (su *saveableUpdatable) Update(ids []string, rec model.Model, fields []string) error {
	return su.update(ids, rec, fields)
}

type saveableDeletable struct {
	*core
}

func (sd *saveableDeletable) Save(recs ...model.Model) error {
	return sd.save(recs...)
}

func (sd *saveableDeletable) Delete(ids ...string) error {
	return sd.delete(ids...)
}

type updatableDeletable struct {
	*core
}

func (ud *updatableDeletable) Update(ids []string, rec model.Model, fields []string) error {
	return ud.update(ids, rec, fields)
}

func (ud *updatableDeletable) Delete(ids ...string) error {
	return ud.delete(ids...)
}

type crud struct {
	*core
}

func (c *crud) Save(recs ...model.Model) error {
	return c.save(recs...)
}

func (c *crud) Update(ids []string, rec model.Model, fields []string) error {
	return c.update(ids, rec, fields)
}

func (c *crud) Delete(ids ...string) error {
	return c.delete(ids...)
}
