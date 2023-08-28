// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raw

import (
	"fmt"
	"io"
)

type TextWriter struct {
	w io.Writer
}

func NewTextWriter(w io.Writer) (*TextWriter, error) {
	_, err := io.WriteString(w, "Trace Go1.22\n")
	if err != nil {
		return nil, err
	}
	return &TextWriter{w: w}, nil
}

func (w *TextWriter) WriteEvent(e Event) error {
	_, err := fmt.Fprintln(w.w, e.String())
	return err
}
