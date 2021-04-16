package internal

import (
	"bytes"
	"math"
	"sync"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

// StringKey implements event.Key for a string.
// We don't want to use an event/keys.Value because we don't want to allocate.
type StringKey string

var _ event.Key = StringKey("")

func (k StringKey) Name() string                         { return string(k) }
func (k StringKey) Of(value interface{}) event.Label     { return event.OfValue(k, value) }
func (k StringKey) From(l event.Label) interface{}       { return l.UnpackValue() }
func (k StringKey) Print(p event.Printer, l event.Label) { p.Value(k.From(l)) }

type StringKeyUint64 string

func (k StringKeyUint64) Name() string                         { return string(k) }
func (k StringKeyUint64) Of(u uint64) event.Label              { return event.Of64(k, u) }
func (k StringKeyUint64) From(l event.Label) uint64            { return l.Unpack64() }
func (k StringKeyUint64) Print(p event.Printer, l event.Label) { p.Uint(k.From(l)) }

type StringKeyFloat64 string

func (k StringKeyFloat64) Name() string                         { return string(k) }
func (k StringKeyFloat64) Of(f float64) event.Label             { return event.Of64(k, math.Float64bits(f)) }
func (k StringKeyFloat64) From(l event.Label) float64           { return math.Float64frombits(l.Unpack64()) }
func (k StringKeyFloat64) Print(p event.Printer, l event.Label) { p.Float(k.From(l)) }

type StringKeyString string

func (k StringKeyString) Name() string                         { return string(k) }
func (k StringKeyString) Of(s string) event.Label              { return event.OfString(k, s) }
func (k StringKeyString) From(l event.Label) string            { return l.UnpackString() }
func (k StringKeyString) Print(p event.Printer, l event.Label) { p.Quote(k.From(l)) }

var (
	// TODO: these should be in event/keys.
	LevelKey = keys.NewInt("level", "log level")
	NameKey  = keys.NewString("name", "log name")
)

// LabelString returns a string representation of the label.
func LabelString(l event.Label) string {
	var buf bytes.Buffer
	p := event.NewPrinter(&buf)
	p.Label(l)
	return buf.String()
}

var CmpOptions = []cmp.Option{
	cmp.Comparer(func(a, b event.Label) bool { return LabelString(a) == LabelString(b) }),
	cmp.AllowUnexported(event.Label{}, keys.String{}, keys.Int{}),
	cmpopts.IgnoreFields(event.Event{}, "At"),
}

type TestExporter struct {
	Got *event.Event
}

func (e *TestExporter) Export(ev *event.Event) {
	e.Got = ev
}

var eventPool = &sync.Pool{
	New: func() interface{} {
		return &event.Event{
			Kind:   event.LogKind,
			ID:     0,
			Parent: 0,
		}
	},
}

func GetLogEvent() *event.Event  { return eventPool.Get().(*event.Event) }
func PutLogEvent(e *event.Event) { eventPool.Put(e) }
