package view

import "webtyp.com/model"

// VisibleFields returns the fields visible to a caller with use, omitting HiddenFromRecordOwner fields
// when use is model.SeesOwnRecords.
func VisibleFields(fields []model.Field, use model.ModuleUse) []model.Field {
	if use != model.SeesOwnRecords {
		return fields
	}
	out := make([]model.Field, 0, len(fields))
	for _, f := range fields {
		if f.HiddenFromRecordOwner {
			continue
		}
		out = append(out, f)
	}
	return out
}
