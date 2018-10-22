package apidiff

import (
	"bufio"
	"go/types"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/packages"
)

func TestChanges(t *testing.T) {
	oldpkg, err := load("veneer/apidiff/testdata/old")
	if err != nil {
		t.Fatal(err)
	}
	newpkg, err := load("veneer/apidiff/testdata/new")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("exportedFields", func(t *testing.T) { testExportedFields(t, oldpkg) })

	wanti, wantc := readWantedMessages()
	sort.Strings(wanti)
	sort.Strings(wantc)

	report := Changes(oldpkg.Types, newpkg.Types)

	if diff := cmp.Diff(report.Incompatible, wanti); diff != "" {
		t.Errorf("incompatibles (got=-, want=+): %s", diff)
	}
	if diff := cmp.Diff(report.Compatible, wantc); diff != "" {
		t.Errorf("compatibles (got=-, want=+): %s", diff)
	}
}

func readWantedMessages() (incompatibles, compatibles []string) {
	f, err := os.Open("testdata/new/new.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if i := strings.Index(line, "//i"); i >= 0 {
			incompatibles = append(incompatibles, strings.TrimSpace(line[i+3:]))
		} else if i := strings.Index(line, "//c"); i >= 0 {
			compatibles = append(compatibles, strings.TrimSpace(line[i+3:]))
		}
	}
	if s.Err() != nil {
		panic(s.Err())
	}
	return
}

func load(importPath string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.LoadTypes,
	}
	pkgs, err := packages.Load(cfg, importPath)
	if err != nil {
		return nil, err
	}
	if len(pkgs[0].Errors) > 0 {
		return nil, pkgs[0].Errors[0]
	}
	return pkgs[0], nil
}

func testExportedFields(t *testing.T, pkg *packages.Package) {
	s4 := pkg.Types.Scope().Lookup("S4").Type()
	s4u := s4.(*types.Named).Underlying().(*types.Struct)

	ef := exportedSelectableFields(s4u)
	wants := []struct {
		name string
		typ  types.Type
	}{
		{"A1", pkg.Types.Scope().Lookup("A1").Type()},
		{"D", types.Typ[types.Bool]},
		{"E", types.Typ[types.Int]},
		{"F", pkg.Types.Scope().Lookup("F").Type()},
		{"S4", types.NewPointer(s4)},
	}

	if got, want := len(ef), len(wants); got != want {
		t.Errorf("got %d fields, want %d\n%+v", got, want, ef)
	}
	for _, w := range wants {
		if got := ef[w.name]; got != nil && !types.Identical(got.Type(), w.typ) {
			t.Errorf("%s: got %v, want %v", w.name, got.Type(), w.typ)
		}
	}
}
