package view

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// indexEntry pairs a record with its id. A slice of these replaces what used
// to be a map[string]model.Model: this file compiles to WASM, and a map pulls
// TinyGo's hashing machinery into the browser binary.
type indexEntry struct {
	id  string
	rec model.Model
}

type core struct {
	lister            Lister
	record            model.Model
	title             string
	searchPlaceholder string

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

func (p *core) Reload(done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	p.lister.List(func(rows []model.Model, err error) {
		if err != nil {
			done(err)
			return
		}

		p.items = p.items[:0]
		p.index = make([]indexEntry, 0, len(rows))
		for _, row := range rows {
			iz, ok := row.(Itemizer)
			if !ok {
				done(fmt.Err("view: Reload: row type", rowName(row), "does not implement view.Itemizer"))
				return
			}
			it := iz.Item()
			p.items = append(p.items, it)
			p.index = append(p.index, indexEntry{id: it.ID, rec: row})
		}

		done(nil)
	})
}

// rowName names the row that failed an Itemizer/model.Model assertion in Reload,
// so the error points at the offending record. webtyp/fmt has no reflect-based
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

func (c *core) save(recs []model.Model, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	if len(recs) == 0 {
		done(fmt.Err("view: Save requires at least one record"))
		return
	}
	for _, rec := range recs {
		if model.IsNil(rec) {
			done(fmt.Err("view: Save payload is nil"))
			return
		}
	}

	b, ok := c.lister.(Saver)
	if !ok {
		done(fmt.Err("view: Save: lister does not implement view.Saver"))
		return
	}
	b.Save(recs, done)
}

func (c *core) update(ids []string, rec model.Model, fields []string, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	if len(ids) == 0 {
		done(fmt.Err("view: Update requires at least one id"))
		return
	}
	if len(fields) == 0 {
		done(fmt.Err("view: Update requires at least one field"))
		return
	}
	if model.IsNil(rec) {
		done(fmt.Err("view: Update record is nil"))
		return
	}

	b, ok := c.lister.(Updater)
	if !ok {
		done(fmt.Err("view: Update: lister does not implement view.Updater"))
		return
	}
	b.Update(ids, rec, fields, done)
}

func (c *core) delete(ids []string, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	if len(ids) == 0 {
		done(fmt.Err("view: Delete requires at least one id"))
		return
	}
	for _, id := range ids {
		if _, ok := c.lookup(id); !ok {
			done(fmt.Errf("view: Delete: unknown id %q", id))
			return
		}
	}

	b, ok := c.lister.(Deleter)
	if !ok {
		done(fmt.Err("view: Delete: lister does not implement view.Deleter"))
		return
	}
	b.Delete(ids, done)
}

// The capability wrappers below follow the pattern documented in lister.go:
// thin structs whose only difference is the method SET. See it before editing.
type saveable struct {
	*core
}

func (s *saveable) Save(recs []model.Model, done func(error)) {
	s.save(recs, done)
}

type updatable struct {
	*core
}

func (u *updatable) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	u.update(ids, rec, fields, done)
}

type deletable struct {
	*core
}

func (d *deletable) Delete(ids []string, done func(error)) {
	d.delete(ids, done)
}

type saveableUpdatable struct {
	*core
}

func (su *saveableUpdatable) Save(recs []model.Model, done func(error)) {
	su.save(recs, done)
}

func (su *saveableUpdatable) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	su.update(ids, rec, fields, done)
}

type saveableDeletable struct {
	*core
}

func (sd *saveableDeletable) Save(recs []model.Model, done func(error)) {
	sd.save(recs, done)
}

func (sd *saveableDeletable) Delete(ids []string, done func(error)) {
	sd.delete(ids, done)
}

type updatableDeletable struct {
	*core
}

func (ud *updatableDeletable) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	ud.update(ids, rec, fields, done)
}

func (ud *updatableDeletable) Delete(ids []string, done func(error)) {
	ud.delete(ids, done)
}

type crud struct {
	*core
}

func (c *crud) Save(recs []model.Model, done func(error)) {
	c.save(recs, done)
}

func (c *crud) Update(ids []string, rec model.Model, fields []string, done func(error)) {
	c.update(ids, rec, fields, done)
}

func (c *crud) Delete(ids []string, done func(error)) {
	c.delete(ids, done)
}
