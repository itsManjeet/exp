// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bench_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func BenchmarkLogZerolog(b *testing.B) {
	logger := zerolog.New(&syncDiscardWriter{})
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			logger.Info().Int(aName, a).Msg(aMsg)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			logger.Info().Str(bName, b).Msg(bMsg)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}

func BenchmarkLogZerologf(b *testing.B) {
	logger := zerolog.New(&syncDiscardWriter{})
	runBenchmark(b, context.Background(), Hooks{
		AStart: func(ctx context.Context, a int) context.Context {
			logger.Info().Msgf(aMsgf, a)
			return ctx
		},
		AEnd: func(ctx context.Context) {},
		BStart: func(ctx context.Context, b string) context.Context {
			logger.Info().Msgf(bMsgf, b)
			return ctx
		},
		BEnd: func(ctx context.Context) {},
	})
}
