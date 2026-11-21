package view

import (
	"webtyp.com/country"
	"webtyp.com/dom"
	"webtyp.com/events"
	"webtyp.com/lang"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

// RecordResult is the id and display label returned when another module's form saves a record (D7).
type RecordResult struct {
	ID    string
	Label string
}

// BrowserDeps is what every module's screens get in the browser. A new need becomes a field here, never a parameter.
//
//	Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }
type BrowserDeps struct {
	Caller          router.Caller
	Zone            time.Zone
	IDs             model.IDGenerator // makes the ids a screen needs (the browser side of the app's NewIDs)
	Countries       country.Catalog   // the installation's countries: the default and which come first (App.Countries); zero when the app set none
	Use             model.ModuleUse   // how the current user uses the module (D3); zero when not scoped
	LinkedRecordIDs []string          // record IDs linked to the current account for this module (D2)
	// OpenForm opens the creation form of another module (D7) and calls onSaved with the created record's id and label.
	OpenForm func(module string, onSaved func(RecordResult), onCancel func()) dom.Component
	// Notices are the notices the server sends to the signed-in user (sse.UserNotices): a topic with no data, after
	// which the screen reads what changed through Caller. nil when nobody is signed in.
	//
	//	if d.Notices != nil { d.Notices.Subscribe(clinicalencounter.AgendaChanged, func(events.Event) { reload() }) }
	Notices events.Subscriber
}

// ModuleBrowser is what a module adds to the browser. Zero on the server.
type ModuleBrowser struct {
	Screens func(d BrowserDeps) []Screen
}

// EntryScreen is what a sign-in mode gives the browser: its entry screen, and open=false when the mode is not open at
// the door the visit came through (it asks its own route, ARCHITECTURE D78). The chassis draws the screen with the
// app's name and logo.
type EntryScreen interface {
	EntryScreen(d BrowserDeps) (e Entry, open bool, err error)
}

// Entry is one entry screen as the mode fills it: the line under the title, the form, and the message line.
//
//	view.Entry{Subtitle: "Enter your key to continue", Form: loginForm.Render(), Message: loginMessage}
type Entry struct {
	Subtitle lang.Text
	Form     dom.Component
	Message  *dom.SignalString
}
