// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"testing"
)

func BenchmarkBaseline(b *testing.B) {
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context { return ctx },
		AEnd:   func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context { return ctx },
		BEnd:   func(ctx context.Context) {},
	})
}

func BenchmarkLogStdlib(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			log.Printf(aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			log.Printf(bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func BenchmarkLogPrintf(b *testing.B) {
	out := ioutil.Discard
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			fmt.Fprintf(out, aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			fmt.Fprintf(out, bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}
