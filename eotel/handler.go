package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/event"
)

// Exporter exports OpenTelemetry spans.
type Exporter struct {
	uploader           batchUploader
	stopOnce           sync.Once
	stopCh             chan struct{}
	defaultServiceName string
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

type OTelTraceHandler struct {
	tracer         *tracer
	mu             sync.Mutex
	spanProcessors []sdktrace.SpanProcessor
	spans          map[uint64]*span // key is ID of Start event
}

func NewOTelTraceHandler() event.Handler {
	h := &OTelTraceHandler{
		spans: map[uint64]*span{},
	}
	h.tracer = &tracer{h}
	return h
}

func (h *OTelTraceHandler) RegisterSpanProcessor(sp sdktrace.SpanProcessor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.spanProcessors = append(h.spanProcessors, sp)
}

func (h *OTelTraceHandler) Handle(e *event.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch e.Kind {
	case event.StartKind:
		s := &span{
			name:   e.Message,
			tracer: h.tracer,
		}
		h.spans[e.ID] = s
		for _, sp := range h.spanProcessors {
			sp.OnStart(context.TODO(), s)
		}

	case event.AnnotateKind:
		if s := h.getSpan(e.Parent); s != nil {
			s.AddEvent(name)
		}

	case event.EndKind:
		if s := h.getSpan(e.Parent); s != nil {
			s.End()
		}

	default:
		panic("bad event kind")
	}
}

// Must be called with the lock held.
func (h *OTelTraceHandler) getSpan(id uint64) *span {
	s := h.spans[id]
	if s == nil {
		fmt.Fprintf(os.Stderr, "Error: no span for parent ID %d", id)
	}
	return s
}

type tracer struct {
	handler *OTelTraceHander
}

type span struct {
	name   string
	parent XXXX
	tracer *tracer
}

var _ trace.Span = (*span)(nil)

func (s *span) Tracer() trace.Tracer { return s.tracer }

// End completes the Span. The Span is considered complete and ready to be
// delivered through the rest of the telemetry pipeline after this method
// is called. Therefore, updates to the Span are not allowed after this
// method has been called.
func (s *span) End(options ...trace.SpanOption) {
	et := getEndTime()
	if !s.IsRecording() {
		return
	}
	// TODO: call safely without lock (use atomic.Value?)
	for _, sp := s.tracer.handler.spanProcessors {
		sp.OnEnd(s)
	}

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
