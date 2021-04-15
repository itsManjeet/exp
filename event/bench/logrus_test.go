// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func BenchmarkLogZap(b *testing.B) {
	logger := newZapLogger()
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			logger.Info(aMsg, zap.Int(aName, a))
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			logger.Info(aMsg, zap.String(bName, b))
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func BenchmarkLogZapf(b *testing.B) {
	logger := newZapLogger().Sugar()
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

func newZapLogger() *zap.Logger {
	ec := zap.NewProductionEncoderConfig()
	ec.EncodeDuration = zapcore.NanosDurationEncoder
	ec.EncodeTime = zapcore.EpochNanosTimeEncoder
	enc := zapcore.NewJSONEncoder(ec)
	return zap.New(zapcore.NewCore(
		enc,
		&syncDiscardWriter{},
		zap.InfoLevel,
	))
}
