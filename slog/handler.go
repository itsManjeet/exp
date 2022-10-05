// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog/internal/buffer"
)

// A Handler handles log records produced by a Logger..
//
// A typical handler may print log records to standard error,
// or write them to a file or database, or perhaps augment them
// with additional attributes and pass them on to another handler.
//
// Any of the Handler's methods may be called concurrently with itself
// or with other methods. It is the responsibility of the Handler to
// manage this concurrency.
type Handler interface {
	// Enabled reports whether the handler handles records at the given level.
	// The handler ignores records whose level is lower.
	Enabled(Level) bool

	// Handle handles the Record.
	// Handle methods that produce output should observe the following rules:
	//   - If r.Time() is the zero time, ignore the time.
	//   - If an Attr's key is the empty string, ignore the Attr.
	Handle(r Record) error

	// With returns a new Handler whose attributes consist of
	// the receiver's attributes concatenated with the arguments.
	// The Handler owns the slice: it may retain, modify or discard it.
	With(attrs []Attr) Handler

	// WithScope returns a new Handler with the given scope appended to
	// the receiver's existing scopes.
	// The keys of all subsequent attributes, whether added by With or in a
	// Record, should be qualified by the sequence of scope names.
	//
	// How this qualification happens is up to the Handler, so long as
	// this Handler's attribute keys differ from those of another Handler
	// with a different sequence of scope names.
	//
	// A Handler should treat a scope as starting a Group of Attrs. That is,
	//
	//     logger.WithScope("s").LogAttrs(slog.Int("a", 1), slog.Int("b", 2))
	//
	// should behave like
	//
	//     logger.LogAttrs(slog.Group("s", slog.Int("a", 1), slog.Int("b", 2)))
	WithScope(name string) Handler
}

type defaultHandler struct {
	attrs  []Attr
	output func(int, string) error // log.Output, except for testing
}

func (*defaultHandler) Enabled(l Level) bool {
	return l >= InfoLevel
}

// Collect the level, attributes and message in a string and
// write it with the default log.Logger.
// Let the log.Logger handle time and file/line.
func (h *defaultHandler) Handle(r Record) error {
	var b strings.Builder
	b.WriteString(r.Level().String())
	b.WriteByte(' ')
	for _, a := range h.attrs {
		h.writeAttr(&b, a)
	}
	r.Attrs(func(a Attr) {
		h.writeAttr(&b, a)
	})
	b.WriteString(r.Message())
	return h.output(4, b.String())
}

func (h *defaultHandler) writeAttr(b *strings.Builder, a Attr) {
	b.WriteString(a.Key)
	b.WriteByte('=')
	b.WriteString(a.Value.Resolve().String())
	b.WriteByte(' ')
}

func (d *defaultHandler) With(as []Attr) Handler {
	d2 := *d
	d2.attrs = concat(d2.attrs, as)
	return &d2
}

func (h *defaultHandler) WithScope(name string) Handler {
	panic("unimplemented")
}

// HandlerOptions are options for a TextHandler or JSONHandler.
// A zero HandlerOptions consists entirely of default values.
type HandlerOptions struct {
	// Add a "source" attribute to the output whose value is of the form
	// "file:line".
	AddSource bool

	// Ignore records with levels below Level.Level().
	// The default is InfoLevel.
	Level Leveler

	// If set, ReplaceAttr is called on each attribute of the message,
	// and the returned value is used instead of the original. If the returned
	// key is empty, the attribute is omitted from the output.
	//
	// The built-in attributes with keys "time", "level", "source", and "msg"
	// are passed to this function first, except that time and level are omitted
	// if zero, and source is omitted if AddSourceLine is false.
	ReplaceAttr func(a Attr) Attr
}

// Keys for "built-in" attributes.
const (
	timeKey    = "time"
	levelKey   = "level"
	messageKey = "msg"
	sourceKey  = "source"
)

type commonHandler struct {
	json              bool // true => output JSON; false => output text
	opts              HandlerOptions
	preformattedAttrs []byte
	scopePrefix       string   // for text: prefix of scopes opened in preformatting
	scopes            []string // all scopes
	nOpenScopes       int      // the number of scopes opened in in preformattedAttrs
	mu                sync.Mutex
	w                 io.Writer
}

