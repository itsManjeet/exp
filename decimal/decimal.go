// Package decimal provides a high-performance, arbitrary precision, fixed-point
// decimal library.
//
// The following type is supported:
//
// 		Big decimal numbers
//
// The zero value for a Big corresponds with 0. Its method naming is the same
// as :math/big's, meaning:
//
//  	func (z *T) SetV(v V) *T          // z = v
//  	func (z *T) Unary(x *T) *T        // z = unary x
//  	func (z *T) Binary(x, y *T) *T    // z = x binary y
//  	func (x *T) Pred() P              // p = pred(x)
//
// In general, its conventions will mirror math/big's.
//
// Compared to other decimal libraries, this package:
//
// 		Does not have a distinction between positive and negative Infinity.
// 		Will panic on NaN values, similar to math/big.Float
//
package decimal

import (
	"bytes"
	"math/big"
	"strconv"
	"strings"

	"golang.org/x/exp/decimal/internal/arith"
	"golang.org/x/exp/decimal/internal/arith/checked"
	"golang.org/x/exp/decimal/internal/c"
)

const debug = false

// Big is a fixed-point, arbitrary-precision decimal number.
//
// A Big decimal is a number and a scale, the latter representing the number
// of digits following the radix if the scale is >= 0. Otherwise, it's the
// number * 10 ^ -scale.
type Big struct {
	// Big is laid out like this so it takes up as little memory as possible.
	//
	// compact is use if the value fits into an int64. The scale does not affect
	// whether this field is used; typically if an decimal has <= 19 digits this
	// field will be used.
	compact int64

	// scale is the number of digits following the radix. If scale is negative
	// the * 10 and ^ -scale is implied--it does not inflated either the
	// compact or unscaled fields.
	scale int32

	ctx      Context
	form     form
	unscaled big.Int
}

func (x *Big) isCompact() bool {
	return x.compact != c.Inflated
}

func (x *Big) isInflated() bool {
	return x.compact == c.Inflated
}

// form indicates whether the Big decimal is zero, normal, or infinite.
type form byte

// Do not change these constants--their order is important.
const (
	zero form = iota
	finite
	inf
)

// An ErrNaN panic is raised by a Decimal operation that would lead to a NaN
// under IEEE-754 rules. An ErrNaN implements the error interface.
type ErrNaN struct {
	msg string
}

func (e ErrNaN) Error() string {
	return e.msg
}

// New creates a new Big decimal with the given value and scale. For example:
//
//  	New(1234, 3) // 1.234
//  	New(42, 0)   // 42
//  	New(4321, 5) // 0.04321
//  	New(-1, 0)   // -1
//  	New(3, -10)  // 30,000,000,000
//
func New(value int64, scale int32) *Big {
	return new(Big).SetMantScale(value, scale)
}

// Abs sets z to the absolute value of x if x is finite and returns z.
func (z *Big) Abs(x *Big) *Big {
	panic("not implemented")
}

// Add sets z to x + y and returns z.
func (z *Big) Add(x, y *Big) *Big {
	if x.form == finite && y.form == finite {
		z.form = finite
		if x.isCompact() {
			if y.isCompact() {
				return z.addCompact(x, y)
			}
			return z.addMixed(x, y)
		}
		if y.isCompact() {
			return z.addMixed(y, x)
		}
		return z.addBig(x, y)
	}

	if x.form == inf && y.form == inf &&
		x.SignBit() != y.SignBit() {
		// +Inf + -Inf
		// -Inf + +Inf
		z.form = zero
		panic(ErrNaN{"addition of infinities with opposing signs"})
	}

	if x.form == zero && y.form == zero {
		// ±0 + ±0
		z.form = zero
		return z
	}

	if x.form == inf || y.form == zero {
		// ±Inf + y
		// x + ±0
		return z.Set(x)
	}

	// ±0 + y
	// x + ±Inf
	return z.Set(y)
}

