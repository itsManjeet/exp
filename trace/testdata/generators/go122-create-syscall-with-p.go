// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tests a G being created from within a syscall.
//
// Specifically, it tests a scenerio wherein a C
// thread is calling into Go, creating a goroutine in
// a syscall (in the tracer's model). Because the actual
// m can be reused, it's possible for that m to have never
// had its P (in _Psyscall) stolen.
//
// This is a regression test. The trace parser once required
// GoCreateSyscall to not have a P, but it can in the scenario
// described above.

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
	t.DisableTimestamps()

	g := t.Generation(1)

	// A C thread calls into Go and acquires a P. It returns
	// back to C, destroying the G. It then comes back to Go
	// on the same thread and again returns to C.
	//
	// Note: on pthread platforms this can't happen on the
	// same thread because the m is stashed in TLS between
	// calls into Go, until the thread dies. This is still
	// possible on other platforms, however.
	b0 := g.Batch(trace.ThreadID(0), 0)
	b0.Event("GoCreateSyscall", trace.GoID(4))
	b0.Event("ProcStatus", trace.ProcID(0), go122.ProcIdle)
	b0.Event("ProcStart", trace.ProcID(0), testgen.Seq(1))
	b0.Event("GoSyscallEndBlocked")
	b0.Event("GoStart", trace.GoID(4), testgen.Seq(1))
	b0.Event("GoSyscallBegin", testgen.Seq(2), testgen.NoStack)
	b0.Event("GoDestroySyscall")
	b0.Event("GoCreateSyscall", trace.GoID(4))
	b0.Event("GoSyscallEnd")
	b0.Event("GoSyscallBegin", testgen.Seq(3), testgen.NoStack)
	b0.Event("GoDestroySyscall")
}
