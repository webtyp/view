package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/view"
)

func TestUser_CanFollowsTheGrants(t *testing.T) {
	u := view.User{ID: "u1", Grants: []model.Grant{{Resource: "patient", Actions: model.Read}}}
	if !u.Can("patient", model.Read) {
		t.Error("a grant of patient:r must allow reading patients")
	}
	if u.Can("patient", model.Update) {
		t.Error("a grant of patient:r must not allow updating patients")
	}
	if u.Can("staff", model.Read) {
		t.Error("a grant of patient:r must not allow reading staff")
	}
}

func TestUser_WildcardAllowsEveryResource(t *testing.T) {
	admin := view.User{ID: "a1", Grants: []model.Grant{{Resource: model.Wildcard, Actions: model.AllActions}}}
	if !admin.Can("staff", model.Delete) {
		t.Error("a wildcard grant must allow every resource and action, as on the server")
	}
}

func TestUser_ZeroUserCanNothing(t *testing.T) {
	if (view.User{}).Can("patient", model.Read) {
		t.Error("nobody signed in must be allowed nothing")
	}
}
