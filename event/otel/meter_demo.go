// +build ignore

package main

import (
	"context"
	"log"
	"time"

	"go.opentelemetry.io/otel/exporters/stdout"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"golang.org/x/exp/event"
)

func main() {
	ctx := context.Background()
	traceProvider, controller, err := stdout.InstallNewPipeline(nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer traceProvider.Shutdown(ctx)
	defer controller.Stop(ctx)
	h := metricHandler{}
	event.SetDefaultExporter(event.NewExporter(h))

	event.To(context.Background()).Metric()
	meter := metric.Must(controller.MeterProvider().Meter("demo"))
	hits := meter.NewInt64Counter("hits")
	hits.Add(ctx, 3)
	hits.Add(ctx, 1)

	glob := metric.Must(global.Meter("g"))
	f := glob.NewFloat64Counter("flots")
	f.Add(ctx, 3.14)
	time.Sleep(9 * time.Second)

}

type metricHandler struct{}

func (metricHandler) Metric(ctx context.Context, e *event.Event) {
	//fmt.Printf("%+v\n", e)
}
