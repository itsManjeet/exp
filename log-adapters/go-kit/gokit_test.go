// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package egokit

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
	"golang.org/x/exp/log-adapters/internal"
)

func Test(t *testing.T) {
	e, h := internal.NewTestExporter()
	log := NewLogger(e)
	log.Log("msg", "mess", "level", 1, "name", "n/m", "traceID", 17, "resource", "R")
	want := &event.Event{
		Kind:    event.LogKind,
		ID:      1,
		At:      internal.TestAt,
		Message: "mess",
		Labels: []event.Label{
			keys.Value("level").Of(1),
			keys.Value("name").Of("n/m"),
			keys.Value("traceID").Of(17),
			keys.Value("resource").Of("R"),
		},
	}
	if diff := cmp.Diff(want, &h.Got, internal.CmpOptions...); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
