// Package decimal...
package decimal

import "math/big"

// Big is a fixed-point, arbitrary-precision decimal number.
//
// A Big decimal is a number and a scale, the latter representing the number
// of digits following the radix if the scale is greater-than or equal to
// zero. Otherwise, it's the number times ten raised to the negation of the
// scale.
type Big struct {
	// Big is laid out like this so it takes up as little memory as possible.
	compact  int64 // used if |v| <= 2^63-1
	scale    int32
	ctx      Context
	form     form
	mantissa big.Int
}

// form indicates whether the Big decimal is norma or infinite.
type form byte

// Do not changese these constants---their order is important.
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

// New creates a new Big decimal with the given value and scale.
func New(value int64, scale int32) *Big {
	return new(Big).SetMantScale(value, scale)
}

// Abs sets z to the absolute value of x if x is finite and returns z.
func (z *Big) Abs(x *Big) *Big {
	panic("not implemented")
}

// Add sets z to x + y and returns z.
func (z *Big) Add(x *Big, y *Big) *Big {
	panic("not implemented")
}

// BitLen returns the absolute value of x in bits.
func (x *Big) BitLen() int {
	panic("not implemented")
}

// Cmp compares d and x and returns:
//
//   -1 if z <  x
//    0 if z == x
//   +1 if z >  x
//
// It does not modify z or x.
func (z *Big) Cmp(x *Big) int {
	panic("not implemented")
}

// Context returns x's Context.
func (x *Big) Context() Context {
	panic("not implemented")
}

// Int returns x as a big.Int, truncating the fractional portion, if any.
func (x *Big) Int() *big.Int {
	panic("not implemented")
}

// Int64 returns x as an int64, truncating the fractional portion, if any.
func (x *Big) Int64() int64 {
	panic("not implemented")
}

// IsBig returns true if x, with its fractional part truncated, cannot fit
// inside an int64.
func (x *Big) IsBig() bool {
	panic("not implemented")
}

// IsFinite returns true if x is finite.
func (x *Big) IsFinite() bool {
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
func (z *Big) Mul(x *Big, y *Big) *Big {
	panic("not implemented")
}

// Neg sets z to -x and returns z.
func (z *Big) Neg(x *Big) *Big {
	panic("not implemented")
}

// PlainString returns the plain string representation of x.
// For special cases, if x == nil returns "<nil>" and x.IsInf() returns "Inf".
func (x *Big) PlainString() string {
	panic("not implemented")
}

// Prec returns the precision of z. That is, it returns the number of
// decimal digits z requires.
func (x *Big) Prec() int {
	panic("not implemented")
}

// Quo sets z to x / y and returns z.
func (z *Big) Quo(x *Big, y *Big) *Big {
	panic("not implemented")
}

// Round rounds z down to n digits of precision and returns z. The result is
// undefined if n is less than zero. No rounding will occur if n is zero.
// The result of Round will always be within the interval [⌊z⌋, z].
func (x *Big) Round(n int32) *Big {
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
func (x *Big) SetBigMantScale(value *big.Int, scale int32) *Big {
	panic("not implemented")
}

// SetContext sets z's Context and returns z.
func (x *Big) SetContext(ctx Context) *Big {
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
func (x *Big) SetFloat64(value float64) *Big {
	panic("not implemented")
}

// SetInf sets z to Inf and returns z.
func (x *Big) SetInf() *Big {
	panic("not implemented")
}

// SetMantScale sets z to the given value and scale.
func (x *Big) SetMantScale(value int64, scale int32) *Big {
	panic("not implemented")
}

// SetMode sets z's RoundingMode to mode and returns z.
func (x *Big) SetMode(mode RoundingMode) *Big {
	panic("not implemented")
}

// SetPrec sets z's precision to prec and returns z.
// This method is distinct from Prec. This sets the internal context which
// dictates rounding and digits after the radix for lossy operations. The
// latter describes the number of digits in the decimal.
func (x *Big) SetPrec(prec int32) *Big {
	panic("not implemented")
}

// SetScale sets z's scale to scale and returns z.
func (x *Big) SetScale(scale int32) *Big {
	panic("not implemented")
}

// SetString sets z to the value of s, returning z and a bool
// indicating success. s must be a string in one of the following
// formats:
//
// 	1.234
// 	1234
// 	1.234e+5
// 	1.234E-5
// 	0.000001234
// 	Inf
// 	+Inf
// 	-Inf
//
//	No distinction is made between +Inf and -Inf.
func (x *Big) SetString(s string) (*Big, bool) {
	panic("not implemented")
}

// Sign returns:
//
//	-1 if x <   0
//	 0 if x is ±0
//	+1 if x >   0
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
// For special cases, x == nil returns "<nil>" and x.IsInf() returns "Inf".
func (x *Big) String() string {
	panic("not implemented")
}

// Sub sets z to x - y and returns z.
func (z *Big) Sub(x *Big, y *Big) *Big {
	panic("not implemented")
}

// UnmarshalText implements encoding/TextUnmarshaler.
func (x *Big) UnmarshalText(data []byte) error {
	panic("not implemented")
}
