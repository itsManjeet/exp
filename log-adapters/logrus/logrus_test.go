// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elogrus

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/event"
	"golang.org/x/exp/log-adapters/internal"
)

func Test(t *testing.T) {
	te := &internal.TestExporter{}
	log := logrus.New()
	log.SetFormatter(NewFormatter(te))
	log.SetOutput(io.Discard)
	// adding WithContext panics, because event.FromContext assumes
	log.WithContext(context.Background()).WithField("traceID", 17).WithField("resource", "R").Info("mess")

	want := &event.Event{
		Kind:    event.LogKind,
		Message: "mess",
		Static:  [2]event.Label{internal.LevelKey.Of(4)},
		Dynamic: []event.Label{
			internal.StringKey("traceID").Of(17),
			internal.StringKey("resource").Of("R"),
		},
	}
	te.Got.At = want.At
	// logrus fields are stored in a map, so we have to sort to overcome map iteration indeterminacy.
	less := func(a, b event.Label) bool { return a.Key().Name() < b.Key().Name() }
	if diff := cmp.Diff(want, te.Got, append([]cmp.Option{cmpopts.SortSlices(less)}, internal.CmpOptions...)...); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

}
