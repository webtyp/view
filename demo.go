package view

import (
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/fmt"
)

var (
	ErrDemoNoLabel    = fmt.Err("view.Demo has no Label: the page would have no title; set Label to the module's name")
	ErrDemoNoTimezone = fmt.Err("view.Demo has no Timezone: dates cannot be shown; set Timezone, e.g. \"America/Santiago\"")
	ErrDemoNoModules  = fmt.Err("view.Demo has no Modules: every screen would fail to load; add the module built in seed.Demo")
	ErrDemoNoScreens  = fmt.Err("view.Demo has no Screens: the menu would be empty; set Screens to the module's ui.Screens")
)

// Demo is a module's runnable demo: the module built on storage/mem with its seed data, and its screens.
// The module returns it from seed.Demo(); the webtyp daemon draws it with the chassis. The module never imports layout.
//
//	return view.Demo{Label: ui.DefaultLabel, Timezone: "America/Santiago", ActingAs: "u1",
//		Modules: []crud.Module{mod}, Migrations: []ddl.Step{{Name: chatroom.ModelName, Run: migrate.Migrate}},
//		Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }}, nil
type Demo struct {
	Label      string                       // required: the chassis titles the page "<Label> — demo"
	Timezone   string                       // required: IANA zone given to the screens as BrowserDeps.Zone
	ActingAs   string                       // the user id operations see in crud.Caller; "" = signed out
	Modules    []crud.Module                // required: the operations the screens call
	Migrations []ddl.Step                   // tables shown in «Database»; nil = no diagram
	Screens    func(d BrowserDeps) []Screen // required
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
