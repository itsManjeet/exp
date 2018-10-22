package apidiff

import (
	"go/types"
	"sort"
)

// Two types are equivalent if they are identical except for type names,
// which must correspond.
//
// This is not a pure function. If we come across named types while traversing,
// we establish correspondence.
func (d *differ) equivalent(old, new types.Type) bool {
	return d.equiv(old, new, nil)
}

// Call recursively as much as possible, to establish more correspondences and so
// check more of the API. E.g. if the new function has more parameters than the old,
// compare all the old ones before returning false.
func (d *differ) equiv(old, new types.Type, p *ifacePair) bool {
	// Structure copied from types.Identical.
	switch old := old.(type) {
	case *types.Basic:
		return types.Identical(old, new)

	case *types.Array:
		if new, ok := new.(*types.Array); ok {
			return d.equiv(old.Elem(), new.Elem(), p) && old.Len() == new.Len()
		}

	case *types.Slice:
		if new, ok := new.(*types.Slice); ok {
			return d.equiv(old.Elem(), new.Elem(), p)
		}

	case *types.Map:
		if new, ok := new.(*types.Map); ok {
			return d.equiv(old.Key(), new.Key(), p) && d.equiv(old.Elem(), new.Elem(), p)
		}

	case *types.Chan:
		if new, ok := new.(*types.Chan); ok {
			return d.equiv(old.Elem(), new.Elem(), p) && old.Dir() == new.Dir()
		}

	case *types.Pointer:
		if new, ok := new.(*types.Pointer); ok {
			return d.equiv(old.Elem(), new.Elem(), p)
		}

	case *types.Signature:
		if new, ok := new.(*types.Signature); ok {
			pe := d.equiv(old.Params(), new.Params(), p)
			re := d.equiv(old.Results(), new.Results(), p)
			return old.Variadic() == new.Variadic() && pe && re
		}

	case *types.Tuple:
		if new, ok := new.(*types.Tuple); ok {
			for i := 0; i < old.Len(); i++ {
				if i >= new.Len() || !d.equiv(old.At(i).Type(), new.At(i).Type(), p) {
					return false
				}
			}
			return old.Len() == new.Len()
		}

	case *types.Struct:
		if new, ok := new.(*types.Struct); ok {
			for i := 0; i < old.NumFields(); i++ {
				if i >= new.NumFields() {
					return false
				}
				of := old.Field(i)
				nf := new.Field(i)
				if of.Embedded() != nf.Embedded() ||
					old.Tag(i) != new.Tag(i) ||
					!d.equiv(of.Type(), nf.Type(), p) ||
					!d.equivFieldNames(of, nf) {
					return false
				}
			}
			return old.NumFields() == new.NumFields()
		}

	case *types.Interface:
		if new, ok := new.(*types.Interface); ok {
			// Deal with circularity. See the comment in types.Identical.
			q := &ifacePair{old, new, p}
			for p != nil {
				if p.identical(q) {
					return true // same pair was compared before
				}
				p = p.prev
			}
			oldms := d.sortedMethods(old)
			newms := d.sortedMethods(new)
			for i, om := range oldms {
				if i >= len(newms) {
					return false
				}
				nm := newms[i]
				if d.methodID(om) != d.methodID(nm) || !d.equiv(om.Type(), nm.Type(), q) {
					return false
				}
			}
			return old.NumMethods() == new.NumMethods()
		}

	case *types.Named:
		if new, ok := new.(*types.Named); ok {
			if old.Obj().Pkg() == d.old && new.Obj().Pkg() == d.new {
				return d.corresponds(old, new)
			}
			return old.Obj().Id() == new.Obj().Id()
		}

	default:
		panic("unknown type kind")
	}
	return false
}

// Compare old and new field names. We are determining equivalence across packages,
// so just compare names, not packages. For an unexported, embedded field of named
// type (non-named embedded fields are possible with aliases), we check that the type
// names correspond. We check the types for equivalence before this is called, so
// we've established correspondence.
func (d *differ) equivFieldNames(of, nf *types.Var) bool {
	if of.Embedded() && nf.Embedded() && !of.Exported() && !nf.Exported() {
		if on, ok := of.Type().(*types.Named); ok {
			nn := nf.Type().(*types.Named)
			return d.corresponds(on, nn)
		}
	}
	return of.Name() == nf.Name()
}

func (d *differ) sortedMethods(iface *types.Interface) []*types.Func {
	ms := make([]*types.Func, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		ms[i] = iface.Method(i)
	}
	sort.Slice(ms, func(i, j int) bool { return d.methodID(ms[i]) < d.methodID(ms[j]) })
	return ms
}

func (d *differ) methodID(m *types.Func) string {
	// If the method belongs to one of the two packages being compared, use
	// just its name even if it's unexported. That lets us treat unexported names
	// from the old and new packages as equal.
	if m.Pkg() == d.old || m.Pkg() == d.new {
		return m.Name()
	}
	return m.Id()
}

// Copied from the go/types package:

// An ifacePair is a node in a stack of interface type pairs compared for identity.
type ifacePair struct {
	x, y *types.Interface
	prev *ifacePair
}

func (p *ifacePair) identical(q *ifacePair) bool {
	return p.x == q.x && p.y == q.y || p.x == q.y && p.y == q.x
}
