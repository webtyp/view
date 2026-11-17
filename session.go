package view

import (
	"webtyp.com/router"
	"webtyp.com/time"
)

// Session is everything appkit hands to a Layout once it has built the app's modules for the browser: the app does not know how it is drawn.
//
//	layout.Show(view.Session{Name: "Mjosefa", Caller: caller, Zone: zone, Screens: screens, Entry: entry, Log: log})
type Session struct {
	Name    string        // the app's name (appkit.App.Name): titles the entry screens and the chassis; the logo is the one derived from docs/logo/logo.png
	Caller  router.Caller // the connection to the server: the layout asks who the user is and signs out through it
	Zone    time.Zone
	Screens []Screen     // the menu, in the order of the app's modules (the first one opens first)
	Entry   EntryScreens // sign-in and first-setup screens; zero when no module sets them
	Log     func(...any) // where the layout writes warnings
}

// Layout is the layout an app chose, seen by appkit: it draws a Session. The app's main picks the implementation; appkit never imports it.
//
//	appkit.RunBrowser(config.App, config.Browser, layoutapp.New(devtools.Menu()))
type Layout interface {
	Show(Session)
}
