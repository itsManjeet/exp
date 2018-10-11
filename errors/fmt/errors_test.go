// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt_test

import (
	"io"
	"os"
	"path"
	"reflect"
	"testing"

	"golang.org/x/exp/errors"
	"golang.org/x/exp/errors/fmt"
)

func TestErrorFormatter(t *testing.T) {
	var (
		simple   = &wrapped{"simple", nil}
		elephant = &wrapped{
			"can't adumbrate elephant",
			detailed{},
		}
		transition = &wrapped2{"elephant still on strike", detailed{}}
		nonascii   = &wrapped{"café", nil}
		newline    = &wrapped{"msg with\nnewline",
			&wrapped{"and another\none", nil}}
		fallback  = &wrapped{"fallback", os.ErrNotExist}
		oldAndNew = &wrapped{"new style", formatError("old style")}
	)
	testCases := []struct {
		err  error
		fmt  string
		want string
	}{{
		err:  simple,
		fmt:  "%s",
		want: "simple",
	}, {
		err:  elephant,
		fmt:  "%s",
		want: "can't adumbrate elephant: out of peanuts",
	}, {
		err:  simple,
		fmt:  "%+v",
		want: "simple\n    somefile.go:123",
	}, {
		err: elephant,
		fmt: "%+v",
		want: `can't adumbrate elephant
    somefile.go:123
--- out of peanuts
    the elephant is on strike
    and the 12 monkeys
    are laughing`,
	}, {
		err: transition,
		fmt: "%+v",
		want: `elephant still on strike
    somefile.go:123
--- out of peanuts
    the elephant is on strike
    and the 12 monkeys
    are laughing`,
	}, {
		err:  simple,
		fmt:  "%#v",
		want: "&fmt_test.wrapped{msg:\"simple\", err:error(nil)}",
	}, {
		err:  fmtTwice("Hello World!"),
		fmt:  "%#v",
		want: "2 times Hello World!",
	}, {
		err:  fallback,
		fmt:  "%s",
		want: "fallback: file does not exist",
	}, {
		err:  fallback,
		fmt:  "%+v",
		want: "fallback\n    somefile.go:123\n--- file does not exist",
	}, {
		err:  oldAndNew,
		fmt:  "%v",
		want: "new style: old style",
	}, {
		err:  oldAndNew,
		fmt:  "%q",
		want: `"new style: old style"`,
	}, {
		err: oldAndNew,
		fmt: "%+v",
		// Note the extra indentation.
		want: "new style\n    somefile.go:123\n--- old style\n    otherfile.go:456",
	}, {
		err:  simple,
		fmt:  "%-12s",
		want: "simple      ",
	}, {
		// Don't use formatting flags for detailed view.
		err:  simple,
		fmt:  "%+12v",
		want: "simple\n    somefile.go:123",
	}, {
		err:  elephant,
		fmt:  "%+50s",
		want: "          can't adumbrate elephant: out of peanuts",
	}, {
		err:  nonascii,
		fmt:  "%q",
		want: `"café"`,
	}, {
		err:  nonascii,
		fmt:  "%+q",
		want: `"caf\u00e9"`,
	}, {
		err:  simple,
		fmt:  "% x",
		want: "73 69 6d 70 6c 65",
	}, {
		err:  newline,
		fmt:  "%s",
		want: "msg with\nnewline: and another\none",
	}, {
		err:  nil,
		fmt:  "%+v",
		want: "<nil>",
	}, {
		err:  (*wrapped)(nil),
		fmt:  "%+v",
		want: "<nil>",
	}, {
		err:  simple,
		fmt:  "%T",
		want: "*fmt_test.wrapped",
	}, {
		err:  simple,
		fmt:  "%🤪",
		want: "%!🤪(*fmt_test.wrapped=&{simple <nil>})",
	}, {
		err:  fmtTwice("%s %s", "ok", panicValue{}),
		fmt:  "%s",
		want: "ok %!s(PANIC=panic)/ok %!s(PANIC=panic)",
	}, {
		err:  fmtTwice("%o %s", panicValue{}, "ok"),
		fmt:  "%s",
		want: "{} ok/{} ok",
	}}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%s", i, tc.fmt), func(t *testing.T) {
			got := fmt.Sprintf(tc.fmt, tc.err)
			if got != tc.want {
				t.Errorf("\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

var _ errors.Formatter = wrapped{}

type wrapped struct {
	msg string
	err error
}

func (e wrapped) Error() string { return "should call Format" }

func (e wrapped) Format(p errors.Printer) (next error) {
	p.Print(e.msg)
	p.Detail()
	p.Print("somefile.go:123")
	return e.err
}

type wrapped2 struct {
	msg string
	err error
}

func (e wrapped2) Error() string { return "should call Format" }

func (e wrapped2) FormatError(p errors.Printer) (next error) {
	p.Print(e.msg)
	p.Detail()
	p.Print("somefile.go:123")
	return e.err
}

var _ errors.Formatter = detailed{}

type detailed struct{}

func (e detailed) Error() string { return fmt.Sprint(e) }

func (detailed) Format(p errors.Printer) (next error) {
	p.Printf("out of %s", "peanuts")
	p.Detail()
	p.Print("the elephant is on strike\n")
	p.Printf("and the %d monkeys\nare laughing", 12)
	return nil
}

// formatError is an error implementing Format instead of errors.Formatter.
// The implementation mimics the implementation of github.com/pkg/errors,
// including that
type formatError string

func (e formatError) Error() string { return string(e) }

func (e formatError) Format(s fmt.State, verb rune) {
	// Body based on pkg/errors/errors.go
	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, string(e))
			fmt.Fprintf(s, "\n%s", "otherfile.go:456")
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, string(e))
	case 'q':
		fmt.Fprintf(s, "%q", string(e))
	}
}

