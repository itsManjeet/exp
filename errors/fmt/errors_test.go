// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt_test

import (
	"os"
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
		nonascii = &wrapped{"café", nil}
		newline  = &wrapped{"msg with\nnewline",
			&wrapped{"and another\none", nil}}
		fallback = &wrapped{"fallback", os.ErrNotExist}
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
		err: simple,
		fmt: "%+v",
		want: `simple
    somefile.go:123`,
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
		err:  fallback,
		fmt:  "%s",
		want: "fallback: file does not exist",
	}, {
		err:  fallback,
		fmt:  "%+v",
		want: "fallback\n    somefile.go:123\n--- file does not exist",
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
		err:  simple,
		fmt:  "%T",
		want: "*fmt_test.wrapped",
	}, {
		err:  simple,
		fmt:  "%🤪",
		want: "%!🤪(*fmt_test.wrapped=&{simple <nil>})",
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
