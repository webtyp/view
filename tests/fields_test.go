package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/view"
)

func TestFields_VisibleFieldsHidesInternalForRecordOwner(t *testing.T) {
	fields := []model.Field{
		{Name: "id", Type: model.Text()},
		{Name: "full_name", Type: model.Text()},
		{Name: "internal_note", Type: model.Text(), HiddenFromRecordOwner: true},
	}

	owner := view.VisibleFields(fields, model.SeesOwnRecords)
	if len(owner) != 2 || owner[0].Name != "id" || owner[1].Name != "full_name" {
		t.Fatalf("expected internal_note hidden for SeesOwnRecords, got %+v", owner)
	}

	staff := view.VisibleFields(fields, model.WorksWithAllRecords)
	if len(staff) != 3 {
		t.Fatalf("expected all 3 fields for WorksWithAllRecords, got %d", len(staff))
	}
}
