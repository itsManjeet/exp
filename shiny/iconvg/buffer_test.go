// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iconvg

import (
	"math"
	"testing"
)

func TestDecodeNatural(t *testing.T) {
	testCases := []struct {
		in    buffer
		want  uint32
		wantN int
	}{{
		buffer{},
		0,
		0,
	}, {
		buffer{0x28},
		20,
		1,
	}, {
		buffer{0x59},
		0,
		0,
	}, {
		buffer{0x59, 0x83},
		8406,
		2,
	}, {
		buffer{0x07, 0x00, 0x80},
		0,
		0,
	}, {
		buffer{0x07, 0x00, 0x80, 0x3f},
		266338305,
		4,
	}}

	for _, tc := range testCases {
		got, gotN := tc.in.decodeNatural()
		if got != tc.want || gotN != tc.wantN {
			t.Errorf("in=%x: got %v, %d, want %v, %d", tc.in, got, gotN, tc.want, tc.wantN)
		}
	}
}

func TestDecodeReal(t *testing.T) {
	testCases := []struct {
		in    buffer
		want  float32
		wantN int
	}{{
		buffer{0x28},
		20,
		1,
	}, {
		buffer{0x59, 0x83},
		8406,
		2,
	}, {
		buffer{0x07, 0x00, 0x80, 0x3f},
		1.000000476837158203125,
		4,
	}}

	for _, tc := range testCases {
		got, gotN := tc.in.decodeReal()
		if got != tc.want || gotN != tc.wantN {
			t.Errorf("in=%x: got %v, %d, want %v, %d", tc.in, got, gotN, tc.want, tc.wantN)
		}
	}
}

func TestDecodeCoordinate(t *testing.T) {
	testCases := []struct {
		in    buffer
		want  float32
		wantN int
	}{{
		buffer{0x8e},
		7,
		1,
	}, {
		buffer{0x81, 0x87},
		7.5,
		2,
	}, {
		buffer{0x03, 0x00, 0xf0, 0x40},
		7.5,
		4,
	}}

	for _, tc := range testCases {
		got, gotN := tc.in.decodeCoordinate()
		if got != tc.want || gotN != tc.wantN {
			t.Errorf("in=%x: got %v, %d, want %v, %d", tc.in, got, gotN, tc.want, tc.wantN)
		}
	}
}

func TestDecodeZeroToOne(t *testing.T) {
	trunc := func(x float32) float32 {
		u := math.Float32bits(x)
		u &^= 0x03
		return math.Float32frombits(u)
	}

	testCases := []struct {
		in    buffer
		want  float32
		wantN int
	}{{
		buffer{0x0a},
		1.0 / 24,
		1,
	}, {
		buffer{0x41, 0x1a},
		1.0 / 9,
		2,
	}, {
		buffer{0x63, 0x0b, 0x36, 0x3b},
		trunc(1.0 / 360),
		4,
	}}

	for _, tc := range testCases {
		got, gotN := tc.in.decodeZeroToOne()
		if got != tc.want || gotN != tc.wantN {
			t.Errorf("in=%x: got %v, %d, want %v, %d", tc.in, got, gotN, tc.want, tc.wantN)
		}
	}
}
