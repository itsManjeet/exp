// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tests syscall P stealing.

package main

import (
	"golang.org/x/exp/trace"
	"golang.org/x/exp/trace/internal/event/go122"
	testgen "golang.org/x/exp/trace/internal/testgen/go122"
)

func main() {
	testgen.Main(gen)
}

func gen(t *testgen.Trace) {
	g := t.Generation(1)

	// One goroutine enters a syscall.
	b0 := g.Batch(trace.ThreadID(0), 0)
	b0.Event("ProcStatus", trace.ProcID(0), go122.ProcRunning)
	b0.Event("GoStatus", trace.GoID(1), trace.ThreadID(0), go122.GoRunning)
	b0.Event("GoSyscallBegin", testgen.Seq(1), testgen.NoStack)
	b0.Event("GoSyscallEndBlocked")

	// A running goroutine steals proc 0.
	b1 := g.Batch(trace.ThreadID(1), 0)
	b1.Event("ProcStatus", trace.ProcID(2), go122.ProcRunning)
	b1.Event("GoStatus", trace.GoID(2), trace.ThreadID(1), go122.GoRunning)
	b1.Event("ProcSteal", trace.ProcID(0), testgen.Seq(2), trace.ThreadID(0))
}
