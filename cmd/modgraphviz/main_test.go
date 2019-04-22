// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"bytes"
	"log"
	"sort"
	"strings"
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
			g, err := newGraph(in)
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
func TestTo(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
		to   string
	}{
		{
			name: "Basic",
			in: `
test.com/A test.com/B
test.com/B test.com/C
`,
			want: `	"test.com/A" -> "test.com/B"
`,
			to: "test.com/B",
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
			to: "test.com/C",
		},
		{
			name: "Cycle_Basic",
			in: `
test.com/A test.com/B
test.com/B test.com/A
`,
			want: `	"test.com/A" -> "test.com/B"
`,
			to: "test.com/B",
		},
		{
			name: "Cycle_WithSplit",
			in: `
test.com/A test.com/B
test.com/B test.com/C
test.com/C test.com/D
test.com/C test.com/B
test.com/B test.com/A
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
	"test.com/C" -> "test.com/D"
`,
			to: "test.com/D",
		},
		{
			name: "TwoPaths_Basic",
			//           /-- C --\
			// A -- B --<         >-- E
			//           \-- D --/
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
			to: "test.com/E",
		},
		{
			name: "TwoPaths_FurtherUp",
			//      /-- B --\
			// A --<         >-- D -- E
			//      \-- C --/
			in: `
test.com/A test.com/B
test.com/A test.com/C
test.com/B test.com/D
test.com/C test.com/D
test.com/D test.com/E
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/D"
	"test.com/D" -> "test.com/E"
	"test.com/A" -> "test.com/C"
	"test.com/C" -> "test.com/D"
`,
			to: "test.com/E",
		},
		{
			// We should include A - C  - D even though it's further up the
			// second path than D (which would already be in the graph by
			// the time we get around to integrating the second path).
			name: "TwoSplits",
			//      /-- B --\          /-- E --\
			// A --<          >-- D --<         >-- G
			//      \-- C --/          \-- F --/
			in: `
test.com/A test.com/B
test.com/A test.com/C
test.com/B test.com/D
test.com/C test.com/D
test.com/D test.com/E
test.com/D test.com/F
test.com/E test.com/G
test.com/F test.com/G
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/D"
	"test.com/D" -> "test.com/E"
	"test.com/E" -> "test.com/G"
	"test.com/D" -> "test.com/F"
	"test.com/F" -> "test.com/G"
	"test.com/A" -> "test.com/C"
	"test.com/C" -> "test.com/D"
`,
			to: "test.com/G",
		},
		{
			// D - E should not be duplicated.
			name: "TwoPaths_TwoSplitsWithGap",
			//      /-- B --\               /-- F --\
			// A --<          >-- D -- E --<         >-- H
			//      \-- C --/               \-- G --/
			in: `
test.com/A test.com/B
test.com/A test.com/C
test.com/B test.com/D
test.com/C test.com/D
test.com/D test.com/E
test.com/E test.com/F
test.com/E test.com/G
test.com/F test.com/H
test.com/G test.com/H
`,
			want: `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/D"
	"test.com/D" -> "test.com/E"
	"test.com/E" -> "test.com/F"
	"test.com/F" -> "test.com/H"
	"test.com/E" -> "test.com/G"
	"test.com/G" -> "test.com/H"
	"test.com/A" -> "test.com/C"
	"test.com/C" -> "test.com/D"
`,
			to: "test.com/H",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.NewBuffer([]byte(tc.in))
			out := bytes.Buffer{}
			g, err := newGraph(in)
			if err != nil {
				t.Fatal(err)
			}
			g, err = g.to(tc.to)
			if err != nil {
				log.Fatal(err)
			}
			if err := g.print(&out); err != nil {
				log.Fatal(err)
			}
			got := sortedByNewline(out.String())
			want := sortedByNewline(tc.want)
			if got != want {
				t.Fatalf("\ngot: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestPathsTo_NoPath(t *testing.T) {
	g, err := newGraph(&bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.to("test.com/biscuit"); err == nil {
		t.Fatal("expected but did not receive fatal error: path not found")
	}
}

func sortedByNewline(s string) string {
	sa := strings.Split(s, "\n")
	sort.Strings(sa)
	return strings.Join(sa, "\n")
}
