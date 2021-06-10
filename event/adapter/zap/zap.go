// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !disable_events

// zap provides an implementation of zapcore.Core for events.
// To use globally:
//     zap.ReplaceGlobals(zap.New(NewCore(exporter)))
//
// If you call elogging.SetExporter, then you can pass nil
// for the exporter above and it will use the global one.
package zap

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
	"golang.org/x/exp/event/severity"
)

type core struct {
	target    event.Target
	prototype event.Prototype
}

var _ zapcore.Core = (*core)(nil)

func NewCore(ctx context.Context) zapcore.Core {
	return &core{
		target:    event.To(ctx),
		prototype: event.Prototype{}.As(event.LogKind),
	}
}

func (c *core) Enabled(level zapcore.Level) bool {
	return true
}

func (c *core) With(fields []zapcore.Field) zapcore.Core {
	c2 := *c
	for _, f := range fields {
		c2.prototype = c2.prototype.Label(newLabel(f))
	}
	return &c2
}

func (c *core) Write(e zapcore.Entry, fs []zapcore.Field) error {
	b := c.target.With(c.prototype)
	if !b.Active() {
		return nil
	}
	b = b.
		At(e.Time).
		Label(convertLevel(e.Level).Label()).
		Name(e.LoggerName)
	// TODO: add these additional labels more efficiently.
	if e.Stack != "" {
		b.Label(keys.String("stack").Of(e.Stack))
	}
	if e.Caller.Defined {
		b.Label(keys.String("caller").Of(e.Caller.String()))
	}
	for _, f := range fs {
		b = b.Label(newLabel(f))
	}

	b.Message(e.Message).Send()
	return nil
}

func (c *core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, c)
}

func (c *core) Sync() error { return nil }

func newLabel(f zap.Field) event.Label {
	switch f.Type {
	case zapcore.ArrayMarshalerType, zapcore.ObjectMarshalerType, zapcore.BinaryType, zapcore.ByteStringType,
		zapcore.Complex128Type, zapcore.Complex64Type, zapcore.TimeFullType, zapcore.ReflectType,
		zapcore.ErrorType:
		return keys.Value(f.Key).Of(f.Interface)
	case zapcore.DurationType:
		// TODO: avoid this allocation?
		return keys.Value(f.Key).Of(time.Duration(f.Integer))
	case zapcore.Float64Type:
		return keys.Float64(f.Key).Of(math.Float64frombits(uint64(f.Integer)))
	case zapcore.Float32Type:
		return keys.Float32(f.Key).Of(math.Float32frombits(uint32(f.Integer)))
	case zapcore.BoolType:
		b := false
		if f.Integer != 0 {
			b = true
		}
		return keys.Bool(f.Key).Of(b)
	case zapcore.Int64Type:
		return keys.Int64(f.Key).Of(f.Integer)
	case zapcore.Int32Type:
		return keys.Int32(f.Key).Of(int32(f.Integer))

		//, zapcore.Int16Type, zapcore.Int8Type,
		// 		zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type, zapcore.UintptrType:
		//		return (f.Key).Of(uint64(f.Integer))
	case zapcore.StringType:
		return keys.String(f.Key).Of(f.String)
	case zapcore.TimeType:
		key := keys.Value(f.Key)
		t := time.Unix(0, f.Integer)
		if f.Interface != nil {
			t = t.In(f.Interface.(*time.Location))
		}
		return key.Of(t)
	case zapcore.StringerType:
		return keys.String(f.Key).Of(stringerToString(f.Interface))
	case zapcore.NamespaceType:
		// TODO: ???
		return event.Label{}
	case zapcore.SkipType:
		// TODO: avoid creating a label at all in this case.
		return event.Label{}
	default:
		panic(fmt.Sprintf("unknown field type: %v", f))
	}
}

// Adapter from encodeStringer in go.uber.org/zap/zapcore/field.go.
func stringerToString(stringer interface{}) (s string) {
	// Try to capture panics (from nil references or otherwise) when calling
	// the String() method, similar to https://golang.org/src/fmt/print.go#L540
	defer func() {
		if err := recover(); err != nil {
			// If it's a nil pointer, just say "<nil>". The likeliest causes are a
			// Stringer that fails to guard against nil or a nil pointer for a
			// value receiver, and in either case, "<nil>" is a nice result.
			if v := reflect.ValueOf(stringer); v.Kind() == reflect.Ptr && v.IsNil() {
				s = "<nil>"
				return
			}
			s = fmt.Sprintf("PANIC=%v", err)
		}
	}()

	return stringer.(fmt.Stringer).String()
}

func convertLevel(level zapcore.Level) severity.Level {
	switch level {
	case zapcore.DebugLevel:
		return severity.Debug
	case zapcore.InfoLevel:
		return severity.Info
	case zapcore.WarnLevel:
		return severity.Warning
	case zapcore.ErrorLevel:
		return severity.Error
	case zapcore.DPanicLevel:
		return severity.Fatal - 1
	case zapcore.PanicLevel:
		return severity.Fatal + 1
	case zapcore.FatalLevel:
		return severity.Fatal
	default:
		return severity.Trace
	}
}
