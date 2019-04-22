// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"testing"
)

func TestRun(t *testing.T) {
	in := bytes.NewBuffer([]byte(`
test.com/A test.com/B
test.com/B test.com/C
`))
	out := bytes.Buffer{}

	g, err := newGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.print(&out, nil, nil); err != nil {
		t.Fatal(err)
	}

	want := `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
`
	if out.String() != want {
		t.Fatalf("\ngot: %s\nwant: %s", out.String(), want)
	}
}

func TestRun_Cycles(t *testing.T) {
	in := bytes.NewBuffer([]byte(`
test.com/A test.com/B
test.com/B test.com/A
`))
	out := bytes.Buffer{}

	g, err := newGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.print(&out, nil, nil); err != nil {
		t.Fatal(err)
	}

	want := `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/A"
`
	if out.String() != want {
		t.Fatalf("\ngot: %s\nwant: %s", out.String(), want)
	}
}

func TestRun_PathTo(t *testing.T) {
	in := bytes.NewBuffer([]byte(`
test.com/A test.com/B
test.com/A test.com/C
`))
	out := bytes.Buffer{}

	g, err := newGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.printPathTo(&out, nil, nil, nil, "test.com/B"); err != nil {
		t.Fatal(err)
	}

	want := `	"test.com/A" -> "test.com/B"
`
	if out.String() != want {
		t.Fatalf("\ngot: %s\nwant: %s", out.String(), want)
	}
}

func TestRun_PathTo_Long(t *testing.T) {
	in := bytes.NewBuffer([]byte(`
test.com/A test.com/B
test.com/B test.com/C
`))
	out := bytes.Buffer{}

	g, err := newGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.printPathTo(&out, nil, nil, nil, "test.com/C"); err != nil {
		t.Fatal(err)
	}

	want := `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
`
	if out.String() != want {
		t.Fatalf("\ngot: %s\nwant: %s", out.String(), want)
	}
}

func TestRun_PathTo_DupePath(t *testing.T) {
	in := bytes.NewBuffer([]byte(`
test.com/A test.com/B
test.com/B test.com/C
test.com/C test.com/E
test.com/B test.com/D
test.com/D test.com/E
`))
	out := bytes.Buffer{}

	g, err := newGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.printPathTo(&out, nil, nil, nil, "test.com/E"); err != nil {
		t.Fatal(err)
	}

	want := `	"test.com/A" -> "test.com/B"
	"test.com/B" -> "test.com/C"
	"test.com/C" -> "test.com/E"
	"test.com/B" -> "test.com/D"
	"test.com/D" -> "test.com/E"
`
	if out.String() != want {
		t.Fatalf("\ngot: %s\nwant: %s", out.String(), want)
	}
}

func TestRun_PathTo_NoPath(t *testing.T) {
	out := bytes.Buffer{}

	g, err := newGraph(&bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if err := g.printPathTo(&out, nil, nil, nil, "test.com/biscuit"); err == nil {
		t.Fatal("expected but did not receive fatal error: path not found")
	}
}
