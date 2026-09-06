package conformance

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// Payload encodes args and returns every scalar it writes — at the top level,
// inside nested objects, and inside arrays — as key/value pairs in write order.
//
// It exists because what crosses a Caller is a model.Encodable, and since view
// began shipping batches that Encodable is an unexported envelope
// ({records:[…]}, {ids:[…]}, {ids,fields,record}). A test double used to reach
// its payload by type-asserting the concrete record; that stopped working, and
// the alternative — exporting view's envelope types — would widen a public
// surface just so tests can peek at it.
//
// Asserting the wire shape is also the better test: it checks what the server
// will actually receive, not which Go type happened to carry it.
//
// Values from an unnamed array element are keyed by the array's own name, so
// {ids: ["a","b"]} comes back as {ids,a},{ids,b}.
func Payload(args model.Encodable) []fmt.KeyValue {
	if args == nil || args.IsNil() {
		return nil
	}
	w := &payloadWriter{}
	args.EncodeFields(w)
	return w.out
}

// Has reports whether pairs contains key with value — the assertion a test
// almost always wants, without a loop at every call site.
func Has(pairs []fmt.KeyValue, key, value string) bool {
	for _, kv := range pairs {
		if kv.Key == key && kv.Value == value {
			return true
		}
	}
	return false
}

// payloadWriter is a model.FieldWriter that keeps what it is told instead of
// serializing it. A slice, never a map: order is part of what is asserted, and
// this package is imported by tests that build for wasm.
type payloadWriter struct {
	out    []fmt.KeyValue
	prefix string // the array name in scope, for unnamed elements
}

func (w *payloadWriter) put(name, val string) {
	if name == "" {
		name = w.prefix
	}
	w.out = append(w.out, fmt.KeyValue{Key: name, Value: val})
}

func (w *payloadWriter) String(name, val string)   { w.put(name, val) }
func (w *payloadWriter) Int(name string, v int64)  { w.put(name, fmt.Sprintf("%d", v)) }
func (w *payloadWriter) Float(name string, v float64) { w.put(name, fmt.Sprintf("%v", v)) }
func (w *payloadWriter) Bool(name string, v bool)  { w.put(name, fmt.Sprintf("%t", v)) }
func (w *payloadWriter) Bytes(name string, v []byte) { w.put(name, string(v)) }
func (w *payloadWriter) Null(name string)          { w.put(name, "") }
func (w *payloadWriter) Raw(name, val string)      { w.put(name, val) }

func (w *payloadWriter) Object(name string, val model.Encodable) {
	if val == nil || val.IsNil() {
		return
	}
	inner := &payloadWriter{prefix: name}
	val.EncodeFields(inner)
	w.out = append(w.out, inner.out...)
}

func (w *payloadWriter) Array(name string, n int) model.ArrayWriter {
	return &payloadArray{parent: w, name: name}
}

type payloadArray struct {
	parent *payloadWriter
	name   string
}

func (a *payloadArray) String(v string)  { a.parent.put(a.name, v) }
func (a *payloadArray) Int(v int64)      { a.parent.put(a.name, fmt.Sprintf("%d", v)) }
func (a *payloadArray) Float(v float64)  { a.parent.put(a.name, fmt.Sprintf("%v", v)) }
func (a *payloadArray) Bool(v bool)      { a.parent.put(a.name, fmt.Sprintf("%t", v)) }
func (a *payloadArray) Bytes(v []byte)   { a.parent.put(a.name, string(v)) }

func (a *payloadArray) Object(v model.Encodable) {
	if v == nil || v.IsNil() {
		return
	}
	inner := &payloadWriter{prefix: a.name}
	v.EncodeFields(inner)
	a.parent.out = append(a.parent.out, inner.out...)
}

// Close satisfies model.ArrayWriter. Nothing to flush: this writer accumulates
// into the parent as it goes.
func (a *payloadArray) Close() {}