// addCompact sets z to x + y and returns z.
func (z *Big) addCompact(x, y *Big) *Big {
	// Fast path: if the scales are the same we can just add
	// without adjusting either number.
	if x.scale == y.scale {
		z.scale = x.scale
		sum, ok := checked.Add(x.compact, y.compact)
		if ok {
			z.compact = sum
			if sum == 0 {
				z.form = zero
			}
		} else {
			z.unscaled.Add(big.NewInt(x.compact), big.NewInt(y.compact))
			z.compact = c.Inflated
			if z.unscaled.Sign() == 0 {
				z.form = zero
			}
		}
		return z
	}

	// Guess the scales. We need to inflate lo.
	hi, lo := x, y
	if hi.scale < lo.scale {
		hi, lo = lo, hi
	}

	// Power of 10 we need to multiply our lo value by in order
	// to equalize the scales.
	inc := hi.scale - lo.scale
	z.scale = hi.scale

	scaledLo, ok := checked.MulPow10(lo.compact, inc)
	if ok {
		sum, ok := checked.Add(hi.compact, scaledLo)
		if ok {
			z.compact = sum
			return z
		}
	}
	scaled := checked.MulBigPow10(big.NewInt(lo.compact), inc)
	z.unscaled.Add(scaled, big.NewInt(hi.compact))
	z.compact = c.Inflated
	if z.unscaled.Sign() == 0 {
		z.form = zero
	}
	return z
}

// addMixed adds a compact Big with a non-compact Big.
func (z *Big) addMixed(comp, non *Big) *Big {
	if comp.scale == non.scale {
		z.unscaled.Add(big.NewInt(comp.compact), &non.unscaled)
		z.scale = comp.scale
		z.compact = c.Inflated
		if z.unscaled.Sign() == 0 {
			z.form = zero
		}
		return z
	}
	// Since we have to rescale we need to add two big.Ints together because
	// big.Int doesn't have an API for increasing its value by an integer.
	return z.addBig(&Big{
		unscaled: *big.NewInt(comp.compact),
		scale:    comp.scale,
	}, non)
}

// addBig sets z to x + y and returns z.
func (z *Big) addBig(x, y *Big) *Big {
	hi, lo := x, y
	if hi.scale < lo.scale {
		hi, lo = lo, hi
	}

	inc := hi.scale - lo.scale
	scaled := checked.MulBigPow10(&lo.unscaled, inc)
	z.unscaled.Add(&hi.unscaled, scaled)
	z.compact = c.Inflated
	z.scale = hi.scale
	if z.unscaled.Sign() == 0 {
		z.form = zero
	}
	return z
}

// Cmp compares x and y and returns:
//
//   -1 if z <  x
//    0 if z == x
//   +1 if z >  x
//
// It does not modify x or y.
func (x *Big) Cmp(y *Big) int {
	// Check for same pointers.
	if x == y {
		return 0
	}

	// Same scales means we can compare straight across.
	if x.scale == y.scale {
		if x.isCompact() && y.isCompact() {
			if x.compact > y.compact {
				return +1
			}
			if x.compact < y.compact {
				return -1
			}
			return 0
		}
		if x.isInflated() && y.isInflated() {
			if x.unscaled.Sign() != y.unscaled.Sign() {
				return x.unscaled.Sign()
			}

			if x.scale < 0 {
				return x.unscaled.Cmp(&y.unscaled)
			}

			zb := x.unscaled.Bits()
			xb := y.unscaled.Bits()

			min := len(zb)
			if len(xb) < len(zb) {
				min = len(xb)
			}
			i := 0
			for i < min-1 && zb[i] == xb[i] {
				i++
			}
			if zb[i] > xb[i] {
				return +1
			}
			if zb[i] < xb[i] {
				return -1
			}
			return 0
		}
	}

	// Different scales -- check signs and/or if they're
	// both zero.

	ds := x.Sign()
	xs := y.Sign()
	switch {
	case ds > xs:
		return +1
	case ds < xs:
		return -1
	case ds == 0 && xs == 0:
		return 0
	}

	// Scales aren't equal, the signs are the same, and both
	// are non-zero.
	dl := int32(x.Prec()) - x.scale
	xl := int32(y.Prec()) - y.scale
	if dl > xl {
		return +1
	}
	if dl < xl {
		return -1
	}

	// We need to inflate one of the numbers.

	dc := x.compact // hi
	xc := y.compact // lo

	var swap bool

	hi, lo := x, y
	if hi.scale < lo.scale {
		hi, lo = lo, hi
		dc, xc = xc, dc
		swap = true // d is lo
	}

	diff := hi.scale - lo.scale
	if diff <= c.BadScale {
		var ok bool
		xc, ok = checked.MulPow10(xc, diff)
		if !ok && dc == c.Inflated {
			// d is lo
			if swap {
				zm := new(big.Int).Set(&x.unscaled)
				return checked.MulBigPow10(zm, diff).Cmp(&y.unscaled)
			}
			// x is lo
			xm := new(big.Int).Set(&y.unscaled)
			return x.unscaled.Cmp(checked.MulBigPow10(xm, diff))
		}
	}

	if swap {
		dc, xc = xc, dc
	}

	if dc != c.Inflated {
		if xc != c.Inflated {
			return arith.AbsCmp(dc, xc)
		}
		return big.NewInt(dc).Cmp(&y.unscaled)
	}
	if xc != c.Inflated {
		return x.unscaled.Cmp(big.NewInt(xc))
	}
	return x.unscaled.Cmp(&y.unscaled)
}

