// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"log"
	"time"
)

var defaultLogger atomicValue[*Logger]

func init() {
	defaultLogger.set(&Logger{
		handler: &defaultHandler{},
	})
}

// Default returns the default Logger.
func Default() *Logger { return defaultLogger.get() }

// SetDefault makes l the default Logger.
// After this call, output from the log package's default Logger
// (as with [log.Print], etc.) will be logged at InfoLevel using l's Handler.
func SetDefault(l *Logger) {
	defaultLogger.set(l)
	log.SetOutput(&handlerWriter{l.Handler(), log.Flags()})
	log.SetFlags(0) // we want just the log message, no time or location
}

// handlerWriter is an io.Writer that calls a Handler.
// It is used to link the default log.Logger to the default slog.Logger.
type handlerWriter struct {
	h     Handler
	flags int
}

func (w *handlerWriter) Write(buf []byte) (int, error) {
	var depth int
	if w.flags&(log.Lshortfile|log.Llongfile) != 0 {
		depth = 2
	}
	r := MakeRecord(time.Now(), InfoLevel, string(buf[:len(buf)-1]), depth)
	return len(buf), w.h.Handle(r)
}

// A Logger generates Records and passes them to a Handler.
//
// Loggers are immutable; to create a new one, call [New] or [Logger.With].
type Logger struct {
	handler Handler // for structured logging
}

// Handler returns l's Handler.
func (l *Logger) Handler() Handler { return l.handler }

// With returns a new Logger whose handler's attributes are a concatenation of
// l's attributes and the given arguments, converted to Attrs as in
// [Logger.Log].
func (l *Logger) With(attrs ...any) *Logger {
	return &Logger{handler: l.handler.With(argsToAttrs(attrs))}
}

func argsToAttrs(args []any) []Attr {
	var r Record
	setAttrs(&r, args)
	return r.Attrs()
}

// New creates a new Logger with the given Handler.
func New(h Handler) *Logger { return &Logger{handler: h} }

// With calls Logger.With on the default logger.
func With(attrs ...any) *Logger {
	return Default().With(attrs...)
}

// Enabled reports whether l emits log records at level.
func (l *Logger) Enabled(level Level) bool {
	return l.Handler().Enabled(level)
}

// Log emits a log record with the current time and the given level and message.
// The Record's Attrs consist of the Logger's attributes followed by
// the Attrs specified by args.
//
// The attribute arguments are processed as follows:
//   - If an argument is an Attr, it is used as is.
//   - If an argument is a string and this is not the last argument,
//     the following argument is treated as the value and the two are combined
//     into an Attr.
//   - Otherwise, the argument is treated as a value with key "!BADKEY".
func (l *Logger) Log(level Level, msg string, args ...any) {
	l.LogDepth(0, level, msg, args...)
}

// LogDepth is like [Logger.Log], but accepts a call depth to adjust the
// file and line number in the log record. 0 refers to the caller
// of LogDepth; 1 refers to the caller's caller; and so on.
func (l *Logger) LogDepth(calldepth int, level Level, msg string, args ...any) {
	if !l.Enabled(level) {
		return
	}
	r := l.makeRecord(msg, level, calldepth)
	setAttrs(&r, args)
	_ = l.Handler().Handle(r)
}

var useSourceLine = true

// Temporary, for benchmarking.
// Eventually, getting the pc should be fast.
func disableSourceLine() { useSourceLine = false }

func (l *Logger) makeRecord(msg string, level Level, depth int) Record {
	if useSourceLine {
		depth += 5
	}
	return MakeRecord(time.Now(), level, msg, depth)
}

const badKey = "!BADKEY"

func setAttrs(r *Record, args []any) {
	i := 0
	for i < len(args) {
		switch x := args[i].(type) {
		case string:
			if i+1 >= len(args) {
				r.AddAttr(String(badKey, x))
			} else {
				r.AddAttr(Any(x, args[i+1]))
			}
			i += 2
		case Attr:
			r.AddAttr(x)
			i++
		default:
			// If the key is not a string or Attr, treat it as a value with a missing key.
			r.AddAttr(Any(badKey, x))
			i++
		}
	}
}

// LogAttrs is a more efficient version of [Logger.Log] that accepts only Attrs.
func (l *Logger) LogAttrs(level Level, msg string, attrs ...Attr) {
	l.LogAttrsDepth(0, level, msg, attrs...)
}

// LogAttrsDepth is like [Logger.LogAttrs], but accepts a call depth argument
// which it interprets like [Logger.LogDepth].
func (l *Logger) LogAttrsDepth(calldepth int, level Level, msg string, attrs ...Attr) {
	if !l.Enabled(level) {
		return
	}
	r := l.makeRecord(msg, level, calldepth)
	r.addAttrs(attrs)
	_ = l.Handler().Handle(r)
}

// Debug logs at DebugLevel.
func (l *Logger) Debug(msg string, args ...any) {
	l.LogDepth(0, DebugLevel, msg, args...)
}

// Info logs at InfoLevel.
func (l *Logger) Info(msg string, args ...any) {
	l.LogDepth(0, InfoLevel, msg, args...)
}

// Warn logs at WarnLevel.
func (l *Logger) Warn(msg string, args ...any) {
	l.LogDepth(0, WarnLevel, msg, args...)
}

// Error logs at ErrorLevel.
// If err is non-nil, Error appends Any("err", err)
// to the list of attributes.
func (l *Logger) Error(msg string, err error, args ...any) {
	if err != nil {
		// TODO: avoid the copy.
		args = append(args[:len(args):len(args)], Any("err", err))
	}
	l.LogDepth(0, ErrorLevel, msg, args...)
}

// Debug calls Logger.Debug on the default logger.
func Debug(msg string, args ...any) {
	Default().LogDepth(0, DebugLevel, msg, args...)
}

// Info calls Logger.Info on the default logger.
func Info(msg string, args ...any) {
	Default().LogDepth(0, InfoLevel, msg, args...)
}

// Warn calls Logger.Warn on the default logger.
func Warn(msg string, args ...any) {
	Default().LogDepth(0, WarnLevel, msg, args...)
}

// Error calls Logger.Error on the default logger.
func Error(msg string, err error, args ...any) {
	if err != nil {
		// TODO: avoid the copy.
		args = append(args[:len(args):len(args)], Any("err", err))
	}
	Default().LogDepth(0, ErrorLevel, msg, args...)
}

// Log calls Logger.Log on the default logger.
func Log(level Level, msg string, args ...any) {
	Default().LogDepth(0, level, msg, args...)
}

// LogAttrs calls Logger.LogAttrs on the default logger.
func LogAttrs(level Level, msg string, attrs ...Attr) {
	Default().LogAttrsDepth(0, level, msg, attrs...)
}
