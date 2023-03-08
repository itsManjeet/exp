// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gobinary provides access to internal details of a Go binary.
package gobinary

import (
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"debug/plan9obj"
	"errors"
)

// Binary represents a Go binary.
//
// The level of detail this type provides depends on which metadata is present
// in the binary. In general, we define 4 levels of metadata:
//
// 0: Binary contains no section headers, no symbol table, no DWARF.
// 1: Binary contains section headers only.
// 2: Level 1, plus symbol table.
// 3: Level 2, plus DWARF.
//
// Binary requires at least level 1 metdata. Without section headers, New*
// return an error. For all existing methods, level 1 metadata is sufficient.
type Binary struct{}

// NewELF returns a Binary representing the passed ELF executable.
//
// Preconditions:
//
// * The binary must contain at least level 1 metadata.
func NewELF(*elf.File) (*Binary, error) {
	return nil, errors.New("unimplementated")
}

// NewMacho returns a Binary representing the passed macho executable.
//
// Preconditions:
//
// * The binary must contain at least level 1 metadata.
func NewMacho(*macho.File) (*Binary, error) {
	return nil, errors.New("unimplementated")
}

// NewPE returns a Binary representing the passed PE executable.
//
// Preconditions:
//
// * The binary must contain at least level 1 metadata.
func NewPE(*pe.File) (*Binary, error) {
	return nil, errors.New("unimplementated")
}

// NewPlan9Obj returns a Binary representing the passed Plan 9 executable.
//
// Preconditions:
//
// * The binary must contain at least level 1 metadata.
func NewPlan9Obj(*plan9obj.File) (*Binary, error) {
	return nil, errors.New("unimplementated")
}

// LookupPhysicalFunc returns the physical function with the passed name.
// LookupPhysicalFunc never returns an inlined instance of a function.
func (*Binary) LookupPhysicalFunc(name string) (*Func, error) {
	return nil, errors.New("unimplementated")
}

// PCToFunc returns the function containing pc.
func (*Binary) PCToFunc(pc uint64) (*Func, error) {
	return nil, errors.New("unimplementated")
}