func (h *commonHandler) clone() *commonHandler {
	// We can't use assignment because we can't copy the mutex.
	return &commonHandler{
		json:              h.json,
		opts:              h.opts,
		preformattedAttrs: h.preformattedAttrs,
		scopePrefix:       h.scopePrefix,
		scopes:            slices.Clip(h.scopes),
		nOpenScopes:       h.nOpenScopes,
		w:                 h.w,
	}
}

// Enabled reports whether l is greater than or equal to the
// minimum level.
func (h *commonHandler) Enabled(l Level) bool {
	minLevel := InfoLevel
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return l >= minLevel
}

func (h *commonHandler) withAttrs(as []Attr) *commonHandler {
	h2 := h.clone()
	// Pre-format the attributes as an optimization.
	prefix := buffer.New()
	defer prefix.Free()
	prefix.WriteString(h.scopePrefix)
	state := handleState{
		h:      h2,
		buf:    (*buffer.Buffer)(&h2.preformattedAttrs),
		sep:    "",
		prefix: prefix,
	}
	if len(h2.preformattedAttrs) > 0 {
		state.sep = h.attrSep()
	}
	state.openScopes()
	for _, a := range as {
		state.appendAttr(a)
	}
	// Remember the new prefix for later keys.
	h2.scopePrefix = state.prefix.String()
	// Remember how many opened scopes are in preformattedAttrs,
	// so we don't open them again when we handle a Record.
	h2.nOpenScopes = len(h2.scopes)
	return h2
}

func (h *commonHandler) withScope(name string) *commonHandler {
	h2 := h.clone()
	h2.scopes = append(h2.scopes, name)
	return h2
}

func (h *commonHandler) handle(r Record) error {
	rep := h.opts.ReplaceAttr
	state := handleState{h: h, buf: buffer.New(), sep: ""}
	defer state.buf.Free()
	if h.json {
		state.buf.WriteByte('{')
	}
	// Built-in attributes. They are not scoped.
	// time
	if !r.Time().IsZero() {
		key := timeKey
		val := r.Time().Round(0) // strip monotonic to match Attr behavior
		if rep == nil {
			state.appendKey(key)
			state.appendTime(val)
		} else {
			state.appendAttr(Time(key, val))
		}
	}
	// level
	key := levelKey
	val := r.Level()
	if rep == nil {
		state.appendKey(key)
		state.appendString(val.String())
	} else {
		state.appendAttr(Any(key, val))
	}
	// source
	if h.opts.AddSource {
		file, line := r.SourceLine()
		if file != "" {
			key := sourceKey
			if rep == nil {
				state.appendKey(key)
				state.appendSource(file, line)
			} else {
				buf := buffer.New()
				buf.WriteString(file) // TODO: escape?
				buf.WriteByte(':')
				itoa((*[]byte)(buf), line, -1)
				s := buf.String()
				buf.Free()
				state.appendAttr(String(key, s))
			}
		}
	}
	key = messageKey
	msg := r.Message()
	if rep == nil {
		state.appendKey(key)
		state.appendString(msg)
	} else {
		state.appendAttr(String(key, msg))
	}
	// preformatted Attrs
	if len(h.preformattedAttrs) > 0 {
		state.buf.WriteString(state.sep)
		state.buf.Write(h.preformattedAttrs)
		state.sep = h.attrSep()
	}
	// Attrs in Record -- unlike the built-in ones, they are scoped.
	state.prefix = buffer.New()
	defer state.prefix.Free()
	state.prefix.WriteString(h.scopePrefix)
	state.openScopes()
	r.Attrs(func(a Attr) {
		state.appendAttr(a)
	})
	if h.json {
		// Close all open scopes.
		for range h.scopes {
			state.buf.WriteByte('}')
		}
		// Close the top-level object.
		state.buf.WriteByte('}')
	}
	state.buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(*state.buf)
	return err
}

// attrSep returns the separator between attributes.
func (h *commonHandler) attrSep() string {
	if h.json {
		return ","
	}
	return " "
}

