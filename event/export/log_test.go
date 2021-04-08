// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package export_test

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/export"
	"golang.org/x/exp/event/keys"
)

func ExampleLog() {
	ctx := event.WithExporter(context.Background(), newTestExporter())
	anInt := keys.NewInt("myInt", "an integer")
	aString := keys.NewString("myString", "a string")
	event.Log(ctx, "my event", anInt.Of(6))
	event.Error(ctx, "error event", errors.New("an error"), aString.Of("some string value"))
	// Output:
	// 2020/03/05 14:27:48 [1] my event
	// 	myInt=6
	// 2020/03/05 14:27:48 [2] error event: an error
	// 	myString="some string value"
}

type testExporter struct {
	logger event.Exporter
	at     time.Time
}

func newTestExporter() *testExporter {
	exporter := &testExporter{}
	exporter.at, _ = time.Parse(time.RFC3339Nano, "2020-03-05T14:27:48Z")
	exporter.logger = export.LogWriter(event.NewPrinter(os.Stdout), false)
	return exporter
}

func (e *testExporter) Export(ev event.Event) {
	ev.At = e.at
	e.logger.Export(ev)
}
