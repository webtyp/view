package view

import (
	"webtyp.com/dom"
	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/model"
)

// Screen is one entry a module offers to the app's menu, declared as data: the
// module says WHAT it shows and the app's renderer (appkit, with platformd and
// crudview) decides HOW. A module that declares its screens this way does not
// import any renderer, so a renderer release never forces a module release.
//
// A Screen is either a leaf (it has a Presenter) or a group (it has Children):
//
//   - list leaf: ID, Label, Icon, Permission and Presenter are required. Its help
//     text is the Presenter's Description() (view.WithDescription), never a
//     second copy here.
//   - component leaf: ID, Label, Icon, Permission, Component and Help are required;
//     no Presenter and no Lookups. For a screen that is not a list.
//   - group: ID, Label, Icon, Help and Children are required; it has no
//     Presenter, Component or Permission of its own — the menu shows it only when
//     the user may see at least one child, and opening it opens the first one.
//
// Example:
//
//	func Screens(caller router.Caller) []view.Screen {
//		return []view.Screen{{
//			ID: "service_catalog", Label: "Catalog", Icon: "service_catalog", Help: descCatalog,
//			Children: []view.Screen{{
//				ID: "service_catalog.services", Label: "Services", Icon: "service_catalog_services",
//				Permission: "service", Presenter: servicecatalog.NewView(caller),
//				Lookups: []view.Lookup{{Field: "specialty_id", Source: servicecatalog.NewSpecialtyView(caller)}},
//			}},
//		}}
//	}
type Screen struct {
	ID         string         // menu route and identity, unique in the app
	Label      lang.Text      // menu text
	Icon       string         // sprite symbol id (the value of an svg.Icon)
	Permission model.Resource // leaf only: the resource the screen needs
	Presenter  Presenter      // leaf only: the records the screen lists and edits
	Component  dom.Component  // leaf only, instead of Presenter: a screen that is not a list (a wizard, a chat)
	Lookups    []Lookup       // leaf only: select fields filled from another list
	Help       lang.Text      // group, or a Component leaf: what the screen is for
	Children   []Screen       // group only
}

// Lookup fills the options of one select field of a screen's form from
// another list: each of Source's items becomes an option (Item.ID → Item.Label),
// loaded when the screen is first opened.
type Lookup struct {
	Field  string    // the form field (model field name)
	Source Presenter // the list the options come from
}

const (
	errScreenNoID          = "view: screen has no ID"
	errScreenNoLabel       = "view: screen has no Label: "
	errScreenNoIcon        = "view: screen has no Icon: "
	errLeafNoPresenter     = "view: screen without Children needs a Presenter or a Component: "
	errLeafBoth            = "view: screen cannot have both a Presenter and a Component: "
	errLeafNoPermission    = "view: screen with a Presenter needs a Permission: "
	errLeafHasHelp         = "view: screen with a Presenter takes its help from Presenter.Description (view.WithDescription), not Help: "
	errComponentNoHelp     = "view: screen with a Component needs Help: "
	errComponentHasLookups = "view: screen with a Component cannot have Lookups: "
	errGroupHasPresenter   = "view: screen with Children cannot have a Presenter, Component, Permission or Lookups: "
	errGroupNoHelp         = "view: screen with Children needs Help: "
	errLookupIncomplete    = "view: lookup needs Field and Source: "
)

// Validate reports the first rule a Screen (and its children) breaks, naming
// the screen. A renderer calls it before drawing anything: a malformed screen
// is a startup error, never a silently empty menu entry.
//
// Example: if err := s.Validate(); err != nil { return err }
func (s Screen) Validate() error {
	if s.ID == "" {
		return fmt.Err(errScreenNoID)
	}
	if s.Label == "" {
		return fmt.Err(errScreenNoLabel + s.ID)
	}
	if s.Icon == "" {
		return fmt.Err(errScreenNoIcon + s.ID)
	}
	if len(s.Children) > 0 {
		if s.Presenter != nil || s.Component != nil || s.Permission != "" || len(s.Lookups) > 0 {
			return fmt.Err(errGroupHasPresenter + s.ID)
		}
		if s.Help == "" {
			return fmt.Err(errGroupNoHelp + s.ID)
		}
		for _, c := range s.Children {
			if err := c.Validate(); err != nil {
				return err
			}
		}
		return nil
	}
	if s.Presenter != nil && s.Component != nil {
		return fmt.Err(errLeafBoth + s.ID)
	}
	if s.Presenter == nil && s.Component == nil {
		return fmt.Err(errLeafNoPresenter + s.ID)
	}
	if s.Permission == "" {
		return fmt.Err(errLeafNoPermission + s.ID)
	}
	if s.Component != nil {
		if s.Help == "" {
			return fmt.Err(errComponentNoHelp + s.ID)
		}
		if len(s.Lookups) > 0 {
			return fmt.Err(errComponentHasLookups + s.ID)
		}
		return nil
	}
	if s.Help != "" {
		return fmt.Err(errLeafHasHelp + s.ID)
	}
	for _, l := range s.Lookups {
		if l.Field == "" || l.Source == nil {
			return fmt.Err(errLookupIncomplete + s.ID)
		}
	}
	return nil
}
