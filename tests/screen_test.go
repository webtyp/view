package tests

import (
	"strings"
	"testing"

	"webtyp.com/view"
	"webtyp.com/view/conformance"
)

func leafPresenter() view.Presenter {
	return view.New(&conformance.FakeLister{}, &conformance.MockRecord{})
}

func leaf(id string) view.Screen {
	return view.Screen{ID: id, Label: "Services", Icon: "services", Permission: "service", Presenter: leafPresenter()}
}

// A module's screens as the renderer receives them: a group with two leaves,
// one of them filling a select from another list.
func TestScreen_ValidGroupAndLeaves(t *testing.T) {
	services := leaf("catalog.services")
	services.Lookups = []view.Lookup{{Field: "specialty_id", Source: leafPresenter()}}
	group := view.Screen{
		ID: "catalog", Label: "Catalog", Icon: "catalog", Help: "Services and their specialties.",
		Children: []view.Screen{services, leaf("catalog.specialties")},
	}
	if err := group.Validate(); err != nil {
		t.Fatalf("valid group rejected: %v", err)
	}
}

func TestScreen_InvalidShapesNameTheScreen(t *testing.T) {
	noPermission := leaf("x")
	noPermission.Permission = ""
	leafWithHelp := leaf("x")
	leafWithHelp.Help = "dup"
	badLookup := leaf("x")
	badLookup.Lookups = []view.Lookup{{Field: "zone_id"}}
	groupWithPresenter := view.Screen{ID: "g", Label: "G", Icon: "g", Help: "h", Presenter: leafPresenter(), Children: []view.Screen{leaf("x")}}
	groupNoHelp := view.Screen{ID: "g", Label: "G", Icon: "g", Children: []view.Screen{leaf("x")}}
	badChild := view.Screen{ID: "g", Label: "G", Icon: "g", Help: "h", Children: []view.Screen{noPermission}}

	cases := []struct {
		name   string
		screen view.Screen
		want   string
	}{
		{"no id", view.Screen{}, "no ID"},
		{"no label", view.Screen{ID: "x"}, "no Label: x"},
		{"no icon", view.Screen{ID: "x", Label: "X"}, "no Icon: x"},
		{"leaf without presenter", view.Screen{ID: "x", Label: "X", Icon: "x"}, "needs a Presenter: x"},
		{"leaf without permission", noPermission, "needs a Permission: x"},
		{"leaf with Help", leafWithHelp, "not Help: x"},
		{"incomplete lookup", badLookup, "lookup needs Field and Source: x"},
		{"group with presenter", groupWithPresenter, "cannot have a Presenter, Permission or Lookups: g"},
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
