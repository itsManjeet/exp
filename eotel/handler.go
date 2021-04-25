package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/event"
)

type HandlerMux struct {
	handlers map[event.Kind]event.Handler
}

func (h *HandlerMux) Set(eh event.Handler, k event.Kind, ks ...event.Kind) {
	h.handlers[k] = eh
	for _, k := range ks {
		h.handlers[k] = eh
	}
}

func (h *HandlerMux) Handle(ctx context.Context, e *event.Event) context.Context {
	if eh, ok := h.handlers[e.Kind]; ok {
		return eh.Handle(ctx, e)
	}
	if eh, ok := h.handlers[event.UnknownKind]; ok {
		return eh.Handle(ctx, e)
	}
	fmt.Fprintf(os.Stderr, "no handler for event kind %s\n", e.Kind)
	return ctx
}

type OTelTraceHandler struct {
	tracer trace.Tracer
	mu     sync.Mutex
	spans  map[uint64]trace.Span
}

func NewOTelTraceHandler(t trace.Tracer) event.Handler {
	return &OTelTraceHandler{
		tracer: t,
		spans:  map[uint64]trace.Span{},
	}
}

func (h *OTelTraceHandler) Handle(ctx context.Context, e *event.Event) context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch e.Kind {
	case event.StartKind:
		// e.Message is the name of the span
		ctx, span := h.tracer.Start(ctx, e.Message)
		h.spans[e.ID] = span
		return ctx

	case event.AnnotateKind:
		span := h.spans[e.Parent]
		if span == nil {
			fmt.Fprintf(os.Stderr, "no span for parent ID %d\n", e.Parent)
			return ctx
		}
		var (
			name  string
			attrs []attribute.KeyValue
		)
		for _, l := range e.Labels {
			if l.Key() == "name" {
				name = l.UnpackString()
			} else {
				attrs = append(attrs, attribute.Any(l.Key(), l.UnpackValue()))
			}
		}
		span.AddEvent(name, trace.WithAttributes(attrs...))
		return ctx

	case event.EndKind:
		span := h.spans[e.Parent]
		if span == nil {
			fmt.Fprintf(os.Stderr, "no span for parent ID %d\n", e.Parent)
			return ctx
		}
		span.End()
		return ctx

	default:
		panic("bad event kind")
	}
}
