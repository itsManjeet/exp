// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slog/internal/buffer"
)

// A Handler processes log records produced by Logger output.
// Any of the Handler's methods may be called concurrently with itself
// or with other methods. It is the responsibility of the Handler to
// manage this concurrency.
type Handler interface {
	// Enabled reports whether this handler is accepting records
	// at the given level.
	Enabled(Level) bool

	// Handle processes the Record.
	// Handle methods that produce output should observe the following rules:
	//   - If r.Time() is the zero time, do not output it.
	//   - If r.Level() is Level(0), do not output it.
	Handle(Record) error

	// With returns a new Handler whose attributes consist of
	// the receiver's attributes concatenated with the arguments.
	With(attrs []Attr) Handler
}

type defaultHandler struct {
	attrs []Attr
}

func (*defaultHandler) Enabled(Level) bool { return true }

// Collect the level, attributes and message in a string and
// write it with the default log.Logger.
// Let the log.Logger handle time and file/line.
func (h *defaultHandler) Handle(r Record) error {
	var b strings.Builder
	if r.Level() > 0 {
		b.WriteString(r.Level().String())
		b.WriteByte(' ')
	}
	for i := 0; i < r.NumAttrs(); i++ {
		fmt.Fprint(&b, r.Attr(i)) // Attr.Format will print key=value
		b.WriteByte(' ')
	}
	b.WriteString(r.Message())
	return log.Output(4, b.String())
}

func (d *defaultHandler) With(as []Attr) Handler {
	d2 := *d
	d2.attrs = concat(d2.attrs, as)
	return &d2
}

// HandlerOptions are options for a TextHandler or JSONHandler.
// A zero HandlerOptions consists entirely of default values.
type HandlerOptions struct {
	// Add a "source" attributes to the output whose value is of the form
	// "file:line".
	AddSource bool

	// Ignore records with levels above LevelRef.Level.
	// If nil, accept all levels.
	LevelRef *LevelRef

	// If set, ReplaceAttr is called on each attribute of the message,
	// and the returned value is used instead of the original. If the returned
	// key is empty, the attribute is omitted from the output.
	//
	// The built-in attributes with keys "time", "level", "source", and "msg"
	// are passed to this function first, except that time and level are omitted
	// if zero, and source is omitted if AddSourceLine is false.
	ReplaceAttr func(a Attr) Attr
}

type commonHandler struct {
	appender
	opts              HandlerOptions
	attrs             []Attr
	preformattedAttrs []byte
	mu                sync.Mutex
	w                 io.Writer
}

// Enabled reports whether l is less than or equal to the
// maximum level.
func (h *commonHandler) Enabled(l Level) bool {
	return l <= h.opts.LevelRef.Level()
}

func (h *commonHandler) with(as []Attr) *commonHandler {
	h2 := &commonHandler{
		appender:          h.appender,
		opts:              h.opts,
		attrs:             concat(h.attrs, as),
		preformattedAttrs: h.preformattedAttrs,
		w:                 h.w,
	}
	if h.opts.ReplaceAttr != nil {
		for i, p := range h2.attrs[len(h.attrs):] {
			h2.attrs[i] = h.opts.ReplaceAttr(p)
		}
	}

	// Pre-format the attributes as an optimization.
	h2.setBuffer((*buffer.Buffer)(&h2.preformattedAttrs))
	for _, p := range h2.attrs[len(h.attrs):] {
		h2.appendAttr(p)
	}
	return h2
}

func (h *commonHandler) handle(r Record) error {
	buf := buffer.New()
	defer buf.Free()
	h.setBuffer(buf)

	rep := h.opts.ReplaceAttr

	replace := func(a Attr) {
		a = rep(a)
		h.appendKey(a.Key())
		h.appendAttrValue(a)
	}

	h.appendStart()
	if !r.Time().IsZero() {
		key := "time"
		val := r.Time()
		if rep == nil {
			h.appendKey(key)
			h.appendTime(val)
		} else {
			replace(Time(key, val))
		}
		h.appendSep()
	}
	if r.Level() != 0 {
		key := "level"
		val := r.Level()
		if rep == nil {
			h.appendKey(key)
			h.appendSafeString(val.String())
		} else {
			replace(Any(key, val))
		}
		h.appendSep()
	}
	if h.opts.AddSource {
		file, line := r.SourceLine()
		if file != "" {
			key := "source"
			if rep == nil {
				h.appendKey(key)
				h.appendSource(file, line)
			} else {
				buf := buffer.New()
				buf.WriteString(file)
				buf.WriteByte(':')
				itoa((*[]byte)(buf), line, -1)
				s := string(*buf)
				buf.Free()
				replace(String(key, s))
			}
			h.appendSep()
		}
	}
	key := "msg"
	val := r.Message()
	if rep == nil {
		h.appendKey(key)
		h.appendQuotedEscapedString(val)
	} else {
		replace(String(key, val))
	}
	*buf = append(*buf, h.preformattedAttrs...)
	for i := 0; i < r.NumAttrs(); i++ {
		h.appendAttr(r.Attr(i))
	}
	h.appendEnd()
	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(*buf)
	return err
}

func (h *commonHandler) appendAttr(a Attr) {
	if a.Key() != "" {
		h.appendSep()
		h.appendKey(a.Key())
		if err := h.appendAttrValue(a); err != nil {
			h.appendQuotedEscapedString(fmt.Sprintf("!ERROR:%v", err))
		}
	}
}

type appender interface {
	setBuffer(*buffer.Buffer)
	appendStart()
	appendSep()
	appendEnd()
	appendKey(key string)
	appendSafeString(string)
	appendQuotedEscapedString(string)
	appendTime(time.Time)
	appendSource(file string, line int)
	appendAttrValue(a Attr) error
}

// This takes half the time of Time.AppendFormat.
func appendTimeRFC3339Millis(buf []byte, t time.Time) []byte {
	char := func(b byte) { buf = append(buf, b) }

	year, month, day := t.Date()
	itoa(&buf, year, 4)
	char('-')
	itoa(&buf, int(month), 2)
	char('-')
	itoa(&buf, day, 2)
	char('T')
	hour, min, sec := t.Clock()
	itoa(&buf, hour, 2)
	char(':')
	itoa(&buf, min, 2)
	char(':')
	itoa(&buf, sec, 2)
	ns := t.Nanosecond()
	char('.')
	itoa(&buf, ns/1e6, 3)
	_, offsetSeconds := t.Zone()
	if offsetSeconds == 0 {
		buf = append(buf, 'Z')
	} else {
		offsetMinutes := offsetSeconds / 60
		if offsetMinutes < 0 {
			char('-')
			offsetMinutes = -offsetMinutes
		} else {
			char('+')
		}
		itoa(&buf, offsetMinutes/60, 2)
		char(':')
		itoa(&buf, offsetMinutes%60, 2)
	}
	return buf
}
