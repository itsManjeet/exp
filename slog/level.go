// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"math"
	"sync/atomic"
)

// A Level is the importance or severity of a log event.
// The higher the level, the less important or severe the event.
type Level int

// The level numbers below don't really matter too much. Any system can map them
// to another numbering scheme if it wishes. We picked them to satisfy two
// constraints.
//
// First, we wanted to make it easy to work with verbosities instead of levels.
// Since higher verbosities are less important, higher levels are as well.
//
// Second, we wanted some room between levels to accommodate schemes with named
// levels between ours. For example, Google Cloud Logging defines a Notice level
// between Info and Warn. Since there are only a few of these intermediate
// levels, the gap between the numbers need not be large. We selected a gap of
// 10, because the majority of humans have 10 fingers.
//
// The missing gap between Info and Debug has to do with verbosities again. It
// is natural to think of verbosity 0 as Info, and then verbosity 1 is the
// lowest level one would call Debug. The simple formula
//   level = InfoLevel + verbosity
// then works well to map verbosities to levels. That is,
//
//   Level(InfoLevel+0).String() == "INFO"
//   Level(InfoLevel+1).String() == "DEBUG"
//   Level(InfoLevel+2).String() == "DEBUG+1"
//
// and so on.

// Names for common levels.
const (
	ErrorLevel Level = 10
	WarnLevel  Level = 20
	InfoLevel  Level = 30
	DebugLevel Level = 31
)

// String returns a name for the level.
// If the level has a name, then that name
// in uppercase is returned.
// If the level is between named values, then
// an integer is appended to the uppercased name.
// Examples:
//
//	WarnLevel.String() => "WARN"
//	(WarnLevel-2).String() => "WARN-2"
func (l Level) String() string {
	str := func(base string, val Level) string {
		if val == 0 {
			return base
		}
		return fmt.Sprintf("%s%+d", base, val)
	}

	switch {
	case l <= 0:
		return fmt.Sprintf("!BADLEVEL(%d)", l)
	case l <= ErrorLevel:
		return str("ERROR", l-ErrorLevel)
	case l <= WarnLevel:
		return str("WARN", l-WarnLevel)
	case l <= InfoLevel:
		return str("INFO", l-InfoLevel)
	default:
		return str("DEBUG", l-DebugLevel)
	}
}

// Level returns the receiver.
// It implements Leveler.
func (l Level) Level() Level { return l }

// An AtomicLevel is a Level that can be read and written safely by multiple
// goroutines.
// Use NewAtomicLevel to create one.
type AtomicLevel struct {
	val atomic.Int64
}

// NewAtomicLevel creates an AtomicLevel initialized to the given Level.
func NewAtomicLevel(l Level) *AtomicLevel {
	var a AtomicLevel
	a.Set(l)
	return &a
}

// Level returns r's level.
// If the receiver is nil, it returns the maximum level.
func (a *AtomicLevel) Level() Level {
	if a == nil {
		return Level(math.MaxInt)
	}
	return Level(int(a.val.Load()))
}

// Set sets the receiver's level to l.
func (a *AtomicLevel) Set(l Level) {
	a.val.Store(int64(l))
}

func (a *AtomicLevel) String() string {
	if a == nil {
		return "AtomicLevel(nil)"
	}
	return fmt.Sprintf("AtomicLevel(%s)", a.Level())
}

// A Leveler reports a Level.
//
// Both Level and *AtomicLevel implement Leveler, so they can be used
// interchangeably when a Leveler is required.
type Leveler interface {
	Level() Level
}
