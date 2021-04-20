// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package elogging

import (
	"sync"

	"golang.org/x/exp/event"
)

var (
	mu  sync.Mutex
	exp *event.Exporter
)

// SetExporter sets the default Exporter for all the logging packages
// under this one.
func SetExporter(e *event.Exporter) {
	mu.Lock()
	defer mu.Unlock()
	exp = e
}

// Exporter returns the default Exporter for all the logging packages
// under this one.
func Exporter() *event.Exporter {
	mu.Lock()
	defer mu.Unlock()
	return exp
}
