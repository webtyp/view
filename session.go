package view

import (
	"webtyp.com/router"
	"webtyp.com/time"
)

// Session is everything appkit hands to a Layout once it has built the app's modules for the browser: the app does not know how it is drawn.
//
//	layout.Show(view.Session{Name: "Mjosefa", Caller: caller, Zone: zone, Screens: screens, User: who, SignOut: signOut, Log: log})
type Session struct {
	Name    string        // the app's name (appkit.App.Name): titles the entry screens and the chassis; the logo is the one derived from docs/logo/logo.png
	Caller  router.Caller // the connection to the server, for the screens
	Zone    time.Zone
	Screens []Screen // the menu, in the order of the app's modules (the first one opens first)

	User        User    // who is signed in; zero when nobody is
	Entries     []Entry // the entry screens of the sign-in modes open at this door, in the app's order; shown while User is zero
	ServerDown  bool    // asking who is signed in got no answer: the chassis says the server did not answer (D76), never the entry screens
	SignOut     func()  // ends the session; nil when the app has no sign-in
	IdleSeconds int     // seconds without activity before SignOut runs; 0 = 30 minutes

	Log func(...any) // where the layout writes warnings
}

// Layout is the layout an app chose, seen by appkit: it draws a Session. The app's main picks the implementation; appkit never imports it.
//
//	browser.Run(config.App, screens.Browser, layoutapp.New(devtools.Menu()))
type Layout interface {
	Show(Session)
}
