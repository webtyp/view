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

func TestScreen_ValidComponentLeaf(t *testing.T) {
	leaf := view.Screen{
		ID:         "booking",
		Label:      "Booking",
		Icon:       "booking",
		Permission: "reservation",
		Help:       "Book an appointment.",
		Component:  comp(),
	}
	if err := leaf.Validate(); err != nil {
		t.Fatalf("valid component leaf rejected: %v", err)
	}

	group := view.Screen{
		ID:       "g",
		Label:    "G",
		Icon:     "g",
		Help:     "h",
		Children: []view.Screen{leaf},
	}
	if err := group.Validate(); err != nil {
		t.Fatalf("valid group containing component leaf rejected: %v", err)
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
	compBoth.Component = comp()
	compNoHelp := view.Screen{ID: "x", Label: "X", Icon: "x", Permission: "p", Component: comp()}
	compNoPermission := view.Screen{ID: "x", Label: "X", Icon: "x", Component: comp(), Help: "h"}
	groupWithComponent := view.Screen{ID: "g", Label: "G", Icon: "g", Help: "h", Component: comp(), Children: []view.Screen{leaf("x")}}

	cases := []struct {
		name   string
		screen view.Screen
		want   string
	}{
		{"no id", view.Screen{}, "no ID"},
		{"no label", view.Screen{ID: "x"}, "no Label: x"},
		{"no icon", view.Screen{ID: "x", Label: "X"}, "no Icon: x"},
		{"leaf without presenter", view.Screen{ID: "x", Label: "X", Icon: "x"}, "needs a Presenter or a Component: x"},
		{"both", compBoth, "both a Presenter and a Component: x"},
		{"component without help", compNoHelp, "with a Component needs Help: x"},
		{"component without permission", compNoPermission, "needs a Permission: x"},
		{"leaf without permission", noPermission, "needs a Permission: x"},
		{"leaf with Help", leafWithHelp, "not Help: x"},
		{"group with presenter", groupWithPresenter, "cannot have a Presenter, Component or Permission: g"},
		{"group with component", groupWithComponent, "cannot have a Presenter, Component or Permission: g"},
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
