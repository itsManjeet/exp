// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import "context"

type contextKey struct{}

// NewContext returns a context that contains the given Logger.
// Use FromContext to retrieve the Logger.
func NewContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the Logger stored in ctx by NewContext, or the default
// Logger if there is none, and adds ctx to it. Handlers can retrieve the
// context with [Record.Context].
func FromContext(ctx context.Context) Logger {
	l, ok := ctx.Value(contextKey{}).(Logger)
	if !ok {
		l = Default()
	}
	l.ctx = ctx
	return l
}
