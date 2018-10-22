package apidiff

import (
	"bufio"
	"fmt"
	"go/types"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/packages"
)

func TestChanges(t *testing.T) {
	wanti, wantc := splitIntoPackages(t)
	defer os.RemoveAll("testdata/tmp")
	sort.Strings(wanti)
	sort.Strings(wantc)

	oldpkg, err := load("golang.org/x/exp/apidiff/testdata/tmp/old")
	if err != nil {
		t.Fatal(err)
	}
	newpkg, err := load("golang.org/x/exp/apidiff/testdata/tmp/new")
	if err != nil {
		t.Fatal(err)
	}

	report := Changes(oldpkg.Types, newpkg.Types)

	if diff := cmp.Diff(report.Incompatible, wanti); diff != "" {
		t.Errorf("incompatibles (got=-, want=+): %s", diff)
	}
	if diff := cmp.Diff(report.Compatible, wantc); diff != "" {
		t.Errorf("compatibles (got=-, want=+): %s", diff)
	}
}

func splitIntoPackages(t *testing.T) (incompatibles, compatibles []string) {
	// Read the input file line by line.
	// Write a line into the old or new package,
	// dependent on comments.
	// Also collect expected messages.
	f, err := os.Open("testdata/tests.go")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := os.MkdirAll("testdata/tmp/old", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("testdata/tmp/new", 0700); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}

	oldf, err := os.Create("testdata/tmp/old/old.go")
	if err != nil {
		t.Fatal(err)
	}
	newf, err := os.Create("testdata/tmp/new/new.go")
	if err != nil {
		t.Fatal(err)
	}

	wl := func(f *os.File, line string) {
		if _, err := fmt.Fprintln(f, line); err != nil {
			t.Fatal(err)
		}
	}
	writeBoth := func(line string) { wl(oldf, line); wl(newf, line) }
	writeln := writeBoth
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		tl := strings.TrimSpace(line)
		switch {
		case tl == "// old":
			writeln = func(line string) { wl(oldf, line) }
		case tl == "// new":
			writeln = func(line string) { wl(newf, line) }
		case tl == "// both":
			writeln = writeBoth
		case strings.HasPrefix(tl, "// i "):
			incompatibles = append(incompatibles, strings.TrimSpace(tl[4:]))
		case strings.HasPrefix(tl, "// c "):
			compatibles = append(compatibles, strings.TrimSpace(tl[4:]))
		default:
			writeln(line)
		}
	}
	if s.Err() != nil {
		t.Fatal(s.Err())
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

func TestExportedFields(t *testing.T) {
	pkg, err := load("golang.org/x/exp/apidiff/testdata/exported_fields")
	if err != nil {
		t.Fatal(err)
	}
	typeof := func(name string) types.Type {
		return pkg.Types.Scope().Lookup(name).Type()
	}

	s := typeof("S")
	su := s.(*types.Named).Underlying().(*types.Struct)

	ef := exportedSelectableFields(su)
	wants := []struct {
		name string
		typ  types.Type
	}{
		{"A1", typeof("A1")},
		{"D", types.Typ[types.Bool]},
		{"E", types.Typ[types.Int]},
		{"F", typeof("F")},
		{"S", types.NewPointer(s)},
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

func TestIsCompatibleBasic(t *testing.T) {
	// All pairs of compatible basic types:
	comp := map[[2]types.BasicKind]bool{
		{types.Uint8, types.Uint8}:   true,
		{types.Uint8, types.Uint16}:  true,
		{types.Uint8, types.Uint32}:  true,
		{types.Uint8, types.Uint}:    true,
		{types.Uint8, types.Uint64}:  true,
		{types.Uint16, types.Uint16}: true,
		{types.Uint16, types.Uint32}: true,
		{types.Uint16, types.Uint}:   true,
		{types.Uint16, types.Uint64}: true,
		{types.Uint32, types.Uint32}: true,
		{types.Uint32, types.Uint}:   true,
		{types.Uint32, types.Uint64}: true,
		{types.Uint, types.Uint}:     true,
		{types.Uint, types.Uint64}:   true,
		{types.Uint64, types.Uint64}: true,

		{types.Int8, types.Int8}:   true,
		{types.Int8, types.Int16}:  true,
		{types.Int8, types.Int32}:  true,
		{types.Int8, types.Int}:    true,
		{types.Int8, types.Int64}:  true,
		{types.Int16, types.Int16}: true,
		{types.Int16, types.Int32}: true,
		{types.Int16, types.Int}:   true,
		{types.Int16, types.Int64}: true,
		{types.Int32, types.Int32}: true,
		{types.Int32, types.Int}:   true,
		{types.Int32, types.Int64}: true,
		{types.Int, types.Int}:     true,
		{types.Int, types.Int64}:   true,
		{types.Int64, types.Int64}: true,

		{types.Float32, types.Float32}: true,
		{types.Float32, types.Float64}: true,
		{types.Float64, types.Float64}: true,

		{types.Complex64, types.Complex64}:   true,
		{types.Complex64, types.Complex128}:  true,
		{types.Complex128, types.Complex128}: true,
	}
	for i := types.BasicKind(1); i < types.UntypedBool; i++ {
		for j := types.BasicKind(1); j < types.UntypedBool; j++ {
			old := types.Typ[i]
			new := types.Typ[j]
			got := isCompatibleBasic(old, new)
			want := comp[[2]types.BasicKind{i, j}]
			if got != want {
				t.Errorf("old %s, new %s: got %t, want %t", old, new, got, want)
			}
		}
	}
}
