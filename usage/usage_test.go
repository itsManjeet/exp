// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/usage"
)

type Options struct {
	Program string
	File    string
	Flag    bool

	Bool       bool
	Int        int
	Int64      int64
	UInt       uint
	UInt64     uint64
	Float64    float64
	String     string
	StringList []string
	Duration   time.Duration
}

type testValue struct {
	program string
	name    string
	args    []string
	expect  Options
}

func TestFlags(t *testing.T) {
	for _, test := range []struct {
		name   string
		help   usage.Text
		values []testValue
	}{{
		// check that no options of any kind works
		// also verify all the arg0 handling
		name: "no options",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"unix_symlink"},
			expect: Options{Program: "unix_symlink"},
		}, {
			args:   []string{"c://windows//arg0/program.exe"},
			expect: Options{Program: "c://windows//arg0/program.exe"},
		}},
	}, {
		// the most basic single boolean flag test
		name: "simple option",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program [options]

options:
  -flag  a description
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"program", "-flag"},
			expect: Options{Program: "program", Flag: true},
		}},
	}, {
		// the most basic value test
		name: "optional value",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program [<file>]
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"program", "afile"},
			expect: Options{Program: "program", File: "afile"},
		}},
	}, {
		// check that -- terminates flag processing
		name: "terminator",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program [-flag] [<file>]
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"program", "afile"},
			expect: Options{Program: "program", File: "afile"},
		}, {
			args:   []string{"program", "-flag"},
			expect: Options{Program: "program", Flag: true},
		}, {
			args:   []string{"program", "--"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"program", "--", "-flag"},
			expect: Options{Program: "program", File: "-flag"},
		}},
	}, {
		// multiple names for the same boolean flag
		name: "multi option",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program [options]

options:
  -flag,-a  a description
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"program", "-flag"},
			expect: Options{Program: "program", Flag: true},
		}, {
			args:   []string{"program", "-a"},
			expect: Options{Program: "program", Flag: true},
		}},
	}, {
		// a choice between two literal values
		name: "choice",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program [flag | bool]
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program"},
		}, {
			args:   []string{"program", "flag"},
			expect: Options{Program: "program", Flag: true},
		}, {
			args:   []string{"program", "bool"},
			expect: Options{Program: "program", Bool: true},
		}},
	}, { // an flag that has a default value
		name: "default value",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
usage:
  program [options]

options:
  -file=value  description [default: somevalue]
`)}},
		},
		values: []testValue{{
			args:   []string{"program"},
			expect: Options{Program: "program", File: "somevalue"},
		}, {
			args:   []string{"program", "-file=othervalue"},
			expect: Options{Program: "program", File: "othervalue"},
		}},
	}, {
		// test that pages work, and that options merge
		name: "pages",
		help: usage.Text{
			Pages: []usage.Page{{
				Content: []byte(`
usage:
  program [options]

options:
  -a                 a description
  -another,-an=file  another description
`),
			}, {
				Name: `hidden`,
				Content: []byte(`
options:
  -a,-alternate,-flag  hidden alternate
  -bool                has no visible form
`),
			}},
		},
		values: []testValue{{
			args:   []string{"program", "-an=normal"},
			expect: Options{Program: "program", File: "normal"},
		}, {
			args:   []string{"program", "-bool"},
			expect: Options{Program: "program", Bool: true},
		}, {
			args:   []string{"program", "-flag"},
			expect: Options{Program: "program", Flag: true},
		}, {
			args:   []string{"program", "-alternate"},
			expect: Options{Program: "program", Flag: true},
		}},
	}, {
		// a reasonable representative of a simple app
		name: "basic command",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
a command description

Usage:
  program [-flag | -other] <file>

Options:
  -flag          description of a flag
  -other=string  description of a string value flag
`)}},
		},
		// a set of values to test moving flags around the verb
		values: []testValue{{
			args:   []string{"program", "pos1"},
			expect: Options{Program: "program", File: "pos1"},
		}, {
			args:   []string{"program", "-other=pos1", "pos2"},
			expect: Options{Program: "program", String: "pos1", File: "pos2"},
		}, {
			args:   []string{"program", "-other", "pos2", "pos3"},
			expect: Options{Program: "program", String: "pos2", File: "pos3"},
		}, {
			args:   []string{"program", "pos1", "-other", "pos2"},
			expect: Options{Program: "program", String: "pos2", File: "pos1"},
		}},
	}, {
		// an example of a standard
		name: "complex command",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
standard help for testing processing flags

Usage:
  program [flags] <string> <file>
  program [flags] <file>

flags:
  -flag    A boolean value
  -bool    Another boolean value
`)}},
		},
		values: []testValue{{
			args:   []string{"program", "pos1"},
			expect: Options{Program: "program", File: "pos1"},
		}, {
			args:   []string{"program", "pos1", "pos2"},
			expect: Options{Program: "program", String: "pos1", File: "pos2"},
		}},
	}, {
		// using all the various types
		name: "all types",
		help: usage.Text{
			Pages: []usage.Page{{Content: []byte(`
Usage:
  program [flags] [<stringlist>...]

flags:
  -bool | -int | -int64 | -uint | -uint64 | -float64 | -string | -duration
`)}},
		},
		values: []testValue{{
			args:   []string{"program", "-bool=true"},
			expect: Options{Program: "program", Bool: true},
		}, {
			args:   []string{"program", "-int", "7"},
			expect: Options{Program: "program", Int: 7},
		}, {
			args:   []string{"program", "-int64=-2147483649"},
			expect: Options{Program: "program", Int64: -2147483649},
		}, {
			args:   []string{"program", "-uint", "95"},
			expect: Options{Program: "program", UInt: 95},
		}, {
			args:   []string{"program", "-uint64", "18446744073709551615"},
			expect: Options{Program: "program", UInt64: 18446744073709551615},
		}, {
			args:   []string{"program", "-float64", "0.1"},
			expect: Options{Program: "program", Float64: 0.1},
		}, {
			args:   []string{"program", "-string", "hi"},
			expect: Options{Program: "program", String: "hi"},
		}, {
			args:   []string{"program", "-duration", "5h6m"},
			expect: Options{Program: "program", Duration: (5 * time.Hour) + (6 * time.Minute)},
		}, {
			args:   []string{"program", "pos1"},
			expect: Options{Program: "program", StringList: []string{"pos1"}},
		}, {
			args:   []string{"program", "pos1", "pos2"},
			expect: Options{Program: "program", StringList: []string{"pos1", "pos2"}},
		}},
	}} {
		t.Run(test.name, func(t *testing.T) {
			if len(test.values) <= 0 {
				t.Fatalf("No test entries")
			}
			// now run all the tests with args
			for _, v := range test.values {
				t.Run(strings.Join(v.args, "‿"), func(t *testing.T) {
					options := &Options{}
					if err := usage.Process(test.help, options, v.args); err != nil {
						t.Fatalf("Unexpected error: %+v", err)
					}
					if diff := cmp.Diff(&v.expect, options); diff != "" {
						t.Errorf("Process() mismatch (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		help     string
		contains string
		remains  string
	}{
		{
			name:     `empty`,
			contains: `no usage`,
		}, {
			name:     `no pattern`,
			help:     "\nusage:\n  # no pattern",
			contains: `expect Expression`,
			remains:  `# no pattern`,
		}, {
			name:     `invalid pattern`,
			help:     "\nusage:\n  program# invalid",
			contains: `expect '\n'`,
			remains:  `# invalid`,
		}, {
			name:     `unclosed optional`,
			help:     "\nusage:\n  program [option# not closed",
			contains: `expect "]"`,
			remains:  `# not closed`,
		}, {
			name:     `invalid optional contents`,
			help:     "\nusage:\n  program [-] # not valid",
			contains: `expect Name`,
			remains:  `] # not valid`,
		}, {
			name:     `value name`,
			help:     "\nusage:\n  program <> # no value name",
			contains: `expect Name`,
			remains:  `> # no value name`,
		}, {
			name:     `unclosed value`,
			help:     "\nusage:\n  program <value # not closed",
			contains: `expect ">"`,
			remains:  ` # not closed`,
		}, {
			name:     `unclosed group`,
			help:     "\nusage:\n  program (group# not closed",
			contains: `expect ")"`,
			remains:  `# not closed`,
		}, {
			name:     `invalid group contents`,
			help:     "\nusage:\n  program (-) # not valid",
			contains: `expect Name`,
			remains:  `) # not valid`,
		}, {
			name:     `invalid sequence`,
			help:     "\nusage:\n  program arg # not arg",
			contains: `expect Choice`,
			remains:  `# not arg`,
		}, {
			name:     `no flag name`,
			help:     "\nusage:\n  - # no name",
			contains: `expect Name`,
			remains:  ` # no name`,
		}, {
			name:     `bad alias`,
			help:     "\nusage:\n  -flag,f # no -",
			contains: `expect "-"`,
			remains:  `f # no -`,
		}, {
			name:     `missing alias`,
			help:     "\nusage:\n  -flag,- # no alias",
			contains: `expect Name`,
			remains:  ` # no alias`,
		}, {
			name:     `missing parameter name`,
			help:     "\nusage:\n  -flag= # no param",
			contains: `expect Name`,
			remains:  `# no param`,
		}, {
			name:     `missing choice`,
			help:     "\nusage:\n  choice| # no choice",
			contains: `expect Repeat`,
			remains:  `# no choice`,
		}, {
			name:     `invalid choice`,
			help:     "\nusage:\n  choice | - # not valid",
			contains: `expect Name`,
			remains:  ` # not valid`,
		}, {
			name:     `invalid default`,
			help:     "\nusage:\n  -flag  comment [default: \n# not closed",
			contains: `expect "]"`,
			remains:  "\n# not closed",
		}, {
			name:     `invalid line default`,
			help:     "\nusage:\n  -flag  comment \n[default: \n# not closed",
			contains: `expect "]"`,
			remains:  "\n# not closed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// now run all the tests with args
			text := usage.Text{Pages: []usage.Page{{Name: test.name, Content: []byte(test.help)}}}
			err := usage.Process(text, nil, nil)
			if err == nil {
				t.Fatalf("Expected error")
			}
			msg := err.Error()
			if !strings.Contains(msg, test.contains) {
				t.Errorf("Wrong error message, got:\n  %v\nexpected error containing:\n  %s", msg, test.contains)
			}
			if test.remains != "" {
				remains := fmt.Sprintf("got %q", test.remains)
				if !strings.Contains(msg, remains) {
					t.Errorf("Wrong error position, got:\n  %v\nexpected:\n  %s", msg, remains)
				}
			}
		})
	}
}
