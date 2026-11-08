package view

import (
	"webtyp.com/fmt"
	"webtyp.com/msgtype"
)

// Duration is how long a notification stays before dismissing itself.
type Duration struct {
	millis func(msg string) int // nil = persistent
	auto   bool                 // built by Auto(): the caller left the length to the chassis
}

// IsAuto reports whether the duration was created with Auto().
func (d Duration) IsAuto() bool { return d.auto }

// HasTimer reports whether the duration auto-dismisses after a positive number of milliseconds.
func (d Duration) HasTimer() bool { return d.millis != nil }

// Millis computes the duration in milliseconds for msg, or 0 if persistent.
func (d Duration) Millis(msg string) int {
	if d.millis == nil {
		return 0
	}
	return d.millis(msg)
}

// Auto sizes the duration to the message (~350ms per word plus 2.5s, floored at 4s and capped at 10s).
func Auto() Duration {
	return Duration{auto: true, millis: func(msg string) int {
		words := fmt.Count(msg, " ") + 1
		ms := 2500 + words*350
		if ms < 4000 {
			return 4000
		}
		if ms > 10000 {
			return 10000
		}
		return ms
	}}
}

// Persistent keeps the notification until the user dismisses it.
func Persistent() Duration { return Duration{} }

// For pins an exact duration in milliseconds.
func For(ms int) Duration {
	return Duration{millis: func(string) int { return ms }}
}

// Notifier raises a notification in the chassis.
type Notifier interface {
	Notify(t msgtype.Type, msg string, d Duration)
}

// UsesNotifier is a screen component that tells the user «saved» or an error through the chassis.
//
//	func (r *RootView) UseNotifier(n view.Notifier) { r.notifier = n }
type UsesNotifier interface {
	UseNotifier(n Notifier)
}
