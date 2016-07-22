package decimal

import (
	"strconv"
	"strings"
	"testing"
)

func didPanic(f func()) (ok bool) {
	defer func() { ok = recover() != nil }()
	f()
	return ok
}

func newbig(t *testing.T, s string) *Big {
	x, ok := new(Big).SetString(s)
	if !ok {
		if t == nil {
			panic("wanted true got false during set")
		}
		t.Fatal("wanted true got false during set")
	}
	return x
}

// Verify that ErrNaN implements the error interface.
var _ error = ErrNaN{}

func TestBig_Add(t *testing.T) {
	type inp struct {
		a   string
		b   string
		res string
	}

	inputs := [...]inp{
		0: {a: "2", b: "3", res: "5"},
		1: {a: "2454495034", b: "3451204593", res: "5905699627"},
		2: {a: "24544.95034", b: ".3451204593", res: "24545.2954604593"},
		3: {a: ".1", b: ".1", res: "0.2"},
		4: {a: ".1", b: "-.1", res: "0"},
		5: {a: "0", b: "1.001", res: "1.001"},
		6: {a: "123456789123456789.12345", b: "123456789123456789.12345", res: "246913578246913578.2469"},
		7: {a: ".999999999", b: ".00000000000000000000000000000001", res: "0.99999999900000000000000000000001"},
	}

	for i, inp := range inputs {
		a, ok := new(Big).SetString(inp.a)
		if !ok {
			t.FailNow()
		}
		b, ok := new(Big).SetString(inp.b)
		if !ok {
			t.FailNow()
		}
		c := a.Add(a, b)
		if cs := c.String(); cs != inp.res {
			t.Errorf("#%d: wanted %s, got %s", i, inp.res, cs)
		}
	}
}

func TestBig_Cmp(t *testing.T) {
	const (
		lesser  = -1
		equal   = 0
		greater = +1
	)

	samePtr := New(0, 0)
	large, ok := new(Big).SetString(strings.Repeat("9", 500))
	if !ok {
		t.Fatal(ok)
	}
	for i, test := range [...]struct {
		a, b *Big
		v    int
	}{
		// Simple
		{New(1, 0), New(0, 0), greater},
		{New(0, 0), New(1, 0), lesser},
		{New(0, 0), New(0, 0), equal},
		// Fractional
		{New(9876, 3), New(1234, 0), lesser},
		{New(1234, 3), New(50, 25), greater},
		// Same pointers
		{samePtr, samePtr, equal},
		// Large int vs large big.Int
		{New(99999999999, 0), large, lesser},
		{large, New(999999999999999999, 0), greater},
		{New(4, 0), New(4, 0), equal},
		{New(4, 0), new(Big).Quo(New(12, 0), New(3, 0)), equal},
		// z.scale < 0
		{large, new(Big).Set(large), equal},
		// Differing signs
		{new(Big).Set(large).Neg(large), large, lesser},
		{new(Big).Quo(new(Big).Set(large), New(314156, 5)), large, lesser},
	} {
		r := test.a.Cmp(test.b)
		if test.v != r {
			t.Errorf("#%d: wanted %d, got %d", i, test.v, r)
		}
	}
}

func TestBig_String(t *testing.T) {
	x := New(1<<63-1, 0)
	tests := [...]struct {
		a   *Big
		b   string
		sci bool
	}{
		0: {a: New(10, 1), b: "1"},                     // Trim trailing zeros
		1: {a: New(12345, 3), b: "12.345"},             // Normal decimal
		2: {a: New(-9876, 2), b: "-98.76"},             // Negative
		3: {a: New(-1e5, 0), b: strconv.Itoa(-1e5)},    // Large number
		4: {a: New(0, -50), b: "0"},                    // "0"
		5: {a: x.Add(x, x), b: "18446744073709551614"}, // Larger number

		// Scientific notation
		6: {a: New(12412, 3), b: "12.412", sci: true},
		7: {a: New(56, 10), b: "5.6e-9", sci: true},
		8: {a: New(5, 9), b: "5e-9", sci: true},
		9: {a: New(-5, 9), b: "-5e-9", sci: true},
	}
	for i, s := range tests {
		var str string
		if s.sci {
			str = s.a.String()
		} else {
			str = s.a.PlainString()
		}
		if str != s.b {
			t.Fatalf("#%d: wanted %q, got %q", i, s.b, str)
		}
	}
}