// handleState holds state for a single call to commonHandler.handle.
// The initial value of sep determines whether to emit a separator
// before the next key, after which it stays true.
type handleState struct {
	h      *commonHandler
	buf    *buffer.Buffer
	sep    string         // write between Attrs
	prefix *buffer.Buffer // for text: key prefix
}

func (s *handleState) openScopes() {
	for _, n := range s.h.scopes[s.h.nOpenScopes:] {
		s.openGroup(n)
	}
}

// openGroup starts a new group of attributes
// with the given name.
// A group can arise from a scope, or from an Attr with a GroupKind value.
func (s *handleState) openGroup(name string) {
	if s.h.json {
		s.appendKey(name)
		s.buf.WriteByte('{')
		s.sep = ""
	} else {
		s.prefix.WriteString(escapeDots(name))
		s.prefix.WriteByte('.')
	}
}

// closeGroup ends the group with the given name.
func (s *handleState) closeGroup(name string) {
	if s.h.json {
		s.buf.WriteByte('}')
	} else {
		(*s.prefix) = (*s.prefix)[:len(*s.prefix)-len(name)-1]
	}
}

// appendAttr appends the Attr's key and value using app.
// If sep is true, it also prepends a separator.
// It handles replacement and checking for an empty key.
// It sets sep to true if it actually did the append (if the key was non-empty
// after replacement).
func (s *handleState) appendAttr(a Attr) {
	if rep := s.h.opts.ReplaceAttr; rep != nil {
		a = rep(a)
	}
	if a.Key == "" {
		return
	}
	v := a.Value.Resolve()
	if v.Kind() == GroupKind {
		s.openGroup(a.Key)
		for _, aa := range v.Group() {
			s.appendAttr(aa)
		}
		s.closeGroup(a.Key)
	} else {
		s.appendKey(a.Key)
		s.appendValue(v)
	}
}

func (s *handleState) appendError(err error) {
	s.appendString(fmt.Sprintf("!ERROR:%v", err))
}

func (s *handleState) appendKey(key string) {
	s.buf.WriteString(s.sep)
	if s.prefix != nil {
		s.buf.Write(*s.prefix)
	}
	s.appendString(key)
	if s.h.json {
		s.buf.WriteByte(':')
	} else {
		s.buf.WriteByte('=')
	}
	s.sep = s.h.attrSep()
}

func (s *handleState) appendSource(file string, line int) {
	if s.h.json {
		s.buf.WriteByte('"')
		*s.buf = appendEscapedJSONString(*s.buf, file)
		s.buf.WriteByte(':')
		itoa((*[]byte)(s.buf), line, -1)
		s.buf.WriteByte('"')
	} else {
		// text
		if needsQuoting(file) {
			s.appendString(file + ":" + strconv.Itoa(line))
		} else {
			// common case: no quoting needed.
			s.appendString(file)
			s.buf.WriteByte(':')
			itoa((*[]byte)(s.buf), line, -1)
		}
	}
}

func (s *handleState) appendString(str string) {
	if s.h.json {
		s.buf.WriteByte('"')
		*s.buf = appendEscapedJSONString(*s.buf, str)
		s.buf.WriteByte('"')
	} else {
		// text
		if needsQuoting(str) {
			*s.buf = strconv.AppendQuote(*s.buf, str)
		} else {
			s.buf.WriteString(str)
		}
	}
}

func (s *handleState) appendValue(v Value) {
	var err error
	if s.h.json {
		err = appendJSONValue(s, v)
	} else {
		err = appendTextValue(s, v)
	}
	if err != nil {
		s.appendError(err)
	}
}

func (s *handleState) appendTime(t time.Time) {
	if s.h.json {
		appendJSONTime(s, t)
	} else {
		*s.buf = appendTimeRFC3339Millis(*s.buf, t)
	}
}

// This takes half the time of Time.AppendFormat.
func appendTimeRFC3339Millis(buf []byte, t time.Time) []byte {
	// TODO: try to speed up by indexing the buffer.
	char := func(b byte) {
		buf = append(buf, b)
	}

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
		char('Z')
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
