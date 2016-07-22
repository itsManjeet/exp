// Package decimal...
package decimal

import "math/big"

type Big struct{}

func (z *Big) Abs(x *Big) *Big {
	panic("not implemented")
}

func (z *Big) Add(x *Big, y *Big) *Big {
	panic("not implemented")
}

func (x *Big) BitLen() int {
	panic("not implemented")
}

func (z *Big) Cmp(x *Big) int {
	panic("not implemented")
}

func (x *Big) Context() Context {
	panic("not implemented")
}

func (x *Big) Int() *big.Int {
	panic("not implemented")
}

func (x *Big) Int64() int64 {
	panic("not implemented")
}

func (x *Big) IsBig() bool {
	panic("not implemented")
}

func (x *Big) IsFinite() bool {
	panic("not implemented")
}

func (x *Big) IsInf() bool {
	panic("not implemented")
}

func (x *Big) IsInt() bool {
	panic("not implemented")
}

func (x *Big) MarshalText() ([]byte, error) {
	panic("not implemented")
}

func (x *Big) Mode() RoundingMode {
	panic("not implemented")
}

func (z *Big) Modf(x *Big) (int *Big, frac *Big) {
	panic("not implemented")
}

func (z *Big) Mul(x *Big, y *Big) *Big {
	panic("not implemented")
}

func (z *Big) Neg(x *Big) *Big {
	panic("not implemented")
}

func (x *Big) PlainString() string {
	panic("not implemented")
}

func (x *Big) Prec() int {
	panic("not implemented")
}

func (z *Big) Quo(x *Big, y *Big) *Big {
	panic("not implemented")
}

func (x *Big) Round(n int32) *Big {
	panic("not implemented")
}

func (x *Big) Scale() int32 {
	panic("not implemented")
}

func (z *Big) Set(x *Big) *Big {
	panic("not implemented")
}

func (x *Big) SetBigMantScale(value *big.Int, scale int32) *Big {
	panic("not implemented")
}

func (x *Big) SetContext(ctx Context) *Big {
	panic("not implemented")
}

func (x *Big) SetFloat64(value float64) *Big {
	panic("not implemented")
}

func (x *Big) SetInf() *Big {
	panic("not implemented")
}

func (x *Big) SetMantScale(value int64, scale int32) *Big {
	panic("not implemented")
}

func (x *Big) SetMode(mode RoundingMode) *Big {
	panic("not implemented")
}

func (x *Big) SetPrec(prec int32) *Big {
	panic("not implemented")
}

func (x *Big) SetScale(scale int32) *Big {
	panic("not implemented")
}

func (x *Big) SetString(s string) (*Big, bool) {
	panic("not implemented")
}

func (x *Big) Sign() int {
	panic("not implemented")
}

func (x *Big) SignBit() bool {
	panic("not implemented")
}

func (z *Big) Sqrt(x *Big) *Big {
	panic("not implemented")
}

func (x *Big) String() string {
	panic("not implemented")
}

func (z *Big) Sub(x *Big, y *Big) *Big {
	panic("not implemented")
}

func (x *Big) UnmarshalText(data []byte) error {
	panic("not implemented")
}
