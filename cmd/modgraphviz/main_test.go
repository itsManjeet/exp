// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"testing"
)

func TestPrint(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Basic",
			in: `
test.com/A test.com/B
test.com/B test.com/C
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
`,
		},
		{
			name: "Cycles",
			in: `
test.com/A test.com/B
test.com/B test.com/A
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/A"
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBuffer([]byte(tc.in))
			out := &bytes.Buffer{}
			g, err := newGraph(in, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := g.print(out); err != nil {
				t.Fatal(err)
			}

			if out.String() != tc.want {
				t.Fatalf("\ngot: %s\nwant: %s", out.String(), tc.want)
			}
		})
	}
}

func TestPathsTo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		in     string
		want   string
		pathTo string
	}{
		{
			name: "Basic",
			in: `
test.com/A test.com/B
test.com/B test.com/C
`,
			want: `	"test.com/A" -> "test.com/B"
`,
			pathTo: "test.com/B",
		},
		{
			name: "Long",
			in: `
test.com/A test.com/B
test.com/B test.com/C
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
`,
			pathTo: "test.com/C",
		},
		{
			name: "DuplicatePaths",
			in: `
test.com/A test.com/B
test.com/B test.com/C
test.com/C test.com/E
test.com/B test.com/D
test.com/D test.com/E
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
	"test.com/C" -> "test.com/E"
	"test.com/B" -> "test.com/D"
	"test.com/D" -> "test.com/E"
`,
			pathTo: "test.com/E",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBuffer([]byte(tc.in))
			out := bytes.Buffer{}

			g, err := newGraph(in, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := g.printPathsTo(&out, tc.pathTo); err != nil {
				t.Fatal(err)
			}

			if out.String() != tc.want {
				t.Fatalf("\ngot: %s\nwant: %s", out.String(), tc.want)
			}
		})
	}
}

func TestPathsTo_NoPath(t *testing.T) {
	out := bytes.Buffer{}

	g, err := newGraph(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.printPathsTo(&out, "test.com/biscuit"); err == nil {
		t.Fatal("expected but did not receive fatal error: path not found")
	}
}

func TestSimple(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "Basic",
			in: `
test.com/A@v1.0.0-de3113 test.com/B@v1.0.0-d31415
test.com/B@v1.0.0-d31415 test.com/C@v2.0.0=bfdf31e
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
`,
		},
		{
			name: "NoVersion",
			in: `
test.com/A test.com/B@v1.0.0-d31415
test.com/B@v1.0.0-d31415 test.com/C/v2@v2.0.0=bfdf31e
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C/v2"
`,
		},
		{
			name: "NoVersionAnywhere",
			in: `
test.com/A test.com/B
test.com/B test.com/C
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
`,
		},
		{
			name: "DedupesWithinMajor",
			in: `
test.com/A@v1.2.3 test.com/B/v2@v2.3.4
test.com/B/v2@v2.3.4 test.com/C/v3@v3.4.5
test.com/C/v3@v3.4.5 test.com/B/v2@v2.9.7
`,
			want: `	"test.com/A" -> "test.com/B/v2"
	"test.com/B/v2" -> "test.com/C/v3"
	"test.com/C/v3" -> "test.com/B/v2"
`,
		},
		{
			name: "DoesNotDedupeMajors",
			in: `
test.com/A@v1.2.3 test.com/B/v2@v2.3.4
test.com/A@v1.2.3 test.com/B/v3@v3.4.5
`,
			want: `	"test.com/A" -> "test.com/B/v2"
	"test.com/A" -> "test.com/B/v3"
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBuffer([]byte(tc.in))
			out := bytes.Buffer{}

			g, err := newGraph(in, true)
			if err != nil {
				t.Fatal(err)
			}
			if err := g.print(&out); err != nil {
				t.Fatal(err)
			}

			if out.String() != tc.want {
				t.Fatalf("\ngot: %s\nwant: %s", out.String(), tc.want)
			}
		})
	}
}
