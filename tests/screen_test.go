package tests

import (
	"strings"
	"testing"

	"webtyp.com/dom"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

type fakeComponent struct{ dom.Element }

func comp() dom.Component { return &fakeComponent{} }

func leafPresenter() view.Presenter {
	return view.New(&conformance.FakeLister{}, &conformance.MockRecord{})
}

func leaf(id string) view.Screen {
	return view.Screen{ID: id, Label: "Services", Icon: "services", Permission: "service", Presenter: leafPresenter()}
}

// A module's screens as the renderer receives them: a group with two leaves.
func TestScreen_ValidGroupAndLeaves(t *testing.T) {
	services := leaf("catalog.services")
	group := view.Screen{
		ID: "catalog", Label: "Catalog", Icon: "catalog", Help: "Services and their specialties.",
		Children: []view.Screen{services, leaf("catalog.specialties")},
	}
	if err := group.Validate(); err != nil {
		t.Fatalf("valid group rejected: %v", err)
	}
}

func TestScreen_ValidCardLeaf(t *testing.T) {
	leaf := view.Screen{
		ID:         "booking",
		Label:      "Booking",
		Icon:       "booking",
		Permission: "reservation",
		Help:       "Book an appointment.",
		Main:       comp(),
	}
	if err := leaf.Validate(); err != nil {
		t.Fatalf("valid one-card leaf rejected: %v", err)
	}
	twoCards := leaf
	twoCards.Aside = view.Aside{Controls: comp(), List: comp(), Footer: comp()}
	twoCards.Focus = view.NewCardFocus()
	if err := twoCards.Validate(); err != nil {
		t.Fatalf("valid two-card leaf rejected: %v", err)
	}

	group := view.Screen{
		ID:       "g",
		Label:    "G",
		Icon:     "g",
		Help:     "h",
		Children: []view.Screen{leaf},
	}
	if err := group.Validate(); err != nil {
		t.Fatalf("valid group containing a card leaf rejected: %v", err)
	}
}

func TestScreen_InvalidShapesNameTheScreen(t *testing.T) {
	noPermission := leaf("x")
	noPermission.Permission = ""
	leafWithHelp := leaf("x")
	leafWithHelp.Help = "dup"
	groupWithPresenter := view.Screen{ID: "g", Label: "G", Icon: "g", Help: "h", Presenter: leafPresenter(), Children: []view.Screen{leaf("x")}}
	groupNoHelp := view.Screen{ID: "g", Label: "G", Icon: "g", Children: []view.Screen{leaf("x")}}
	badChild := view.Screen{ID: "g", Label: "G", Icon: "g", Help: "h", Children: []view.Screen{noPermission}}

	compBoth := leaf("x")
	compBoth.Main = comp()
	compNoHelp := view.Screen{ID: "x", Label: "X", Icon: "x", Permission: "p", Main: comp()}
	compNoPermission := view.Screen{ID: "x", Label: "X", Icon: "x", Main: comp(), Help: "h"}
	groupWithComponent := view.Screen{ID: "g", Label: "G", Icon: "g", Help: "h", Main: comp(), Children: []view.Screen{leaf("x")}}
	asideNoMain := leaf("x")
	asideNoMain.Aside = view.Aside{List: comp()}
	asideNoList := view.Screen{ID: "x", Label: "X", Icon: "x", Permission: "p", Help: "h", Main: comp(), Aside: view.Aside{Controls: comp()}}
	focusNoAside := view.Screen{ID: "x", Label: "X", Icon: "x", Permission: "p", Help: "h", Main: comp(), Focus: view.NewCardFocus()}

	cases := []struct {
		name   string
		screen view.Screen
		want   string
	}{
		{"no id", view.Screen{}, "no ID"},
		{"no label", view.Screen{ID: "x"}, "no Label: x"},
		{"no icon", view.Screen{ID: "x", Label: "X"}, "no Icon: x"},
		{"leaf without presenter", view.Screen{ID: "x", Label: "X", Icon: "x"}, "needs a Presenter or a Main card: x"},
		{"both", compBoth, "both a Presenter and a Main card: x"},
		{"main without help", compNoHelp, "with a Main card needs Help: x"},
		{"main without permission", compNoPermission, "needs a Permission: x"},
		{"aside without main", asideNoMain, "needs a Main card: x"},
		{"aside without list", asideNoList, "needs its List: x"},
		{"focus without aside", focusNoAside, "needs an Aside: x"},
		{"leaf without permission", noPermission, "needs a Permission: x"},
		{"leaf with Help", leafWithHelp, "not Help: x"},
		{"group with presenter", groupWithPresenter, "cannot have a Presenter, cards or Permission: g"},
		{"group with a card", groupWithComponent, "cannot have a Presenter, cards or Permission: g"},
		{"group without help", groupNoHelp, "needs Help: g"},
		{"invalid child", badChild, "needs a Permission: x"},
	}
	for _, c := range cases {
		err := c.screen.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error containing %q", c.name, err, c.want)
		}
	}
}
