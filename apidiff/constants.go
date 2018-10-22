package apidiff

import (
	"go/constant"
	"go/types"
	"math"
)

// We check these against const values for representability.
var basicKinds = []types.BasicKind{
	types.Int8,
	types.Int16,
	types.Int32,
	types.Int64,
	types.Uint8,
	types.Uint16,
	types.Uint32,
	types.Uint64,
	types.Float32,
	types.Float64,
	types.Complex64,
	types.Complex128,
}

// Compare two constants.
func (d *differ) constChanges(old, new *types.Const) {
	ot := old.Type()
	nt := new.Type()
	// Check for change of type.
	if !d.equivalent(ot, nt) {
		d.typeChanged(old, "", ot, nt)
		return
	}
	// Check that the new set of representable types is a superset of the old.
	for _, k := range basicKinds {
		typ := types.Typ[k]
		if isRepresentable(old.Val(), typ) && !isRepresentable(new.Val(), typ) {
			d.incompatible(old, "", "can no longer represent %s", typ)
			break // report only one type
		}
	}
}

// isRepresentable reports whether a contant value is representable by a type.
//
// Copied from representableConst in the go/types package, with modifications.
func isRepresentable(x constant.Value, typ *types.Basic) bool {
	if x.Kind() == constant.Unknown {
		return true
	}

	switch {
	case isInteger(typ):
		x := constant.ToInt(x)
		if x.Kind() != constant.Int {
			return false
		}
		if x, ok := constant.Int64Val(x); ok {
			switch typ.Kind() {
			case types.Int:
				unreachable()
			case types.Int8:
				const s = 8
				return -1<<(s-1) <= x && x <= 1<<(s-1)-1
			case types.Int16:
				const s = 16
				return -1<<(s-1) <= x && x <= 1<<(s-1)-1
			case types.Int32:
				const s = 32
				return -1<<(s-1) <= x && x <= 1<<(s-1)-1
			case types.Int64, types.UntypedInt:
				return true
			case types.Uint, types.Uintptr:
				unreachable()
				return 0 <= x
			case types.Uint8:
				const s = 8
				return 0 <= x && x <= 1<<s-1
			case types.Uint16:
				const s = 16
				return 0 <= x && x <= 1<<s-1
			case types.Uint32:
				const s = 32
				return 0 <= x && x <= 1<<s-1
			case types.Uint64:
				return 0 <= x
			default:
				unreachable()
			}
		}
		// x does not fit into int64
		switch n := constant.BitLen(x); typ.Kind() {
		case types.Uint, types.Uintptr:
			unreachable()
		case types.Uint64:
			return constant.Sign(x) >= 0 && n <= 64
		case types.UntypedInt:
			return true
		}

	case isFloat(typ):
		x := constant.ToFloat(x)
		if x.Kind() != constant.Float {
			return false
		}
		switch typ.Kind() {
		case types.Float32:
			return fitsFloat32(x)
		case types.Float64:
			return fitsFloat64(x)
		case types.UntypedFloat:
			return true
		default:
			unreachable()
		}

	case isComplex(typ):
		x := constant.ToComplex(x)
		if x.Kind() != constant.Complex {
			return false
		}
		switch typ.Kind() {
		case types.Complex64:
			return fitsFloat32(constant.Real(x)) && fitsFloat32(constant.Imag(x))
		case types.Complex128:
			return fitsFloat64(constant.Real(x)) && fitsFloat64(constant.Imag(x))
		case types.UntypedComplex:
			return true
		default:
			unreachable()
		}

	case isString(typ):
		return x.Kind() == constant.String

	case isBoolean(typ):
		return x.Kind() == constant.Bool
	}

	return false
}

func isFloat(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsFloat != 0
}

func isComplex(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsComplex != 0
}

func isString(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsString != 0
}

func isBoolean(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsBoolean != 0
}

func isInteger(typ types.Type) bool {
	t, ok := typ.Underlying().(*types.Basic)
	return ok && t.Info()&types.IsInteger != 0
}

func fitsFloat32(x constant.Value) bool {
	f32, _ := constant.Float32Val(x)
	f := float64(f32)
	return !math.IsInf(f, 0)
}

func fitsFloat64(x constant.Value) bool {
	f, _ := constant.Float64Val(x)
	return !math.IsInf(f, 0)
}

func unreachable() {
	panic("unreachable")
}