type fmtTwiceErr struct {
	format string
	args   []interface{}
}

func fmtTwice(format string, a ...interface{}) error {
	return fmtTwiceErr{format, a}
}

func (e fmtTwiceErr) Error() string { return fmt.Sprint(e) }

func (e fmtTwiceErr) Format(p errors.Printer) (next error) {
	p.Printf(e.format, e.args...)
	p.Print("/")
	p.Printf(e.format, e.args...)
	return nil
}

func (e fmtTwiceErr) GoString() string {
	return "2 times " + fmt.Sprintf(e.format, e.args...)
}

type panicValue struct{}

func (panicValue) String() string { panic("panic") }

func TestErrorf(t *testing.T) {
	chained := &wrapped{"chained", nil}

	chain := func(s ...string) []string { return s }
	testCases := []struct {
		got  error
		want []string
	}{{
		fmt.Errorf("foo: %s", "simple"),
		chain("foo: simple"),
	}, {
		fmt.Errorf("foo: %v", "simple"),
		chain("foo: simple"),
	}, {
		fmt.Errorf("%s failed: %v", "foo", chained),
		chain("foo failed", "chained/somefile.go:123"),
	}, {
		fmt.Errorf("foo: %s", chained),
		chain("foo", "chained/somefile.go:123"),
	}, {
		fmt.Errorf("foo: %v", chained),
		chain("foo", "chained/somefile.go:123"),
	}, {
		fmt.Errorf("foo: %-12s", chained),
		chain("foo: chained     "), // no dice with special formatting
	}, {
		fmt.Errorf("foo: %+v", chained),
		chain("foo: chained\n    somefile.go:123"), // ditto
	}}
	for _, tc := range testCases {
		t.Run(path.Join(tc.want...), func(t *testing.T) {
			got := errToParts(tc.got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Format:\n got: %#v\nwant: %#v", got, tc.want)
			}

			gotStr := tc.got.Error()
			wantStr := fmt.Sprint(tc.got)
			if gotStr != wantStr {
				t.Errorf("Error:\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

func errToParts(err error) (a []string) {
	for err != nil {
		f, ok := err.(errors.Formatter)
		if !ok {
			a = append(a, err.Error())
			break
		}
		var p testPrinter
		err = f.Format(&p)
		a = append(a, string(p))
	}
	return a

}

type testPrinter string

func (p *testPrinter) Print(a ...interface{}) {
	*p += testPrinter(fmt.Sprint(a...))
}

func (p *testPrinter) Printf(format string, a ...interface{}) {
	*p += testPrinter(fmt.Sprintf(format, a...))
}

func (p *testPrinter) Detail() bool {
	*p += "/"
	return true
}
