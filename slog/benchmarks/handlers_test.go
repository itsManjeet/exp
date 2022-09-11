package benchmarks

import (
	"bytes"
	"testing"

	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

func TestHandlers(t *testing.T) {
	r := slog.NewRecord(TestTime, slog.InfoLevel, TestMessage, 0)
	r.AddAttrs(TestAttrs...)
	t.Run("text", func(t *testing.T) {
		var b bytes.Buffer
		h := newFastTextHandler(&b)
		if err := h.Handle(r); err != nil {
			t.Fatal(err)
		}
		got := b.String()
		if got != WantText {
			t.Errorf("\ngot  %q\nwant %q", got, WantText)
		}
	})
	t.Run("async", func(t *testing.T) {
		h := newAsyncHandler()
		if err := h.Handle(r); err != nil {
			t.Fatal(err)
		}
		got := h.ringBuffer[0]
		if !got.Time().Equal(r.Time()) || !slices.EqualFunc(attrSlice(got), attrSlice(r), slog.Attr.Equal) {
			t.Errorf("got %+v, want %+v", got, r)
		}
	})
}

func attrSlice(r slog.Record) []slog.Attr {
	var as []slog.Attr
	r.Attrs(func(a slog.Attr) { as = append(as, a) })
	return as
}
