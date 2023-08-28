// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raw

import (
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/exp/trace/internal/event"
	v2 "golang.org/x/exp/trace/internal/v2"
)

type Reader struct {
	r io.ByteReader
}

func NewReader(r io.ByteReader) (*Reader, error) {
	header := []byte("go 1.22 trace\x00\x00\x00")
	for i := range header {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != header[i] {
			return nil, fmt.Errorf("failed to parse header")
		}
	}
	return &Reader{r}, nil
}

func (r *Reader) NextEvent() (Event, error) {
	evb, err := r.r.ReadByte()
	if err == io.EOF {
		return Event{}, io.EOF
	}
	if err != nil {
		return Event{}, err
	}
	if int(evb) >= len(v2.Specs()) || evb == 0 {
		return Event{}, fmt.Errorf("invalid event type: %d", evb)
	}
	ev := event.Type(evb)
	spec := v2.Specs()[ev]
	args, err := r.readArgs(len(spec.Args))
	if err != nil {
		return Event{}, err
	}
	if spec.IsStack {
		len := int(args[1])
		for i := 0; i < len; i++ {
			frame, err := r.readArgs(4)
			if err != nil {
				return Event{}, err
			}
			args = append(args, frame...)
		}
	}
	var data []byte
	if spec.HasData {
		data, err = r.readData()
		if err != nil {
			return Event{}, err
		}
	}
	return Event{
		Ev:   ev,
		Args: args,
		Data: data,
	}, nil
}

func (r *Reader) readArgs(n int) ([]uint64, error) {
	var args []uint64
	for i := 0; i < n; i++ {
		val, err := binary.ReadUvarint(r.r)
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}
	return args, nil
}

func (r *Reader) readData() ([]byte, error) {
	len, err := binary.ReadUvarint(r.r)
	if err != nil {
		return nil, err
	}
	var data []byte
	for i := 0; i < int(len); i++ {
		b, err := r.r.ReadByte()
		if err != nil {
			return nil, err
		}
		data = append(data, b)
	}
	return data, nil
}
