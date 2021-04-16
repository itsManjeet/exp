// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elogr

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
	"golang.org/x/exp/log-adapters/internal"
)

func TestInfo(t *testing.T) {
	e, th := internal.NewTestExporter()
	log := NewLogger(e, "/").WithName("n").V(3)
	log = log.WithName("m")
	log.Info("mess", "traceID", 17, "resource", "R")
	want := &event.Event{
		Kind:    event.LogKind,
		At:      internal.TestAt,
		ID:      1,
		Message: "mess",
		Labels: []event.Label{
			internal.LevelKey.Of(3),
			internal.NameKey.Of("n/m"),
			keys.Value("traceID").Of(17),
			keys.Value("resource").Of("R"),
		},
	}
	if diff := cmp.Diff(want, &th.Got, internal.CmpOptions...); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
