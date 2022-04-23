// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package log implements a full-featured structured logging package with
// levels. Its API allows a single log line to have both key-value pairs and
// string formatting.
//
// It is built on top of event.Log. This package is more convenient to use,
// but will be somewhat slower and will allocate.
package log

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"golang.org/x/exp/event"
	"golang.org/x/exp/event/severity"
)

// Labels is a list of event.Labels.
type Labels []event.Label

// With constructs a Labels from alternating key-value pairs.
func With(kvs ...any) Labels {
	return Labels(nil).With(kvs...)
}

// With constructs a Labels from alternating key-value pairs by appending
// to its receiver.
func (ls Labels) With(kvs ...any) Labels {
	if len(kvs)%2 != 0 {
		panic("args must be key-value pairs")
	}
	for i := 0; i < len(kvs); i += 2 {
		ls = append(ls, pairToLabel(kvs[i].(string), kvs[i+1]))
	}
	return ls
}

func pairToLabel(name string, value any) event.Label {
	if d, ok := value.(time.Duration); ok {
		return event.Duration(name, d)
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return event.String(name, v.String())
	case reflect.Bool:
		return event.Bool(name, v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return event.Int64(name, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return event.Uint64(name, v.Uint())
	case reflect.Float32, reflect.Float64:
		return event.Float64(name, v.Float())
	default:
		return event.Value(name, value)
	}
}

// Logf emits a log event at the given level with the given message.
func (l Labels) Logf(ctx context.Context, s severity.Level, format string, args ...any) {
	event.Log(ctx, fmt.Sprintf(format, args...), append(l, s.Label())...)
}

func (l Labels) Debugf(ctx context.Context, format string, args ...any) {
	l.Logf(ctx, severity.Debug, format, args...)
}

func (l Labels) Infof(ctx context.Context, format string, args ...any) {
	l.Logf(ctx, severity.Info, format, args...)
}

func (l Labels) Warningf(ctx context.Context, format string, args ...any) {
	l.Logf(ctx, severity.Warning, format, args...)
}

func (l Labels) Errorf(ctx context.Context, format string, args ...any) {
	l.Logf(ctx, severity.Error, format, args...)
}

func Logf(ctx context.Context, s severity.Level, format string, args ...any) {
	Labels(nil).Logf(ctx, s, format, args...)
}

func Debugf(ctx context.Context, ft string, args ...any)   { Labels(nil).Debugf(ctx, ft, args...) }
func Infof(ctx context.Context, ft string, args ...any)    { Labels(nil).Infof(ctx, ft, args...) }
func Warningf(ctx context.Context, ft string, args ...any) { Labels(nil).Warningf(ctx, ft, args...) }
func Errorf(ctx context.Context, ft string, args ...any)   { Labels(nil).Errorf(ctx, ft, args...) }
