// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"testing"
)

var (
	baseline = Hooks{
		AStart: func(ctx context.Context, a int) context.Context { return ctx },
		AEnd:   func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context { return ctx },
		BEnd:   func(ctx context.Context) {},
	}

	stdlibLog = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			logCtx(ctx).Printf(aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			logCtx(ctx).Printf(bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	}

	stdlibPrintf = Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			ctxPrintf(ctx, aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			ctxPrintf(ctx, bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	}
)

func BenchmarkBaseline(b *testing.B) {
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context { return ctx },
		AEnd:   func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context { return ctx },
		BEnd:   func(ctx context.Context) {},
	})
}

type stdlibLogKey struct{}

func logCtx(ctx context.Context) *log.Logger {
	return ctx.Value(stdlibLogKey{}).(*log.Logger)
}

func stdlibLogger(w io.Writer) context.Context {
	logger := log.New(w, "", log.LstdFlags)
	return context.WithValue(context.Background(), stdlibLogKey{}, logger)
}

func stdlibLoggerNoTime(w io.Writer) context.Context {
	// there is no way to fixup the time, so we have to suppress it
	logger := log.New(w, "", 0)
	return context.WithValue(context.Background(), stdlibLogKey{}, logger)
}

type writerKey struct{}

func ctxPrintf(ctx context.Context, msg string, args ...interface{}) {
	ctx.Value(writerKey{}).(func(string, ...interface{}))(msg, args...)
}

func stdlibWriter(w io.Writer) context.Context {
	now := repeatableNow()
	return context.WithValue(context.Background(), writerKey{},
		func(msg string, args ...interface{}) {
			fmt.Fprint(w, now().Format(timeFormat), " ")
			fmt.Fprintf(w, msg, args...)
			fmt.Fprintln(w)
		},
	)
}

func BenchmarkLogStdlib(b *testing.B) {
	runBenchmark(b, stdlibLogger(ioutil.Discard), stdlibLog)
}

func BenchmarkLogPrintf(b *testing.B) {
	runBenchmark(b, stdlibWriter(io.Discard), stdlibPrintf)
}

func TestLogStdlib(t *testing.T) {
	testBenchmark(t, stdlibLoggerNoTime, stdlibLog, `
A where a=0
b where b="A value"
A where a=1
b where b="Some other value"
A where a=22
b where b="Some other value"
A where a=333
b where b=""
A where a=4444
b where b="prime count of values"
A where a=55555
b where b="V"
A where a=666666
b where b="A value"
A where a=7777777
b where b="A value"
`)
}

func TestLogPrintf(t *testing.T) {
	testBenchmark(t, stdlibWriter, stdlibPrintf, `
2020/03/05 14:27:49 A where a=0
2020/03/05 14:27:50 b where b="A value"
2020/03/05 14:27:51 A where a=1
2020/03/05 14:27:52 b where b="Some other value"
2020/03/05 14:27:53 A where a=22
2020/03/05 14:27:54 b where b="Some other value"
2020/03/05 14:27:55 A where a=333
2020/03/05 14:27:56 b where b=""
2020/03/05 14:27:57 A where a=4444
2020/03/05 14:27:58 b where b="prime count of values"
2020/03/05 14:27:59 A where a=55555
2020/03/05 14:28:00 b where b="V"
2020/03/05 14:28:01 A where a=666666
2020/03/05 14:28:02 b where b="A value"
2020/03/05 14:28:03 A where a=7777777
2020/03/05 14:28:04 b where b="A value"
`)
}
