package internal_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/exp/cmd/modgraphviz/internal"
)

func TestReq(t *testing.T) {
	a1 := internal.Version{Path: "A", Version: "1.0.0"}
	b1 := internal.Version{Path: "B", Version: "1.2.3"}
	b2 := internal.Version{Path: "B", Version: "1.2.4"}
	b3 := internal.Version{Path: "B", Version: "2.0.0"}
	b4 := internal.Version{Path: "B", Version: "0.1.8"}
	c1 := internal.Version{Path: "C", Version: "2.7.9"}
	list := []internal.Version{b1, b2, b3, b4, c1}
	reqs := make(internal.ReqsMap)
	reqs[a1] = []internal.Version{b1}
	reqs[b1] = []internal.Version{c1}
	reqs[c1] = []internal.Version{b2, b3, b4}
	reqs[b2] = []internal.Version{}
	reqs[b3] = []internal.Version{}
	reqs[b4] = []internal.Version{}

	got, err := internal.Req(a1, list, []string{"B", "C"}, reqs)
	if err != nil {
		t.Fatal(err)
	}

	want := []internal.Version{b2, b3, c1}
	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("got %v, want %v", got, want)
	}
}
