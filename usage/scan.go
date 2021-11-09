// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// input tracks the parsers progress through the data being processed.
type input struct {
	filename string
	data     string
	offset   int
}

// errNotMatched is produced by any node that does not match the input.
var errNotMatched = errors.New("not matched")

// parseError indicates an error in parsing and the position at which it
// happened.
type parseError struct {
	In      input
	Message string
	Fatal   bool
}

// newInput returns an Input ready to parse for the supplied data.
// The filename is used when reporting errors.
func newInput(filename string, data string) *input {
	return &input{
		filename: filename,
		data:     data,
	}
}

// Error makes ParseError conform to the error interface.
// It prints the error in the standard "file:line:column: message" format.
// It may be expensive as it works out the line and column from the offest.
func (err *parseError) Error() string {
	next := err.In.data[err.In.offset:]
	if len(next) > 20 {
		next = next[:20]
	}
	line, lastEOL := 1, 0
	for i, r := range err.In.data[:err.In.offset] {
		if r == '\n' {
			lastEOL = i
			line++
		}
	}
	column := err.In.offset - lastEOL
	return fmt.Sprintf("%s:%d:%d: %s at %q",
		err.In.filename, line, column, err.Message, next)
}

// eof returns true if the input has reached the end of the data.
func (in *input) eof() bool {
	return in.offset >= len(in.data)
}

// scan invokes the supplied scanner, and returns any parse error that occurs.
// This is normally used to invoke the top level scanner for an entire file.
func scan(in *input, scanner func(*input)) (err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case *parseError:
			err = r
		case error:
			if r != errNotMatched {
				panic(err)
			}
			err = r
		default:
			panic(err)
		}
	}()
	scanner(in)
	return nil
}

// scanCapture invokes the supplied scanner, and then returns the input it consumed
// as a string.
func scanCapture(in *input, scanner func(*input)) string {
	start := in.offset
	scanner(in)
	return in.data[start:in.offset]
}

// scanOptional tries to match the scanner against the input.
// If the scanner does not match, then the input is restored to its original
// state and the match failure is suppressed.
// It returns true if the input is advanced by the scanner. This means that
// a scanner that matches without consuming will still return false.
func scanOptional(in *input, scanner func(*input)) bool {
	mark := *in
	defer func() {
		err := recover()
		switch err {
		case nil:
		case errNotMatched:
			*in = mark
		default:
			panic(err)
		}
	}()
	scanner(in)
	return mark.offset != in.offset
}

// scanPeek looks ahead to see if the scanner would consume some input.
// It always returns the input to its original state when it is done.
func scanPeek(in *input, scanner func(*input)) bool {
	mark := *in
	result := scanOptional(in, scanner)
	*in = mark
	return result
}

// scanMust invokes the scanner and if it does not match then it panics with a
// ParseError at the original location of the input with the suppied message.
func scanMust(in *input, message string, scanner func(*input)) {
	mark := *in
	defer func() {
		err := recover()
		switch err {
		case nil:
		case errNotMatched:
			panic(&parseError{
				In:      mark,
				Message: message,
				Fatal:   true,
			})
		default:
			panic(err)
		}
	}()
	scanner(in)
}

// scanRune consumes the next rune from the input and checks that it matches the
// supplied rune.
func scanRune(in *input, r rune) {
	if in.eof() {
		panic(errNotMatched)
	}
	next, size := utf8.DecodeRuneInString(in.data[in.offset:])
	in.offset += size
	if rune(r) != next {
		panic(errNotMatched)
	}
}

// scanClass consumes the next rune from the input and checks that it matches the
// supplied predicate.
func scanClass(in *input, o func(rune) bool) rune {
	if in.eof() {
		panic(errNotMatched)
	}
	next, size := utf8.DecodeRuneInString(in.data[in.offset:])
	in.offset += size
	if !o(next) {
		panic(errNotMatched)
	}
	return next
}

// scanString checks that the head of the input matches the supplied string and
// consumes it.
func scanString(in *input, s string) {
	remains := in.data[in.offset:]
	if !strings.HasPrefix(remains, s) {
		panic(errNotMatched)
	}
	in.offset += len(s)
}

// scanSkip over all the runes at the head of the input that the matcher returns
// true for. This will never fail, but it may not match anything and thus not
// consume anything.
func scanSkip(in *input, m func(rune) bool) {
	for !in.eof() {
		r, size := utf8.DecodeRuneInString(in.data[in.offset:])
		if !m(r) {
			break
		}
		in.offset += size
	}
}
