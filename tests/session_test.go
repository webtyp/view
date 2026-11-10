package tests

import (
	"testing"

	"webtyp.com/view"
)

type recordingLayout struct {
	got view.Session
}

func (l *recordingLayout) Show(s view.Session) {
	l.got = s
}

func TestLayout_ADrawsWhatItIsGiven(t *testing.T) {
	var l view.Layout = &recordingLayout{}
	session := view.Session{
		Brand:   view.Brand{Name: "acme"},
		Screens: []view.Screen{{ID: "a"}},
	}
	l.Show(session)

	rec := l.(*recordingLayout)
	if rec.got.Brand.Name != "acme" {
		t.Errorf("expected Brand.Name 'acme', got %q", rec.got.Brand.Name)
	}
	if len(rec.got.Screens) != 1 {
		t.Errorf("expected 1 screen, got %d", len(rec.got.Screens))
	}
}
