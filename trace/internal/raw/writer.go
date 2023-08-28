// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raw

import (
	"encoding/binary"
	"io"

	v2 "golang.org/x/exp/trace/internal/v2"
)

type Writer struct {
	w   io.Writer
	buf []byte
}

func NewWriter(w io.Writer) (*Writer, error) {
	_, err := w.Write([]byte("go 1.22 trace\x00\x00\x00"))
	return &Writer{w: w}, err
}

func (w *Writer) WriteEvent(e Event) error {
	// Write event header byte.
	w.buf = append(w.buf, uint8(e.Ev))

	// Write out all arguments.
	spec := v2.Specs()[e.Ev]
	for _, arg := range e.Args[:len(spec.Args)] {
		w.buf = binary.AppendUvarint(w.buf, arg)
	}
	if spec.IsStack {
		frameArgs := e.Args[len(spec.Args):]
		for i := 0; i < len(frameArgs); i++ {
			w.buf = binary.AppendUvarint(w.buf, frameArgs[i])
		}
	}

	// Write out the length of the data.
	if spec.HasData {
		w.buf = binary.AppendUvarint(w.buf, uint64(len(e.Data)))
	}

	// Write out varint events.
	_, err := w.w.Write(w.buf)
	w.buf = w.buf[:0]
	if err != nil {
		return err
	}

	// Write out data.
	if spec.HasData {
		_, err := w.w.Write(e.Data)
		return err
	}
	return nil
}
