package decimal

import "math"

// Precision and scale limits.
const (
	MaxScale = math.MaxInt32 // smallest allowed scale.
	MinScale = math.MinInt32 // largest allowed scale.

	MinPrec = 0             // smallest allowed context precision.
	MaxPrec = math.MaxInt32 // largest allowed context precision.
)

// DefaultPrec is the default precision used for decimals created as literals
// or using new.
const DefaultPrec = 16

// Context tells the lossy arithmetic operations how to do their jobs.
// Precision is the maximum number of digits to be used for the decimal. Mode
// instructs lossy operations how to round. Context is exported for this
// documentation but is not expected to be used itself.
type Context struct {
	precision int32
	mode      RoundingMode
}

// Precision returns c's precision.
func (c Context) Precision() int32 {
	return c.prec()
}

// prec is a slight hack to work around Go's zero-values. Were we to allow
// Context's zero-value to be used as-is, all newly-created decimals would have
// a precision of zero---very problematic. Because of this, the unexported
// prec method will return DefaultPrec if the precision is zero, zero if it's
// less than zero, and its current value if it's greater than zero.
func (c Context) prec() int32 {
	if c.precision == 0 {
		return DefaultPrec
	}
	if c.precision < 0 {
		return 0
	}
	return c.precision
}

// The following are called ContextXX instead of DecimalXX
// to reserve the DecimalXX namespace for future decimal types.

// The following Contexts are based on IEEE 754R.
var (
	// Context32 is the IEEE 754R Decimal32 format.
	// It has a precision of 7 and mode of ToNearestEven.
	Context32 = Context{precision: 7, mode: ToNearestEven}

	// Context64 is the IEEE 754R Decimal64 format.
	// It has a precision of 16 and mode of ToNearestEven.
	Context64 = Context{precision: 16, mode: ToNearestEven}

	// Context128 is the IEEE 754R Decimal128 format.
	// It has a precision of 34 and mode of ToNearestEven.
	Context128 = Context{precision: 34, mode: ToNearestEven}
)

// RoundingMode determines how a Decimal will be rounded
// if the exact result cannot accurately be represented.
type RoundingMode byte

// The following rounding modes are supported.
const (
	ToNearestEven RoundingMode = iota // == IEEE 754-2008 roundTiesToEven
	ToNearestAway                     // == IEEE 754-2008 roundTiesToAway
	ToZero                            // == IEEE 754-2008 roundTowardZero
	AwayFromZero                      // no IEEE 754-2008 equivalent
	ToNegativeInf                     // == IEEE 754-2008 roundTowardNegative
	ToPositiveInf                     // == IEEE 754-2008 roundTowardPositive

	// Unneeded means finite decimal expansion. Will panic if this RoundingMode
	// is provided and the lossy operation does not have a finite decimal
	// expansion.
	Unneeded
)

//go:generate stringer -type RoundingMode
