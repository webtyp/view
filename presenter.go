
package view

import (
	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/model"
)

var (
	opSavePrefix   = "view: Save"
	opUpdatePrefix = "view: Update"
	opDeletePrefix = "view: Delete"
	identLister    = "lister"
	identID        = "id"
)

type core struct {
	lister            Lister
	record            model.Model
	title             string
	searchPlaceholder lang.Text
	description       lang.Text
	emptyMessage      lang.Text

	order    []string // ids of the last List, in its order; ids put later go first
	selected string
}

func (p *core) Title() string {
	return p.title
}

func (p *core) SearchPlaceholder() lang.Text {
	return p.searchPlaceholder
}

func (p *core) Description() lang.Text {
	return p.description
}

func (p *core) EmptyMessage() lang.Text {
	return p.emptyMessage
}

func (p *core) Record() model.Model {
	return p.record
}

// Items: the rows of this model in the shared copy — those this presenter never
// listed (saved here or elsewhere, synced) first, then the listed ones in List order.
func (p *core) Items() []Item {
	all := p.lister.Records().All(p.record.ModelName())
	var fresh, listed []Item
	for _, rec := range all {
		if !containsID(p.order, model.RowOf(rec).ID) {
			fresh = append(fresh, itemOf(rec))
		}
	}
	for _, id := range p.order {
		if rec, ok := p.lister.Records().Get(p.record.ModelName(), id); ok {
			listed = append(listed, itemOf(rec))
		}
	}
	return append(fresh, listed...)
}

func (p *core) OnChange(fn func()) { p.lister.Records().OnChange(p.record.ModelName(), fn) }

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
		p.order = p.order[:0]
		for _, row := range rows {
			p.order = append(p.order, model.RowOf(row).ID)
		}
		p.lister.Records().Put(rows...)
		done(nil)
	})
}

func (c *core) lookup(id string) (model.Model, bool) {
	return c.lister.Records().Get(c.record.ModelName(), id)
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
	items := p.Items()
	if term == "" {
		res := make([]Item, len(items))
		copy(res, items)
		return res
	}
	var filtered []Item
	for _, it := range items {
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
		done(fmt.Err(opSavePrefix, "requires at least one", "record"))
		return
	}
	for _, rec := range recs {
		if model.IsNil(rec) {
			done(fmt.Err(opSavePrefix, "payload", "is nil"))
			return
		}
	}

	b, ok := c.lister.(Saver)
	if !ok {
		done(fmt.Err(opSavePrefix, ":", identLister, "does not implement", typeName("view.Saver")))
		return
	}
	b.Save(recs, done)
}

func (c *core) update(ids []string, rec model.Model, fields []string, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	if len(ids) == 0 {
		done(fmt.Err(opUpdatePrefix, "requires at least one", identID))
		return
	}
	if len(fields) == 0 {
		done(fmt.Err(opUpdatePrefix, "requires at least one", "field"))
		return
	}
	if model.IsNil(rec) {
		done(fmt.Err(opUpdatePrefix, "record", "is nil"))
		return
	}

	b, ok := c.lister.(Updater)
	if !ok {
		done(fmt.Err(opUpdatePrefix, ":", identLister, "does not implement", typeName("view.Updater")))
		return
	}
	b.Update(ids, rec, fields, done)
}

func (c *core) Actions() []Action {
	if runner, ok := c.lister.(ActionRunner); ok {
		return runner.Actions()
	}
	return nil
}

func (c *core) Run(op string, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	runner, ok := c.lister.(ActionRunner)
	if !ok {
		done(errUnknownAction(op))
		return
	}

	actions := runner.Actions()
	var action *Action
	for i := range actions {
		if actions[i].Op == op {
			action = &actions[i]
			break
		}
	}
	if action == nil {
		done(errUnknownAction(op))
		return
	}

	var args model.Encodable
	if action.Args != nil {
		records := make([]model.Model, 0, len(c.order))
		for _, id := range c.order {
			if rec, ok := c.lookup(id); ok {
				records = append(records, rec)
			}
		}
		args = action.Args(records)
	}

	runner.RunAction(op, args, func(err error) {
		if err != nil {
			done(err)
			return
		}
		c.Reload(done)
	})
}

func (c *core) delete(ids []string, done func(error)) {
	if done == nil {
		done = func(error) {}
	}
	if len(ids) == 0 {
		done(fmt.Err(opDeletePrefix, "requires at least one", identID))
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
		done(fmt.Err(opDeletePrefix, ":", identLister, "does not implement", typeName("view.Deleter")))
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

func (c *crud) OnChange(fn func()) {
	c.core.OnChange(fn)
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

func typeName(name string) string { return name }
