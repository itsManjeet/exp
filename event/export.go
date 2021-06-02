// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

package event

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// Exporter synchronizes the delivery of events to handlers.
type Exporter struct {
	Now func() time.Time

	mu            sync.Mutex
	handler       Handler
	lastEvent     uint64
	pcToNamespace map[uintptr]string
}

// contextKey is used as the key for storing a contextValue on the context.
type contextKeyType struct{}

var contextKey interface{} = contextKeyType{}

// contextValue is stored by value in the context to track the exporter and
// current parent event.
type contextValue struct {
	exporter *Exporter
	parent   uint64
}

type noopHandler struct{}

func (noopHandler) Handle(ctx context.Context, ev *Event) context.Context { return ctx }

var (
	defaultExporter unsafe.Pointer
)

// NewExporter creates an Exporter using the supplied handler.
// Event delivery is serialized to enable safe atomic handling.
func NewExporter(handler Handler) *Exporter {
	if handler == nil {
		handler = noopHandler{}
	}
	return &Exporter{
		Now:           time.Now,
		handler:       handler,
		pcToNamespace: map[uintptr]string{},
	}
}

func setDefaultExporter(e *Exporter) {
	atomic.StorePointer(&defaultExporter, unsafe.Pointer(e))
}

func getDefaultExporter() *Exporter {
	return (*Exporter)(atomic.LoadPointer(&defaultExporter))
}

func newContext(ctx context.Context, exporter *Exporter, parent uint64) context.Context {
	return context.WithValue(ctx, contextKey, contextValue{exporter: exporter, parent: parent})
}

// FromContext returns the exporter and current trace for the supplied context.
func FromContext(ctx context.Context) (*Exporter, uint64) {
	if v, ok := ctx.Value(contextKey).(contextValue); ok {
		return v.exporter, v.parent
	}
	return getDefaultExporter(), 0
}

// deliver events to the underlying handler.
// If the event does not have a timestamp, and the exporter has a Now function
// then the timestamp will be updated.
func (e *Exporter) deliver(ctx context.Context, ev *Event) context.Context {
	if e.Now != nil && ev.At.IsZero() {
		ev.At = e.Now()
	}
	var pc uintptr
	if ev.Namespace == "" {
		// Get the pc of the user function that delivered the event.
		// This is sensitive to the call stack.
		// 0: runtime.Callers
		// 1: Exporter.deliver (this function)
		// 2: Builder.deliver
		// 3: Builder.{Start,End,etc.}
		// 4: user function
		var pcs [1]uintptr
		runtime.Callers(4, pcs[:])
		pc = pcs[0]
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if pc != 0 {
		ns, ok := e.pcToNamespace[pc]
		if !ok {
			// If we call runtime.CallersFrames(pcs[:1]) in this function, the
			// compiler will think the pcs array escapes and will allocate.
			f := callerFrameFunction(pc)
			ns = namespace(f)
			e.pcToNamespace[pc] = ns
		}
		ev.Namespace = ns
	}
	return e.handler.Handle(ctx, ev)
}

func callerFrameFunction(pc uintptr) string {
	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	return frame.Function
}

func namespace(funcPath string) string {
	// Function is the fully-qualified function name. The name itself may
	// have dots (for a closure, for instance), but it can't have slashes.
	// So the package path ends at the first dot after the last slash.
	i := strings.LastIndexByte(funcPath, '/')
	if i < 0 {
		i = 0
	}
	end := strings.IndexByte(funcPath[i:], '.')
	if end >= 0 {
		end += i
	} else {
		end = len(funcPath)
	}
	return funcPath[:end]
}
