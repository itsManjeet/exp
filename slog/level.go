// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

import (
	"fmt"
	"math"
	"strconv"
)

// A Level is the importance or severity of a log event.
// The higher the level, the less important or severe the event.
type Level int

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
		if val > 0 {
			base += "+"
		}
		return base + strconv.Itoa(int(val))
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

// A LevelRef is a reference to a level.
// LevelRefs are safe for use by multiple goroutines.
// Use NewLevelRef to create a LevelRef.
//
// If all the Handlers of a program use the same LevelRef,
// then a single Set on that LevelRef will change the level
// for all of them.
type LevelRef struct {
	val *atomicValue[Level]
}

// NewLevelRef creates a LevelRef initialized to the given Level.
func NewLevelRef(l Level) *LevelRef {
	r := &LevelRef{val: &atomicValue[Level]{}}
	r.val.set(l)
	return r
}

// Level returns the LevelRef's level.
// If LevelRef is nil, it returns the maximum level.
func (r *LevelRef) Level() Level {
	if r == nil {
		return Level(math.MaxInt)
	}
	return r.val.get()
}

// Set sets the LevelRef's level to l.
func (r *LevelRef) Set(l Level) {
	r.val.set(l)
}
