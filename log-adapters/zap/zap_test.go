// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ezap

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

func Test(t *testing.T) {
	te := &internal.TestExporter{}
	log := zap.New(NewCore(te), zap.Fields(zap.Int("traceID", 17), zap.String("resource", "R")))
	log = log.Named("n/m")
	log.Info("mess", zap.Float64("pi", 3.14))
	want := &event.Event{
		Kind:    event.LogKind,
		Message: "mess",
		Static:  [2]event.Label{internal.LevelKey.Of(0), internal.NameKey.Of("n/m")},
		Dynamic: []event.Label{
			internal.StringKeyUint64("traceID").Of(17),
			internal.StringKeyString("resource").Of("R"),
			internal.StringKeyFloat64("pi").Of(3.14),
		},
	}
	te.Got.At = want.At
	if diff := cmp.Diff(want, te.Got, internal.CmpOptions...); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
