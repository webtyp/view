package view

import (
	"webtyp.com/country"
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/model"
)

// Errors of a demo that lacks something it needs. They are constants, not fmt.Err variables: only the development
// demo uses them, and a constant nobody uses adds nothing to an app's binary (langc still extracts model.Invalid).
const (
	ErrDemoNoLabel    model.Invalid = "a demo needs a page name"
	ErrDemoNoTimezone model.Invalid = "set the time zone so the demo can show dates"
	ErrDemoNoModules  model.Invalid = "give the demo its module, or every screen fails"
	ErrDemoNoScreens  model.Invalid = "add the screens of the module to the demo"
)

// Demo is a module's runnable demo: the module built on storage/mem with its seed data, and its screens.
// The module returns it from seed.Demo(); the webtyp daemon draws it with the chassis. The module never imports layout.
//
//	return view.Demo{Label: ui.DefaultLabel, Timezone: "UTC", ActingAs: "u1",
//		Modules: []crud.Module{mod}, Migrations: []ddl.Step{{Name: chatroom.ModelName, Run: migrate.Migrate}},
//		Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }}, nil
type Demo struct {
	Label      string                       // required: the chassis titles the page "<Label> — demo"
	Timezone   string                       // required: IANA zone given to the screens as BrowserDeps.Zone
	ActingAs   string                       // the user id operations see in crud.CurrentUser; "" = signed out
	Modules    []crud.Module                // required: the operations the screens call
	Migrations []ddl.Step                   // tables shown in «Database»; nil = no diagram
	Screens    func(d BrowserDeps) []Screen // required
	Countries  country.Catalog              // given to the screens as BrowserDeps.Countries; zero when the module needs none
}

// Validate reports the first missing required field.
func (d Demo) Validate() error {
	switch {
	case d.Label == "":
		return ErrDemoNoLabel
	case d.Timezone == "":
		return ErrDemoNoTimezone
	case len(d.Modules) == 0:
		return ErrDemoNoModules
	case d.Screens == nil:
		return ErrDemoNoScreens
	}
	return nil
}
