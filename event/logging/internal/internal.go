package internal

import (
	"context"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

var (
	// TODO: these should be in event/keys.
	LevelKey = keys.Int("level")
	NameKey  = keys.String("name")
	ErrorKey = keys.Value("error")
)

type TestHandler struct {
	Got event.Event
}

func (h *TestHandler) Handle(ctx context.Context, ev *event.Event) context.Context {
	h.Got = *ev
	h.Got.Labels = make([]event.Label, len(ev.Labels))
	copy(h.Got.Labels, ev.Labels)
	return ctx
}

var TestAt = time.Now()

func NewTestExporter() (*event.Exporter, *TestHandler) {
	te := &TestHandler{}
	e := event.NewExporter(te)
	e.Now = func() time.Time { return TestAt }
	return e, te
}