// Context returns the Context of x.
func (x *Big) Context() Context {
	panic("not implemented")
}

// Int returns x as a big.Int, truncating the fractional portion, if any.
func (x *Big) Int() *big.Int {
	panic("not implemented")
}

// Int64 returns x as an int64, truncating the fractional portion, if any. The
// result is undefined if x cannot fit inside an int64.
func (x *Big) Int64() int64 {
	panic("not implemented")
}

// IsInt64 returns true if x, with its fractional part truncated, can fit
// inside an int64.
func (x *Big) IsInt64() bool {
	panic("not implemented")
}

// IsInf returns true if x is an infinity.
func (x *Big) IsInf() bool {
	panic("not implemented")
}

// IsInt reports whether x is an integer.
// ±Inf values are not integers.
func (x *Big) IsInt() bool {
	panic("not implemented")
}

// MarshalText implements encoding/TextMarshaler.
func (x *Big) MarshalText() ([]byte, error) {
	panic("not implemented")
}

// Mode returns the rounding mode of x.
func (x *Big) Mode() RoundingMode {
	panic("not implemented")
}

// Mode returns the rounding mode of x.
func (z *Big) Modf(x *Big) (int *Big, frac *Big) {
	panic("not implemented")
}

// Mul sets z to x * y and returns z.
func (z *Big) Mul(x, y *Big) *Big {
	panic("not implemented")
}

// Neg sets z to -x and returns z.
func (z *Big) Neg(x *Big) *Big {
	panic("not implemented")
}

// PlainString returns the plain string representation of x. A plain string is
// the full decimal representation of a Big decimal. For example,
// 6,720,000,000 instead of 6.72x10^9. Special cases are the same as String.
func (x *Big) PlainString() string {
	return x.toString(false, lower)
}

// Prec returns the precision of x. That is, it returns the number of
// decimal digits required to fully represent x.
func (x *Big) Prec() int {
	panic("not implemented")
}

// Quo sets z to x / y and returns z.
func (z *Big) Quo(x, y *Big) *Big {
	panic("not implemented")
}

// Round rounds z down to n digits of precision and returns z. The result is
// undefined if n is less than zero. No rounding will occur if n is zero.
// The result of Round will always be within the interval [⌊z⌋, z].
func (z *Big) Round(n int32) *Big {
	panic("not implemented")
}

// Scale returns x's scale.
func (x *Big) Scale() int32 {
	panic("not implemented")
}

// Set sets z to x and returns z.
func (z *Big) Set(x *Big) *Big {
	panic("not implemented")
}

// SetBigMantScale sets z to the given value and scale.
func (z *Big) SetBigMantScale(value *big.Int, scale int32) *Big {
	panic("not implemented")
}

// SetContext sets z's Context and returns z.
func (z *Big) SetContext(ctx Context) *Big {
	panic("not implemented")
}

