package main

import (
	"bytes"
	"testing"
)

func TestAllSubgraphs(t *testing.T) {
	g1 := "A B\nB C\nC D"

	for _, tc := range []struct {
		name          string
		in            string
		minVertices   int
		wantRoot      *graph2
		wantSubgraphs []*graph2
	}{
		{
			name:          "Basic 1",
			in:            g1,
			minVertices:   1,
			wantRoot:      mustConvert2(bytes.NewBufferString("A")),
			wantSubgraphs: []*graph2{mustConvert2(bytes.NewBufferString("B C\nC D"))},
		},
		{
			name:          "Basic 2",
			in:            g1,
			minVertices:   2,
			wantRoot:      mustConvert2(bytes.NewBufferString("A")),
			wantSubgraphs: []*graph2{mustConvert2(bytes.NewBufferString("B C\nC D"))},
		},
		{
			name:          "Basic 3",
			in:            g1,
			minVertices:   3,
			wantRoot:      mustConvert2(bytes.NewBufferString("A")),
			wantSubgraphs: []*graph2{mustConvert2(bytes.NewBufferString("B C\nC D"))},
		},
		{
			name:          "Basic 4 - Not enough",
			in:            g1,
			minVertices:   4,
			wantRoot:      mustConvert2(bytes.NewBufferString(g1)),
			wantSubgraphs: []*graph2{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, err := convert2(bytes.NewBufferString(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			rootGraph, subgraphs, err := g.allSubgraphs("A", tc.minVertices)
			if err != nil {
				t.Fatal(err)
			}
			if rootGraph == nil {
				t.Fatal("rootGraph is nil - that should never happen")
			}
			if got, want := string(rootGraph.printVertices()), string(tc.wantRoot.printVertices()); got != want {
				t.Fatalf("rootGraph: got:%s\nwant:%s", got, want)
			}
			if got, want := len(subgraphs), len(tc.wantSubgraphs); got != want {
				t.Fatalf("want %d subgraphs, got %d", want, got)
			}
			for i := range tc.wantSubgraphs {
				if got, want := string(subgraphs[i].printVertices()), string(tc.wantSubgraphs[i].printVertices()); got != want {
					t.Fatalf("subgraph %d: got:%s\nwant:%s", i, got, want)
				}
			}
		})
	}
}

func TestIsConnected(t *testing.T) {
	for _, tc := range []struct {
		name string
		root string
		in   string
		want bool
	}{
		{
			name: "Basic Connected",
			root: "A",
			in:   "A B",
			want: true,
		},
		{
			name: "Basic Unconnected",
			root: "A",
			in:   "A B\nC D",
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := bytes.Buffer{}
			in.Write([]byte(tc.in))
			g, err := convert2(&in)
			if err != nil {
				t.Fatal(err)
			}
			if want, got := tc.want, g.isConnected(tc.root); want != got {
				t.Fatalf("connected?: want %v, got %v", want, got)
			}
		})
	}
}
