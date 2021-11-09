// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage_test

import (
	"regexp"
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
	err     string
	options Options
}

func TestProcess(t *testing.T) {
	for _, test := range []struct {
		name   string
		help   []usage.Page
		values []testValue
	}{{
		// check that no options of any kind works
		// also verify all the arg0 handling
		name: "no options",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"unix_symlink"},
			options: Options{Program: "unix_symlink"},
		}, {
			args:    []string{"c://windows//arg0/program.exe"},
			options: Options{Program: "c://windows//arg0/program.exe"},
		}, {
			args:    []string{"program", "value"},
			options: Options{Program: "program"},
			err:     `^unexpected "value"`,
		}, {
			args:    []string{"program", "-nope"},
			options: Options{},
			err:     `^flag provided but not defined: -nope`,
		}},
	}, {
		// the most basic single boolean flag test
		name: "simple option",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [options]

options:
  -flag  a description
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "-flag"},
			options: Options{Program: "program", Flag: true},
		}},
	}, {
		// the most basic value test
		name: "optional value",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [<file>]
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "afile"},
			options: Options{Program: "program", File: "afile"},
		}},
	}, {
		// check that -- terminates flag processing
		name: "terminator",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [-flag] [<file>]
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "afile"},
			options: Options{Program: "program", File: "afile"},
		}, {
			args:    []string{"program", "-flag"},
			options: Options{Program: "program", Flag: true},
		}, {
			args:    []string{"program", "--"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "--", "-flag"},
			options: Options{Program: "program", File: "-flag"},
		}},
	}, {
		// multiple names for the same boolean flag
		name: "multi option",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [options]

options:
  -flag,-a  a description
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "-flag"},
			options: Options{Program: "program", Flag: true},
		}, {
			args:    []string{"program", "-a"},
			options: Options{Program: "program", Flag: true},
		}},
	}, {
		// a choice between two literal values
		name: "choice",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [flag | bool]
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "flag"},
			options: Options{Program: "program", Flag: true},
		}, {
			args:    []string{"program", "bool"},
			options: Options{Program: "program", Bool: true},
		}},
	}, {
		// a choice between two literal values expressed with full options
		// this also catches the best match behavior
		name: "choice_longest",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program flag
  program
  program bool
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program"},
		}, {
			args:    []string{"program", "flag"},
			options: Options{Program: "program", Flag: true},
		}, {
			args:    []string{"program", "bool"},
			options: Options{Program: "program", Bool: true},
		}},
	}, { // an flag that has a default value
		name: "default value",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [options]

options:
  -file=value  description (default somevalue)
`}},
		values: []testValue{{
			args:    []string{"program"},
			options: Options{Program: "program", File: "somevalue"},
		}, {
			args:    []string{"program", "-file=othervalue"},
			options: Options{Program: "program", File: "othervalue"},
		}},
	}, {
		// test that pages work, and that options merge
		name: "pages",
		help: []usage.Page{{Name: "help", Content: `
usage:
  program [options]

options:
  -a                 a description
  -another,-an=file  another description
`}, {Name: "extra", Content: `
options:
  -a,-alternate,-flag  hidden alternate
  -bool                has no visible form
`}},
		values: []testValue{{
			args:    []string{"program", "-an=normal"},
			options: Options{Program: "program", File: "normal"},
		}, {
			args:    []string{"program", "-bool"},
			options: Options{Program: "program", Bool: true},
		}, {
			args:    []string{"program", "-flag"},
			options: Options{Program: "program", Flag: true},
		}, {
			args:    []string{"program", "-alternate"},
			options: Options{Program: "program", Flag: true},
		}},
	}, {
		// a reasonable representative of a simple app
		name: "basic command",
		help: []usage.Page{{Name: "help", Content: `
a command description

Usage:
  program [-flag | -other] <file>

Options:
  -flag          description of a flag
  -other=string  description of a string value flag
`}},
		// a set of values to test moving flags around the verb
		values: []testValue{{
			args:    []string{"program", "pos1"},
			options: Options{Program: "program", File: "pos1"},
		}, {
			args:    []string{"program", "-other=pos1", "pos2"},
			options: Options{Program: "program", String: "pos1", File: "pos2"},
		}, {
			args:    []string{"program", "-other", "pos2", "pos3"},
			options: Options{Program: "program", String: "pos2", File: "pos3"},
		}, {
			args:    []string{"program", "pos1", "-other", "pos2"},
			options: Options{Program: "program", String: "pos2", File: "pos1"},
		}},
	}, {
		// an example of a standard
		name: "complex command",
		help: []usage.Page{{Name: "help", Content: `
standard help for testing processing flags

Usage:
  program [flags] <string> <file>
  program [flags] [-bool] <file>

flags:
  -flag    A boolean value
  -int=v   Another integer value
`}},
		values: []testValue{{
			args:    []string{"program", "pos1"},
			options: Options{Program: "program", File: "pos1"},
		}, {
			args:    []string{"program", "pos1", "pos2"},
			options: Options{Program: "program", String: "pos1", File: "pos2"},
		}, {
			args:    []string{"program", "-flag", "pos1"},
			options: Options{Program: "program", File: "pos1", Flag: true},
		}, {
			args:    []string{"program", "-bool", "pos1", "pos2"},
			options: Options{Program: "program", String: "pos1", File: "pos2", Bool: true},
			err:     `^flag "bool" present but not allowed`,
		}},
	}, {
		// using all the various types
		name: "all types",
		help: []usage.Page{{Name: "help", Content: `
Usage:
  program [flags] [<stringlist>...]

flags:
  -bool | -int=v | -int64=v | -uint=v | -uint64=v | -float64=v | -string=v | -duration=v
`}},
		values: []testValue{{
			args:    []string{"program", "-bool=true"},
			options: Options{Program: "program", Bool: true},
		}, {
			args:    []string{"program", "-int", "7"},
			options: Options{Program: "program", Int: 7},
		}, {
			args:    []string{"program", "-int64=-2147483649"},
			options: Options{Program: "program", Int64: -2147483649},
		}, {
			args:    []string{"program", "-uint", "95"},
			options: Options{Program: "program", UInt: 95},
		}, {
			args:    []string{"program", "-uint64", "18446744073709551615"},
			options: Options{Program: "program", UInt64: 18446744073709551615},
		}, {
			args:    []string{"program", "-float64", "0.1"},
			options: Options{Program: "program", Float64: 0.1},
		}, {
			args:    []string{"program", "-string", "hi"},
			options: Options{Program: "program", String: "hi"},
		}, {
			args:    []string{"program", "-duration", "5h6m"},
			options: Options{Program: "program", Duration: (5 * time.Hour) + (6 * time.Minute)},
		}, {
			args:    []string{"program", "pos1"},
			options: Options{Program: "program", StringList: []string{"pos1"}},
		}, {
			args:    []string{"program", "pos1", "pos2"},
			options: Options{Program: "program", StringList: []string{"pos1", "pos2"}},
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
					//TODO: if we had an in memory implementation of a file system, we could use usage.Process here
					err := process(test.help, options, v.args)
					switch {
					case v.err == "" && err != nil:
						t.Errorf("Unexpected error: %+v", err)
					case v.err != "" && err == nil:
						t.Errorf("Expected error matching %q", v.err)
					case v.err != "":
						re := regexp.MustCompile(v.err)
						if !re.MatchString(err.Error()) {
							t.Errorf("Wrong error message, got:\n  %v\nexpected to match:\n  %s", err, v.err)
						}
					}
					if diff := cmp.Diff(&v.options, options); diff != "" {
						t.Errorf("Process() mismatch (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func process(help usage.Pages, options interface{}, args []string) error {
	grammar, err := help.Compile()
	if err != nil {
		return err
	}
	fields := &usage.Fields{}
	if err := fields.Scan(options); err != nil {
		return err
	}
	bindings, err := grammar.Bind(fields)
	if err != nil {
		return err
	}
	return bindings.Process(args)
}

func TestCompileErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		help   string
		expect string
	}{
		{
			name:   `empty`,
			expect: `no usage`,
		}, {
			name:   `no pattern`,
			help:   "usage:\n  # no pattern",
			expect: `^help:2:3: expect expression at "# no`,
		}, {
			name:   `invalid pattern`,
			help:   "usage:\n  program# invalid",
			expect: `^help:2:10: expect EOL at "# invalid`,
		}, {
			name:   `unclosed optional`,
			help:   "usage:\n  program [option# not closed",
			expect: `^help:2:18: expect "]" at "# not`,
		}, {
			name:   `invalid optional contents`,
			help:   "usage:\n  program [-] # not valid",
			expect: `^help:2:13: expect flag name at "] # not`,
		}, {
			name:   `value name`,
			help:   "usage:\n  program <> # no value name",
			expect: `^help:2:12: expect name at "> # no`,
		}, {
			name:   `unclosed value`,
			help:   "usage:\n  program <value # not closed",
			expect: `^help:2:17: expect ">" at " # not`,
		}, {
			name:   `unclosed group`,
			help:   "usage:\n  program (group# not closed",
			expect: `^help:2:17: expect "\)" at "# not`,
		}, {
			name:   `invalid group contents`,
			help:   "usage:\n  program (-) # not valid",
			expect: `^help:2:13: expect flag name at "\) # not`,
		}, {
			name:   `invalid sequence`,
			help:   "usage:\n  program arg # not arg",
			expect: `^help:2:15: expect sequence expression at "# not`,
		}, {
			name:   `no flag name`,
			help:   "usage:\n  - # no name",
			expect: `^help:2:4: expect flag name at " # no`,
		}, {
			name:   `bad alias`,
			help:   "usage:\n  -flag,f # no -",
			expect: `^help:2:9: expect "-" at "f # no`,
		}, {
			name:   `missing alias`,
			help:   "usage:\n  -flag,- # no alias",
			expect: `^help:2:10: expect flag name at " # no`,
		}, {
			name:   `missing parameter name`,
			help:   "usage:\n  -flag= # no param",
			expect: `^help:2:10: expect flag parameter at "# no`,
		}, {
			name:   `missing choice`,
			help:   "usage:\n  choice| # no choice",
			expect: `^help:2:11: expect choice expression at "# no`,
		}, {
			name:   `invalid choice`,
			help:   "usage:\n  choice | - # not valid",
			expect: `^help:2:13: expect flag name at " # not`,
		}, {
			name:   `invalid default`,
			help:   "usage:\n  -flag  comment (default \n# not closed",
			expect: `^help:2:27: expect "\)" at "\\n# not`,
		}, {
			name:   `invalid line default`,
			help:   "usage:\n  -flag  comment \n(default \n# not closed",
			expect: `^help:3:10: expect "\)" at "\\n# not`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			help := usage.Pages{{Name: "help", Content: test.help}}
			// now run all the tests with args
			_, err := help.Compile()
			if err == nil {
				t.Fatalf("Expected error")
			}
			expect := regexp.MustCompile(test.expect)
			if !expect.MatchString(err.Error()) {
				t.Errorf("Wrong error message, got:\n  %v\nexpected to match:\n  %s", err, test.expect)
			}
		})
	}
}
