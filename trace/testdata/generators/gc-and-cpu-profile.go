// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tests a few things all at once, and the interactions are
// interesting.
// - Concurrent GC mark phase.
// - Force >1 generation for Go 1.22+.
// - CPU profile samples.

package main

import (
	"bytes"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"
)

var i uint64

func main() {
	go func() {
		for {
			i++
		}
	}()
	f, err := os.Create(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var pf bytes.Buffer
	pprof.StartCPUProfile(&pf)
	defer pprof.StopCPUProfile()

	trace.Start(f)
	defer trace.Stop()

	go func() {
		runtime.GC()
	}()
	time.Sleep(2 * time.Second)
}
