package view

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// MountSave registers op as the save operation of a screen: it reads the records
// the screen sends ({"records":[…]}), calls save for each one in order, and
// answers the last one saved (save may fill its id). Errors go through
// rpc.WriteError: no records → 400; a save error → its own status.
//
//	view.MountSave(reg, OpUpsertDevice, func(d *Device) error { return m.SaveDevice(d) }).
//		Requires(ResourceDevice, model.Create|model.Update)
func MountSave[T any, PT interface {
	*T
	model.Model
}](reg router.OperationRegistry, op string, save func(PT) error) router.Route {
	return reg.Operation(op, func(ctx router.Context) {
		var in saveIn[T, PT]
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if len(in.recs) == 0 {
			rpc.WriteError(ctx, ErrNoRecords)
			return
		}
		var lastSaved PT
		for _, rec := range in.recs {
			if err := save(rec); err != nil {
				rpc.WriteError(ctx, err)
				return
			}
			lastSaved = rec
		}
		_ = ctx.Encode(lastSaved)
	}).Accepts(PT(new(T)))
}

// MountDelete registers op as the delete operation of a screen: it reads the ids
// the screen sends ({"ids":[…]}) and calls remove with them. No ids → 400.
//
//	view.MountDelete(reg, OpDeleteDevice, m.DeleteDevices).Requires(ResourceDevice, model.Delete)
func MountDelete(reg router.OperationRegistry, op string, remove func(ids []string) error) router.Route {
	return reg.Operation(op, func(ctx router.Context) {
		var in deleteIn
		if err := ctx.Decode(&in); err != nil {
			rpc.WriteError(ctx, ErrCannotRead)
			return
		}
		if len(in.ids) == 0 {
			rpc.WriteError(ctx, ErrNoIDs)
			return
		}
		if err := remove(in.ids); err != nil {
			rpc.WriteError(ctx, err)
			return
		}
		ctx.WriteStatus(204)
	})
}
