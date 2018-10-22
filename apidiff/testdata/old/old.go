// Comments beginning with an "i" are expected messages for incompatible changes.
// Comments beginning with a "c", for compatible changes.

package old

import (
	"io"
	"math"
)

type (
	A int

	u string

	u1 int
	u2 int
	u3 int
	u4 int

	Split1 = u1
	Split2 = u1

	GoodMerge1 = u2
	GoodMerge2 = u3

	BadMerge1 = u3
	BadMerge2 = u4
)

func (u4) M() {}

const (
	C1           = 1
	C2 int       = 2
	C3           = 3
	C4 u1        = 4
	C5 bool      = true
	C6 uint      = 5
	C7 string    = "7"
	C8 float64   = 3.1
	C9 complex64 = -2i

	// representability
	Cr1 = 1
	Cr2 = math.MaxInt8
	Cr3 = math.MaxFloat32
	Cr4 = complex(0, math.MaxFloat32)

	// value changes that break conversions (unimplemented)
	// Cc1 = 1.0
	// Cc2 = 1 + 0i
)

var (
	V1  string
	V2  u
	V3  A
	V4  u
	V5  u1
	V7  <-chan int
	V8  int
	V9  interface{ M() }
	V10 interface{ M() }
	V11 interface{ M() }

	VS1 struct{ A, B int }
	VS2 struct{ A, B int }
	VS3 struct{ A, B int }
	VS4 struct {
		A int
		u1
	}
)

type (
	A1 [1]int
	A2 [C3]int
	A3 [2]int

	Sl []int

	P1 *int
	P2 *u1

	M1 map[string]int
	M2 map[string]int
	M3 map[string]int

	Ch1 chan int
	Ch2 <-chan int
	Ch3 chan int
	Ch4 <-chan int

	I1 interface {
		M1()
		M2()
	}

	I2 interface {
		M1()
		m()
	}

	I3 interface {
		io.Reader
		M()
	}

	// This tests merging types + aliases, which is not supported yet.
	// I4 interface {
	// 	Write([]byte) (int, error)
	// }

	I5 io.Writer

	I6 = io.Writer
)

func F1(a int, b string) map[u1]A { return nil }
func F2(int)                      {}
func F3(int)                      {}
func F4(int) int                  { return 0 }
func F5(int) int                  { return 0 }
func F6(int)                      {}
func F7(interface{})              {}
func F8(bool)                     {}

type S1 struct {
	A int
	B string
	C bool
	d float32
}

type embed struct {
	E string
}

type S2 struct {
	A int
	embed
}

type S3 struct {
	A int
	embed
}

type F int

type embed2 struct {
	embed3
	F // shadows embed3.F

}

type embed3 struct {
	F bool
}

type alias = struct{ D bool }

// This is also used for testing exportedFields.
// Its exported fields are:
//   A1 [1]int
//   D bool
//   E int
//   F F
//   S4 *S4
type S4 struct {
	int
	*embed2
	embed
	E int // shadows embed.E
	alias
	A1
	*S4
}

// Difference between exported field set and exported shape.
type S5 struct {
	A int
}

// Exported fields: A int, S5 S5
// Exported literal keys: A, S5
type S6 struct {
	S5 S5
	A  int
}

// Method sets.

type SM struct {
	embedm
	Embedm
}

func (SM) V1() {}
func (SM) V2() {}
func (SM) V3() {}
func (SM) V4() {}
func (SM) v()  {}

func (*SM) P1() {}
func (*SM) P2() {}
func (*SM) P3() {}
func (*SM) P4() {}
func (*SM) p()  {}

type embedm int

func (embedm) EV1()  {}
func (embedm) EV2()  {}
func (embedm) EV3()  {}
func (*embedm) EP1() {}
func (*embedm) EP2() {}
func (*embedm) EP3() {}

type Embedm struct {
	A int
}

func (Embedm) FV()  {}
func (*Embedm) FP() {}

type RepeatEmbedm struct {
	Embedm
}

var Z w

type w []x
type x []z
type z int

type H struct{}

func (H) M() {}

type Rem int

// The whole-package interface satisfaction test.

type WI1 interface {
	M1()
	m1()
}

type WI2 interface {
	M2()
	m2()
}

type WS1 int

func (WS1) M1() {}
func (WS1) m1() {}

type WS2 int

func (WS2) M2() {}
func (WS2) m2() {}
