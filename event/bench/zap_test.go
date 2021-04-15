// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/sirupsen/logrus"
)

func BenchmarkLogrus(b *testing.B) {
	logger := newLogrusLogger()
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			logger.WithField(aName, a).Info(aMsg)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			logger.WithField(bName, b).Info(bMsg)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func BenchmarkLogrusf(b *testing.B) {
	logger := newLogrusLogger()
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			logger.Infof(aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			logger.Infof(bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func newLogrusLogger() logrus.Logger {
	return logrus.Logger{
		Out:   ioutil.Discard,
		Level: logrus.InfoLevel,
		Formatter: &logrus.TextFormatter{
			FullTimestamp:  true,
			DisableSorting: true,
			DisableColors:  true,
		},
	}
}
