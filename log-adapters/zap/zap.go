// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ezap provides an implementation of zapcore.Core for events.
package ezap

import (
	"fmt"
	"math"
	"reflect"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
	"golang.org/x/exp/log-adapters/internal"
)

type core struct {
	exporter event.Exporter
	labels   []event.Label
}

var _ zapcore.Core = (*core)(nil)

func NewCore(e event.Exporter) zapcore.Core {
	return &core{exporter: e}
}

func (c *core) Enabled(level zapcore.Level) bool {
	return true
}

func (c *core) With(fields []zapcore.Field) zapcore.Core {
	c2 := *c
	c2.labels = addLabels(c.labels, fields)
	return &c2
}

var (
	stackKey  = keys.NewString("stack", "stack trace")
	callerKey = keys.NewString("caller", "entry caller")
)

func (c *core) Write(e zapcore.Entry, fs []zapcore.Field) error {
	ev := internal.GetLogEvent()
	defer internal.PutLogEvent(ev)
	ev.At = e.Time
	ev.Message = e.Message
	ev.Static[0] = internal.LevelKey.Of(int(e.Level)) // TODO: convert zap level to general level
	ev.Static[1] = internal.NameKey.Of(e.LoggerName)
	ev.Dynamic = addLabels(c.labels, fs)
	// TODO: add these additional labels more efficiently.
	if e.Stack != "" {
		ev.Dynamic = append(ev.Dynamic, stackKey.Of(e.Stack))
	}
	if e.Caller.Defined {
		ev.Dynamic = append(ev.Dynamic, callerKey.Of(e.Caller.String()))
	}
	c.exporter.Export(ev)
	return nil
}

func (c *core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, c)
}

func (c *core) Sync() error { return nil }

// addLabels creates a new []event.Label with the given labels followed by the
// labels constructed from fields.
func addLabels(labels []event.Label, fields []zap.Field) []event.Label {
	ls := make([]event.Label, len(labels)+len(fields))
	n := copy(ls, labels)
	for i := 0; i < len(fields); i++ {
		ls[n+i] = newLabel(fields[i])
	}
	return ls
}

func newLabel(f zap.Field) event.Label {
	switch f.Type {
	case zapcore.ArrayMarshalerType, zapcore.ObjectMarshalerType, zapcore.BinaryType, zapcore.ByteStringType,
		zapcore.Complex128Type, zapcore.Complex64Type, zapcore.TimeFullType, zapcore.ReflectType,
		zapcore.ErrorType:
		return internal.StringKey(f.Key).Of(f.Interface)
	case zapcore.DurationType:
		// TODO: avoid this allocation?
		return internal.StringKey(f.Key).Of(time.Duration(f.Integer))
	case zapcore.Float64Type:
		return internal.StringKeyFloat64(f.Key).Of(math.Float64frombits(uint64(f.Integer)))
	case zapcore.Float32Type:
		return internal.StringKeyFloat64(f.Key).Of(float64(math.Float32frombits(uint32(f.Integer))))
	case zapcore.BoolType, zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type,
		zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type, zapcore.UintptrType:
		return internal.StringKeyUint64(f.Key).Of(uint64(f.Integer))
	case zapcore.StringType:
		return internal.StringKeyString(f.Key).Of(f.String)
	case zapcore.TimeType:
		key := internal.StringKey(f.Key)
		t := time.Unix(0, f.Integer)
		if f.Interface != nil {
			t = t.In(f.Interface.(*time.Location))
		}
		return key.Of(t)
	case zapcore.StringerType:
		return internal.StringKeyString(f.Key).Of(stringerToString(f.Interface))
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
