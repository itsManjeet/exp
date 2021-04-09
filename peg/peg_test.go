// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//+build go1.16

package peg_test

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/peg"
)

//go:embed grammar.peg
var grammar string

// TestLanguage verifies that the PEG language declaration matches the parser.
// This is necessary because the parser is not declared using the PEG language,
// so to bootstrap it is declared directly using the underlying grammar
// expression nodes.
// It also acts as a fairly extensive test of the parsing itself.
func TestLanguage(t *testing.T) {
	want := strings.TrimSpace(grammar)
	got := fmt.Sprint(peg.LanguageParser())
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("PEG grammar mismatch (-want +got):\n%s", diff)
	}
	// we know our language spec matches now, so next check
	// we can round trip it
	g, err := peg.NewGrammar(`Language`, want)
	if err != nil {
		t.Error(err)
		return
	}
	got = fmt.Sprint(g)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("PEG roundtrip mismatch (-want +got):\n%s", diff)
	}
}

// TestCompileErrors helps check that invalid PEG source produces correct and
// useful error messages.
func TestCompileErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		language string
		contains string
		remains  string
	}{
		{
			name:     `unclosed group`,
			language: "rule <- ( # not closed ",
			contains: `expect ")"`,
			remains:  `# not closed`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// now run all the tests with args
			_, err := peg.NewGrammar(test.name, test.language)
			if err == nil {
				t.Fatalf("Expected error")
			}
			msg := err.Error()
			if !strings.Contains(msg, test.contains) {
				t.Errorf("Wrong error message, got:\n  %v\nexpected error containing:\n  %s", msg, test.contains)
			}
			if test.remains != "" {
				remains := `got "` + test.remains
				if !strings.Contains(msg, remains) {
					t.Errorf("Wrong error position, got:\n  %v\nexpected:\n  %s", msg, remains)
				}
			}
		})
	}
}

func TestSimplifications(t *testing.T) {
	// this test is to verify that the simplifications work by checking that
	// two languages are equivalent when debug printed.
	for _, test := range []struct {
		name     string
		language string
		simple   string
	}{
		{
			name:     `empty sequence`,
			language: `rule <- "a" () "b"`,
			simple:   `rule <- "a" "b"`,
		}, {
			name:     `nil sequence`,
			language: `rule <- "a" (()) "b"`,
			simple:   `rule <- "a" "b"`,
		}, {
			name:     `nested sequence`,
			language: `rule <- "a" ("b" "c") "d"`,
			simple:   `rule <- "a" "b" "c" "d"`,
		}, {
			name:     `nested choice`,
			language: `rule <- "a" / ("b" / "c") / "d"`,
			simple:   `rule <- "a" / "b" / "c" / "d"`,
		}, {
			name:     `nil optional`,
			language: `rule <- "a" ()? "b"`,
			simple:   `rule <- "a" "b"`,
		}, {
			name:     `nested optional`,
			language: `rule <- "a"??`,
			simple:   `rule <- "a"?`,
		}, {
			name:     `empty zero or more`,
			language: `rule <- "a" ()*`,
			simple:   `rule <- "a"`,
		}, {
			name:     `nested zero or more`,
			language: `rule <- "a"****`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `empty one or more`,
			language: `rule <- "a" ()+`,
			simple:   `rule <- "a"`,
		}, {
			name:     `nested one or more`,
			language: `rule <- "a"+++`,
			simple:   `rule <- "a"+`,
		}, {
			name:     `zero or more one or mores`,
			language: `rule <- "a"+*`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `one or more zero or mores`,
			language: `rule <- "a"*+`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `optional zero or more`,
			language: `rule <- "a"*?`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `zero or more optionals`,
			language: `rule <- "a"?*`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `optional one or more`,
			language: `rule <- "a"+?`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `one or more optionals`,
			language: `rule <- "a"?+`,
			simple:   `rule <- "a"*`,
		}, {
			name:     `double negative`,
			language: `rule <- !!"a"`,
			simple:   `rule <- &"a"`,
		}, {
			name:     `double positive`,
			language: `rule <- &&"a"`,
			simple:   `rule <- &"a"`,
		}, {
			name:     `negative positive`,
			language: `rule <- !&"a"`,
			simple:   `rule <- !"a"`,
		}, {
			name:     `positive negative`,
			language: `rule <- &!"a"`,
			simple:   `rule <- !"a"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// now run all the tests with args
			g, err := peg.NewGrammar(test.name, test.language)
			if err != nil {
				t.Fatal(err)
			}
			got := fmt.Sprint(g)
			if got != test.simple {
				t.Errorf("Simplification did not match, got:\n  %v\nexpected:\n  %v", got, test.simple)
			}
		})
	}
}
