package tests

import (
	"testing"

	"webtyp.com/view"
)

// The module asks for a card; once the renderer connects the cards, the request reaches it.
func TestCardFocus_ShowsTheCardTheModuleAsksFor(t *testing.T) {
	var shown string
	f := view.NewCardFocus()
	f.ConnectCards(func() { shown = "main" }, func() { shown = "aside" })
	f.ShowMain()
	if shown != "main" {
		t.Fatalf("ShowMain showed %q", shown)
	}
	f.ShowAside()
	if shown != "aside" {
		t.Fatalf("ShowAside showed %q", shown)
	}
}

// A card asked for before the screen is drawn is not lost: it is shown when the renderer connects the cards.
func TestCardFocus_KeepsARequestMadeBeforeTheScreenIsDrawn(t *testing.T) {
	var shown string
	f := view.NewCardFocus()
	f.ShowMain()
	f.ConnectCards(func() { shown = "main" }, func() { shown = "aside" })
	if shown != "main" {
		t.Fatalf("the request made before drawing was lost, shown %q", shown)
	}
}