// TODO: Should this set the Big decimal to exactly the float64 or a rounded
// version?

// SetFloat64 sets z to the provided float64.
//
// Remember floating-point to decimal conversions can be lossy. For example,
// the floating-point number `0.1' appears to simply be 0.1, but its actual
// value is 0.1000000000000000055511151231257827021181583404541015625.
//
// SetFloat64 is particularly lossy because will round non-integer values.
// For example, if passed the value `3.1415' it attempts to do the same as if
// SetMantScale(31415, 4) were called.
//
// To do this, it scales up the provided number by its scale. This involves
// rounding, so approximately 2.3% of decimals created from floats will have a
// rounding imprecision of ± 1 ULP.
func (z *Big) SetFloat64(value float64) *Big {
	panic("not implemented")
}

// SetInf sets z to Inf and returns z.
func (z *Big) SetInf() *Big {
	panic("not implemented")
}

// SetMantScale sets z to the given value and scale.
func (z *Big) SetMantScale(value int64, scale int32) *Big {
	panic("not implemented")
}

// SetMode sets z's RoundingMode to mode and returns z.
func (z *Big) SetMode(mode RoundingMode) *Big {
	panic("not implemented")
}

// SetPrec sets z's precision to prec and returns z.
// This method is distinct from Prec. This sets the internal context which
// dictates rounding and digits after the radix for lossy operations. The
// latter describes the number of digits in the decimal.
func (z *Big) SetPrec(prec int32) *Big {
	panic("not implemented")
}

// SetScale sets z's scale to scale and returns z.
func (z *Big) SetScale(scale int32) *Big {
	panic("not implemented")
}

// SetString sets z to the value of s, returning z and a bool
// indicating success. s must be a string in one of the following
// formats:
//
// 		1.234
// 		1234
// 		1.234e+5
// 		1.234E-5
// 		0.000001234
// 		Inf
// 		+Inf
// 		-Inf
//
//	No distinction is made between +Inf and -Inf.
func (z *Big) SetString(s string) (*Big, bool) {
	// Inf, +Inf, or -Inf
	if strings.EqualFold(s, "Inf") ||
		(len(s) == 4 && (s[0] == '+' || s[0] == '-') &&
			strings.EqualFold(s[1:], "Inf")) {
		z.form = inf
		return z, true
	}

	var scale int32

	// Check for a scientific string.
	i := strings.LastIndexAny(s, "Ee")
	if i > 0 {
		eint, err := strconv.ParseInt(s[i+1:], 10, 32)
		if err != nil {
			return nil, false
		}
		s = s[:i]
		scale = -int32(eint)
	}

	switch strings.Count(s, ".") {
	case 0:
	case 1:
		i = strings.IndexByte(s, '.')
		s = s[:i] + s[i+1:]
		scale += int32(len(s) - i)
	default:
		return nil, false
	}

	var err error
	// Numbers == 19 can be out of range, but try the edge case anyway.
	if len(s) <= 19 {
		z.compact, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			nerr, ok := err.(*strconv.NumError)
			if !ok || nerr.Err == strconv.ErrSyntax {
				return nil, false
			}
			err = nerr.Err
		}
	}
	if (err == strconv.ErrRange && len(s) == 19) || len(s) > 19 {
		_, ok := z.unscaled.SetString(s, 10)
		if !ok {
			return nil, false
		}
		z.compact = c.Inflated
	}
	z.scale = scale
	z.form = finite
	return z, true
}

// Sign returns:
//
//	-1 if x <  0
//	 0 if x is 0
//	+1 if x >  0
//
// Undefined if x is ±Inf.
func (x *Big) Sign() int {
	panic("not implemented")
}

// SignBit returns true if x is negative.
func (x *Big) SignBit() bool {
	panic("not implemented")
}

// Sqrt sets z to the square root of x and returns z. The precision of Sqrt
// is determined by z's Context. Sqrt will panic on negative values since
// Big cannot represent imaginary numbers.
func (z *Big) Sqrt(x *Big) *Big {
	panic("not implemented")
}

