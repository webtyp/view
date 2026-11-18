package view

import "webtyp.com/model"

// User is who is signed in, as the chassis draws it: zero when nobody is. Grants are the permissions the server
// answered for them; Can evaluates them with the same rule the server applies.
//
//	if s.User.ID == "" { /* entry screens */ } else if s.User.Can("patient", model.Read) { … }
type User struct {
	ID     string
	Name   string
	Avatar string
	Grants []model.Grant
}

// Can reports whether the user holds action on resource ("*" grants every resource, as on the server).
func (u User) Can(resource model.Resource, action model.Action) bool {
	for _, g := range u.Grants {
		if g.Matches(resource, action) {
			return true
		}
	}
	return false
}
