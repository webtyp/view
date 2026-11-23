package view

// CardFocus lets a screen with two cards choose which one a phone shows; on a wide screen both are on view and
// it does nothing. The module creates it, puts it in Screen.Focus and calls ShowMain or ShowAside when the user
// moves on (a patient picked → the form). A call made before the screen is drawn is kept and applied when the
// renderer connects it.
//
//	focus := view.NewCardFocus()
//	view.Screen{…, Main: form, Aside: view.Aside{List: patients}, Focus: focus}
//	focus.ShowMain() // after a patient is picked
type CardFocus struct {
	showMain, showAside func()
	pending             cardOnView
}

// cardOnView is the card a phone shows.
type cardOnView int

const (
	noCardRequested cardOnView = iota
	mainCardOnView
	asideCardOnView
)

// NewCardFocus is the focus of one screen's two cards.
func NewCardFocus() *CardFocus { return &CardFocus{} }

// ShowMain brings the main card into view on a phone.
func (f *CardFocus) ShowMain() { f.show(mainCardOnView) }

// ShowAside brings the side card back into view on a phone.
func (f *CardFocus) ShowAside() { f.show(asideCardOnView) }

func (f *CardFocus) show(card cardOnView) {
	switch {
	case card == mainCardOnView && f.showMain != nil:
		f.showMain()
	case card == asideCardOnView && f.showAside != nil:
		f.showAside()
	default:
		f.pending = card // not drawn yet: applied by ConnectCards
	}
}

// ConnectCards is the renderer's half: it hands over how to show each card, and the card a module asked for
// before the screen was drawn is shown now. A module never calls it.
//
//	screen.Focus.ConnectCards(cards.ShowMain, cards.ShowAside)
func (f *CardFocus) ConnectCards(showMain, showAside func()) {
	f.showMain, f.showAside = showMain, showAside
	if f.pending != noCardRequested {
		card := f.pending
		f.pending = noCardRequested
		f.show(card)
	}
}