// String returns the scientific string representation of x.
// Special cases are:
//
//  	x == nil = "<nil>"
//  	x.IsInf() = "Inf".
//
func (x *Big) String() string {
	return x.toString(true, lower)
}

const (
	lower = 0 // opts for lowercase sci notation
	upper = 1 // opts for uppercase sci notation
)

func (x *Big) toString(sci bool, opts byte) string {
	if x == nil {
		return "<nil>"
	}
	if x.IsInf() {
		return "Inf"
	}

	// Fast path: return our value as-is.
	if x.scale == 0 {
		if x.isInflated() {
			return x.unscaled.String()
		}
		return strconv.FormatInt(x.compact, 10)
	}

	// Keep from allocating a buffer if x is zero.
	if (x.form == zero ||
		x.isCompact() && x.compact == 0) ||
		(x.isInflated() && x.unscaled.Sign() == 0) {
		return "0"
	}

	// (x.scale > 0 || x.scale < 0) && x != 0

	var (
		str string
		b   buffer // is bytes.Buffer
	)

	if x.isInflated() {
		str = x.unscaled.String()
	} else {
		str = strconv.FormatInt(x.compact, 10)
	}

	// Either this or we have to use a boolean flag plus format the unscaled
	// and/or compact integer as unsigned values which incurs
	// an allocation with the unscaled.
	if str[0] == '-' {
		b.WriteByte('-')
		str = str[1:]
	}
	if sci {
		return x.toSciString(str, &b, opts)
	}
	return x.toPlainString(str, &b)
}

func (x *Big) toSciString(str string, b writer, opts byte) string {
	if debug && (opts < 0 || opts > 1) {
		panic("toSciString: (bug) opts != 0 || opts != 1")
	}

	// Following quotes are from:
	// http://speleotrove.com/decimal/daconvs.html#reftostr

	adj := -int(x.scale) + (len(str) - 1)
	pos := adj > 0

	// "If the exponent is less than or equal to zero and the
	// adjusted exponent is greater than or equal to -6..."
	if x.scale >= 0 && adj >= -6 {
		// "...the number will be converted to a character
		// form without using exponential notation."
		return x.normString(str, b)
	}

	b.WriteByte(str[0])
	if len(str) > 1 {
		b.WriteByte('.')
		b.WriteString(str[1:])
	}
	if adj != 0 {
		b.WriteByte([2]byte{'e', 'E'}[opts])
		// If !pos the following strconv.Itoa call will add
		// the minus sign for us.
		if pos {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(adj))
	}
	return b.String()
}

// toPlainString returns the plain string version of x.
func (x *Big) toPlainString(str string, b writer) string {
	// Just unscaled + z.scale "0"s -- no radix.
	if x.scale < 0 {
		b.WriteString(str)
		b.Write(bytes.Repeat([]byte{'0'}, -int(x.scale)))
		return b.String()
	}
	return x.normString(str, new(buffer))
}

// normString returns the plain string version of x.
// It's distinct from 'toPlainString' in that toPlainString
// calls this method once it's done its own internal checks.
// Additionally, toSciString also calls this method if it
// does not need to add the {e,E} suffix.
func (x *Big) normString(str string, b writer) string {
	switch pad := len(str) - int(x.scale); {

	// log10(unscaled) == scale, so immediately before str.
	case pad == 0:
		b.WriteString("0.")
		b.WriteString(str)

	// log10(unscaled) > scale, so somewhere inside str.
	case pad > 0:
		b.WriteString(str[:pad])
		b.WriteByte('.')
		b.WriteString(str[pad:])

	// log10(unscaled) < scale, so before p "0s" and before str.
	default:
		b.WriteString("0.")
		b.Write(bytes.Repeat([]byte{'0'}, -pad))
		b.WriteString(str)
	}
	return b.String()
}

// Sub sets z to x - y and returns z.
func (z *Big) Sub(x, y *Big) *Big {
	panic("not implemented")
}

// UnmarshalText implements encoding/TextUnmarshaler.
func (x *Big) UnmarshalText(data []byte) error {
	panic("not implemented")
}
