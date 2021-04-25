package main

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/stdout"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

var theTracer trace.Tracer

func main() {
	ctx := context.Background()
	exporter, err := stdout.NewExporter(stdout.WithPrettyPrint())
	if err != nil {
		log.Fatalf("failed to initialize stdout export pipeline: %v", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
	defer tp.Shutdown(ctx)

	otel.SetTracerProvider(tp)

	//	runOtel(ctx)
	runEvents(ctx)
}

func runOtel(ctx context.Context) {
	fooKey := attribute.Key("foo")
	barKey := attribute.Key("bar")
	theTracer = otel.Tracer("")
	ctx = baggage.ContextWithValues(ctx,
		fooKey.String("foo1"),
		barKey.String("bar1"),
	)
	f(ctx)
}

func f(ctx context.Context) {
	ctx, span := theTracer.Start(ctx, "f")
	defer span.End()

	span.AddEvent("in f", trace.WithAttributes(attribute.Int("bogons", 100)))
	span.SetAttributes(attribute.Bool("working", true))

	g(ctx)
}

func g(ctx context.Context) {
	ctx, span := theTracer.Start(ctx, "g")
	defer span.End()

	span.SetAttributes(attribute.Int("gattr", 17))
}

func runEvents(ctx context.Context) {
	hm := &HandlerMux{}
	hm.Register(event.UnknownKind, event.NewPrinter(os.Stdout))
	th := &OtelTraceHandler{Tracer: otel.Tracer("event")}
	hm.Register(event.StartKind, th)
	hm.Register(event.EndKind, th)
	exp := event.NewExporter(hm)
	ctx = event.WithExporter(ctx, exp)
	fe(ctx)
}

func fe(ctx context.Context) {
	ctx, end := event.Start(ctx, "fe")
	defer end()

	ge(ctx)
}

func ge(ctx context.Context) {
	ctx, end := event.Start(ctx, "ge")
	defer end()

	event.To(ctx).With(keys.Int("count").Of(17)).Annotate()
}
