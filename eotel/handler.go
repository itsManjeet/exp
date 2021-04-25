package main

import (
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/event"
)

type OtelTraceHandler struct {
	Tracer trace.Tracer
	mu     sync.Mutex
	spans  map[uint64]*span // key is ID of Start event
}

func (h *OtelTraceHandler) Handle(e *event.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch e.Kind {
	case event.StartKind:
		if h.spans == nil {
			h.spans = map[uint64]*span{}
		}
		h.spans[e.ID] = &span{}

	case event.EndKind:
		s := h.spans[e.Parent]
		if s == nil {
			fmt.Fprintf(os.Stderr, "Error: no span for parent ID %d", e.Parent)
			return
		}
		s.End()

	default:
		panic("bad event kind")
	}
}

type HandlerMux struct {
	handlers [256]event.Handler
}

func (h *HandlerMux) Handle(e *event.Event) {
	if kh := h.handlers[e.Kind]; kh != nil {
		kh.Handle(e)
		return
	}
	if kh := h.handlers[event.UnknownKind]; kh != nil {
		kh.Handle(e)
		return
	}
	fmt.Fprintf(os.Stderr, "no handler for event kind %s", e.Kind)
}

func (h *HandlerMux) Register(k event.Kind, hh event.Handler) {
	h.handlers[k] = hh
}

// func CombineHandlers(hs ...event.Handler) event.Handler {
// 	return ch{hs}
// }

// type ch struct {
// 	hs []event.Handler
// }

// func (h ch) Handle(e *event.Event) {
// 	for _, h := range h.hs {
// 		h.Handle(e)
// 	}
// }

type span struct {
	name   string
	tracer trace.Tracer
}

var _ trace.Span = (*span)(nil)

func (s *span) Tracer() trace.Tracer { return s.tracer }

// End completes the Span. The Span is considered complete and ready to be
// delivered through the rest of the telemetry pipeline after this method
// is called. Therefore, updates to the Span are not allowed after this
// method has been called.
func (s *span) End(options ...trace.SpanOption) {
}

// AddEvent adds an event with the provided name and options.
func (s *span) AddEvent(name string, options ...trace.EventOption) {
}

// IsRecording returns the recording state of the Span. It will return
// true if the Span is active and events can be recorded.
func (s *span) IsRecording() bool {
}

// RecordError will record err as an exception span event for this span. An
// additional call toSetStatus is required if the Status of the Span should
// be set to Error, this method does not change the Span status. If this
// span is not being recorded or err is nil than this method does nothing.
func (s *span) RecordError(err error, options ...trace.EventOption) {
}

// SpanContext returns the SpanContext of the Span. The returned
// SpanContext is usable even after the End has been called for the Span.
func (s *span) SpanContext() trace.SpanContext {
}

// SetStatus sets the status of the Span in the form of a code and a
// message. SetStatus overrides the value of previous calls to SetStatus
// on the Span.
func (s *span) SetStatus(code codes.Code, msg string) {
}

// SetName sets the Span name.
func (s *span) SetName(name string) { s.name = name }

// SetAttributes sets kv as attributes of the Span. If a key from kv
// already exists for an attribute of the Span it will be overwritten with
// the value contained in kv.
func (s *span) SetAttributes(kv ...attribute.KeyValue) {
	if !s.IsRecording() {
		return
	}

}
