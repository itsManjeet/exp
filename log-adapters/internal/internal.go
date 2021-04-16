package internal

import (
	"bytes"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

var (
	// TODO: these should be in event/keys.
	LevelKey = keys.Int("level")
	NameKey  = keys.String("name")
	ErrorKey = keys.Value("error")
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
}

type TestHandler struct {
	Got event.Event
}

func (h *TestHandler) Handle(ev *event.Event) {
	h.Got = *ev
	h.Got.Labels = make([]event.Label, len(ev.Labels))
	copy(h.Got.Labels, ev.Labels)
}

var TestAt = time.Now()

func NewTestExporter() (*event.Exporter, *TestHandler) {
	te := &TestHandler{}
	e := event.NewExporter(te)
	e.Now = func() time.Time { return TestAt }
	return e, te
}
