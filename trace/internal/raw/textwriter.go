// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raw

import (
	"fmt"
	"io"

	"golang.org/x/exp/trace/internal/version"
)

type TextWriter struct {
	w io.Writer
	v version.Version
}

func NewTextWriter(w io.Writer, v version.Version) (*TextWriter, error) {
	_, err := fmt.Fprintf(w, "Trace Go1.%d\n", v)
	if err != nil {
		return nil, err
	}
	return &TextWriter{w: w, v: v}, nil
}

func (w *TextWriter) WriteEvent(e Event) error {
	// Check version.
	if e.Version != w.v {
		return fmt.Errorf("mismatched version between writer (go 1.%d) and event (go 1.%d)", w.v, e.Version)
	}

	// Write event.
	_, err := fmt.Fprintln(w.w, e.String())
	return err
}
