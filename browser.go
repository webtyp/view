package view

import (
	"webtyp.com/dom"
	"webtyp.com/lang"
	"webtyp.com/router"
	"webtyp.com/time"
)

// BrowserDeps is what every module's screens get in the browser. A new need becomes a field here, never a parameter.
//
//	Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }
type BrowserDeps struct {
	Caller router.Caller
	Zone   time.Zone
}

// ModuleBrowser is what a module adds to the browser. Zero on the server.
type ModuleBrowser struct {
	Screens func(d BrowserDeps) []Screen
	Entry   func(d BrowserDeps) EntryScreens // only the entry module (webtyp/access) sets it
}

// EntryScreens are what the browser shows before the menu. The chassis draws them with the app's name and logo.
// IdleSeconds locks an idle session (0 = 30 minutes).
type EntryScreens struct {
	Login       Entry
	Setup       Entry // shown while the server has no administrator
	IdleSeconds int
}

// Entry is one entry screen as the module fills it: the line under the title, the form, and the message line.
//
//	view.Entry{Subtitle: "Enter your key to continue", Form: loginForm.Render(), Message: loginMessage}
type Entry struct {
	Subtitle lang.Text
	Form     dom.Component
	Message  *dom.SignalString
}
