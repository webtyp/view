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
// The renderer draws the same outline around every screen and puts the content in
// cards it draws: one (Main) or two (Main and Aside). Each card scrolls inside
// itself. A module never draws a frame, a card or a scroll area.
//
// A Screen is either a leaf or a group (it has Children):
//
//   - list leaf: ID, Label, Icon, Permission and Presenter are required. The
//     renderer makes its cards (the form as Main, search/list/actions as Aside).
//     Its help text is the Presenter's Description() (view.WithDescription),
//     never a second copy here.
//   - card leaf: ID, Label, Icon, Permission, Main and Help are required; Aside
//     and Focus are optional. For a screen that is not a list.
//   - group: ID, Label, Icon, Help and Children are required; it has no
//     Presenter, cards or Permission of its own — the menu shows it only when
//     the user may see at least one child, and opening it opens the first one.
//
// Example:
//
//	view.Screen{ID: "booking", Label: "Booking", Icon: "booking", Permission: "reservation",
//		Help: descBooking, Main: bookingFlow}
//
//	focus := view.NewCardFocus()
//	view.Screen{ID: "encounter", Label: "Encounter", Icon: "encounter", Permission: "encounter",
//		Help: descEncounter, Main: form, Aside: view.Aside{Controls: picker, List: today}, Focus: focus}
type Screen struct {
	ID            string         // menu route and identity, unique in the app
	Label         lang.Text      // menu text
	Icon          string         // sprite symbol id (the value of an svg.Icon)
	Permission    model.Resource // leaf only: the resource the screen needs
	Presenter     Presenter      // leaf only: the records the screen lists and edits
	OptionSources []Presenter    // optional: presenters that feed Field.Options/Ref selects without adding a menu entry
	Main          dom.Component  // leaf only, instead of Presenter: what goes in the main card
	Aside         Aside          // optional with Main: the side card; the zero value is no side card
	Focus         *CardFocus     // optional with Aside: lets the module choose which card a phone shows
	Help          lang.Text      // group, or a card leaf: what the screen is for
	Children      []Screen       // group only
}

// Aside is the side card of a screen, in three bands the renderer stacks: Controls on top (a search, a picker),
// List in the middle (it scrolls inside the card), Footer at the bottom (actions). List is required; Controls and
// Footer are optional.
//
//	view.Aside{Controls: patientPicker, List: todayList}
type Aside struct {
	Controls dom.Component
	List     dom.Component
	Footer   dom.Component
}

// isEmpty reports the zero value: the screen has no side card.
func (a Aside) isEmpty() bool { return a.Controls == nil && a.List == nil && a.Footer == nil }

const (
	errScreenNoID        = "view: screen has no ID"
	errScreenNoLabel     = "view: screen has no Label: "
	errScreenNoIcon      = "view: screen has no Icon: "
	errLeafNoPresenter   = "view: screen without Children needs a Presenter or a Main card: "
	errLeafBoth          = "view: screen cannot have both a Presenter and a Main card: "
	errLeafNoPermission  = "view: screen with a Presenter needs a Permission: "
	errLeafHasHelp       = "view: screen with a Presenter takes its help from Presenter.Description (view.WithDescription), not Help: "
	errMainNoHelp        = "view: screen with a Main card needs Help: "
	errAsideNoMain       = "view: a side card (Aside) needs a Main card: "
	errAsideNoList       = "view: a side card (Aside) needs its List: "
	errFocusNoAside      = "view: Focus chooses between two cards and needs an Aside: "
	errGroupHasPresenter = "view: screen with Children cannot have a Presenter, cards or Permission: "
	errGroupNoHelp       = "view: screen with Children needs Help: "
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
		if s.Presenter != nil || s.Main != nil || !s.Aside.isEmpty() || s.Focus != nil || s.Permission != "" {
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
	if !s.Aside.isEmpty() && s.Main == nil {
		return fmt.Err(errAsideNoMain + s.ID)
	}
	if s.Presenter != nil && s.Main != nil {
		return fmt.Err(errLeafBoth + s.ID)
	}
	if s.Presenter == nil && s.Main == nil {
		return fmt.Err(errLeafNoPresenter + s.ID)
	}
	if s.Permission == "" {
		return fmt.Err(errLeafNoPermission + s.ID)
	}
	if s.Main != nil {
		if s.Help == "" {
			return fmt.Err(errMainNoHelp + s.ID)
		}
		if !s.Aside.isEmpty() && s.Aside.List == nil {
			return fmt.Err(errAsideNoList + s.ID)
		}
		if s.Focus != nil && s.Aside.isEmpty() {
			return fmt.Err(errFocusNoAside + s.ID)
		}
		return nil
	}
	if s.Help != "" {
		return fmt.Err(errLeafHasHelp + s.ID)
	}
	return nil
}
