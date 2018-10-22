// TODO: test swap corresponding types (e.g. u1 <-> u2 and u2 <-> u1)
// TODO: test exported alias refers to something in another package -- does correspondence work then?
// TODO: CODE COVERAGE
// TODO: note that we may miss correspondences because we bail early when we compare a signature (e.g. when lengths differ; we could do up to the shorter)
// TODO: if you add an unexported method to an exposed interface, you have to check that
//		every exposed type that previously implemented the interface still does. Otherwise
//		an external assignment of the exposed type to the interface type could fail.
// TODO: check constant values: large values aren't representable by some types.
// TODO: Document all the incompatibilities we don't check for.

package apidiff

import (
	"fmt"
	"go/types"
)

// Changes reports on the differences between the APIs of the old and new packages.
// It classifies each difference as either compatible or incompatible (breaking.) For
// a detailed discussion of what constitutes an incompatible change, see the package
// documentation.
func Changes(old, new *types.Package) Report {
	d := newDiffer(old, new)
	d.checkPackage()
	return Report{
		Incompatible: d.incompatibles.collect(),
		Compatible:   d.compatibles.collect(),
	}
}

type differ struct {
	old, new *types.Package
	// Correspondences between named types.
	// Even though it is the named types (*types.Named) that correspond, we use
	// *types.TypeName because they are canonical.
	correspond map[*types.TypeName]*types.TypeName

	// Messages.
	incompatibles messageSet
	compatibles   messageSet
}

func newDiffer(old, new *types.Package) *differ {
	return &differ{
		old:           old,
		new:           new,
		correspond:    map[*types.TypeName]*types.TypeName{},
		incompatibles: messageSet{},
		compatibles:   messageSet{},
	}
}

func (d *differ) incompatible(obj types.Object, part, format string, args ...interface{}) {
	d.addMessage(d.incompatibles, obj, part, format, args)
}

func (d *differ) compatible(obj types.Object, part, format string, args ...interface{}) {
	d.addMessage(d.compatibles, obj, part, format, args)
}

func (d *differ) addMessage(ms messageSet, obj types.Object, part, format string, args []interface{}) {
	ms.add(obj, part, fmt.Sprintf(format, args...))
}

func (d *differ) checkPackage() {
	// Old changes.
	for _, name := range d.old.Scope().Names() {
		oldobj := d.old.Scope().Lookup(name)
		if !oldobj.Exported() {
			continue
		}
		newobj := d.new.Scope().Lookup(name)
		if newobj == nil {
			d.incompatible(oldobj, "", "removed")
			continue
		}
		d.checkObjects(oldobj, newobj)
	}
	// New additions.
	for _, name := range d.new.Scope().Names() {
		newobj := d.new.Scope().Lookup(name)
		if newobj.Exported() && d.old.Scope().Lookup(name) == nil {
			d.compatible(newobj, "", "added")
		}
	}

	// Whole-package satisfaction.
	// For every old exposed type T and interface I, if T implements I, then
	// corresponding(T) must implement corresponding(I).
	for otn1, ntn1 := range d.correspond {
		for otn2, ntn2 := range d.correspond {
			if otn1 == otn2 {
				continue
			}
			//fmt.Printf("# %s - %s\n", otn1, otn2)
			if oiface, ok := otn2.Type().Underlying().(*types.Interface); ok {
				niface := ntn2.Type().Underlying().(*types.Interface)
				if types.Implements(otn1.Type(), oiface) && !types.Implements(ntn1.Type(), niface) {
					d.incompatible(otn1, "", "no longer implements %s", objectString(otn2))
				}
			}
		}
	}
}

func (d *differ) checkObjects(old, new types.Object) {
	switch old := old.(type) {
	case *types.Const:
		if new, ok := new.(*types.Const); ok {
			d.constChanges(old, new)
			return
		}
	case *types.Var:
		if new, ok := new.(*types.Var); ok {
			d.checkEquivalent(old, "", old.Type(), new.Type())
			return
		}
	case *types.Func:
		switch new := new.(type) {
		case *types.Func:
			d.checkEquivalent(old, "", old.Type(), new.Type())
			return
		case *types.Var:
			d.compatible(old, "", "changed from func to var")
			d.checkEquivalent(old, "", old.Type(), new.Type())
			return

		}
	case *types.TypeName:
		if new, ok := new.(*types.TypeName); ok {
			d.checkEquivalent(old, "", old.Type(), new.Type())
			return
		}
	default:
		panic("unexpected obj type")
	}
	// Here if kind of type changed.
	d.incompatible(old, "", "changed from %s to %s",
		objectKindString(old), objectKindString(new))
}

func objectKindString(obj types.Object) string {
	switch obj.(type) {
	case *types.Const:
		return "const"
	case *types.Var:
		return "var"
	case *types.Func:
		return "func"
	case *types.TypeName:
		return "type name"
	default:
		return "???"
	}
}

func (d *differ) checkEquivalent(obj types.Object, part string, old, new types.Type) {
	if !d.equivalent(old, new) {
		d.typeChanged(obj, part, old, new)
	}
}

func (d *differ) typeChanged(obj types.Object, part string, old, new types.Type) {
	old = removeNamesFromSignature(old)
	new = removeNamesFromSignature(new)
	olds := types.TypeString(old, types.RelativeTo(d.old))
	news := types.TypeString(new, types.RelativeTo(d.new))
	d.incompatible(obj, part, "changed from %s to %s", olds, news)
}

func removeNamesFromSignature(t types.Type) types.Type {
	sig, ok := t.(*types.Signature)
	if !ok {
		return t
	}

	dename := func(p *types.Tuple) *types.Tuple {
		var vars []*types.Var
		for i := 0; i < p.Len(); i++ {
			v := p.At(i)
			vars = append(vars, types.NewVar(v.Pos(), v.Pkg(), "", v.Type()))
		}
		return types.NewTuple(vars...)
	}

	return types.NewSignature(sig.Recv(), dename(sig.Params()), dename(sig.Results()), sig.Variadic())
}

func (d *differ) corresponds(old, new *types.Named) bool {
	oldname := old.Obj()
	newname := new.Obj()
	oldc := d.correspond[oldname]
	if oldc == nil {
		// If there is no correspondence, create one, and check for compatibility.
		d.correspond[oldname] = newname
		d.checkCompatibleNamed(oldname, old, new)
		return true
	}
	return oldc == newname
}
