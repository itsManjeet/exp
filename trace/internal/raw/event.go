// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package raw

import (
	"strconv"
	"strings"

	"golang.org/x/exp/trace/internal/event"
	"golang.org/x/exp/trace/internal/version"
)

type Event struct {
	Version version.Version
	Ev      event.Type
	Args    []uint64
	Data    []byte
}

func (e *Event) String() string {
	spec := e.Version.Specs()[e.Ev]

	var s strings.Builder
	s.WriteString(spec.Name)
	for i := range spec.Args {
		s.WriteString(" ")
		s.WriteString(spec.Args[i])
		s.WriteString("=")
		s.WriteString(strconv.FormatUint(e.Args[i], 10))
	}
	if spec.IsStack {
		frames := e.Args[len(spec.Args):]
		for i := 0; i < len(frames); i++ {
			if i%4 == 0 {
				s.WriteString("\n\t")
			} else {
				s.WriteString(" ")
			}
			s.WriteString(frameFields[i%4])
			s.WriteString("=")
			s.WriteString(strconv.FormatUint(frames[i], 10))
		}
	}
	if e.Data != nil {
		s.WriteString("\n\tdata=")
		s.WriteString(strconv.Quote(string(e.Data)))
	}
	return s.String()
}
