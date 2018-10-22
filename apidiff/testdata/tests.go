// This file is split into two packages, old and new.
// It is syntactically valid Go so that gofmt can process it.

package ignore

// old
import "io"

// both
import "math"

//// Same type in both: OK.
// both
type A int

//// Changing the type is an incompatible change.
// old
type B int

// new
// i B: changed from int to string
type B string

//// Adding a new type, whether alias or not, is a compatible change.
// new
// c AA: added
type AA = A

// c B1: added
type B1 bool

//// Change of type for an unexported name doesn't matter...
// old
type t int

// new
type t string

//// ...unless it is exposed.
// both
var V2 u

// old
type u string

// new
// i u: changed from string to int
type u int

//// An exposed, unexported type can be renamed.
// both
type u2 int

// old
type u1 int

var V5 u1

// new
var V5 u2 // no message, even though V5 has apparently changed type

//// Splitting a single type into two is an incompatible change.
// both
type u3 int

// old
type (
	Split1 = u1
	Split2 = u1
)

// new
type (
	Split1 = u2 // OK, since old u1 corresponds to new u2

	// This tries to make u1 correspond to u3
	// i Split2: changed from u1 to u3
	Split2 = u3
)

//// Merging two types into one is OK.
// old
type (
	GoodMerge1 = u2
	GoodMerge2 = u3
)

// new
type (
	GoodMerge1 = u3
	GoodMerge2 = u3
)

//// Merging isn't OK here because a method is lost.
// both
type u4 int

func (u4) M() {}

// old
type (
	BadMerge1 = u3
	BadMerge2 = u4
)

// new
type (
	BadMerge1 = u3
	// i u4.M: removed
	// What's really happening here is that old u4 corresponds to new u3,
	// and new u3's method set is not a superset of old u4's.
	BadMerge2 = u3
)

// old
type Rem int

// new
// i Rem: removed

//////////////// Constants

//// type changes
// old
const (
	C1     = 1
	C2 int = 2
	C3     = 3
	C4 u1  = 4
)

var V8 int

// new
const (
	// i C1: changed from untyped int to untyped string
	C1 = "1"
	// i C2: changed from int to untyped int
	C2 = -1
	// i C3: changed from untyped int to int
	C3 int = 3
	// i V8: changed from var to const
	V8 int = 1
	C4 u2  = 1 // OK: u1 corresponds to u2
)

// representability
// old
const (
	Cr1 = 1
	Cr2 = math.MaxInt8
	Cr3 = math.MaxFloat32
	Cr4 = complex(0, math.MaxFloat32)
)

// new
const (
	// i Cr1: can no longer represent uint8
	Cr1 = -1
	// i Cr2: can no longer represent int8
	Cr2 = math.MaxInt8 + 1
	// i Cr3: can no longer represent float32
	Cr3 = math.MaxFloat32 * 2
	// i Cr4: can no longer represent complex64
	Cr4 = complex(0, math.MaxFloat32*2)
)

//////////////// Variables

//// simple type changes
// old
var (
	V1 string
	V3 A
	V7 <-chan int
)

// new
var (
	// i V1: changed from string to []string
	V1 []string
	V3 A // OK: same
	// i V7: changed from <-chan int to chan int
	V7 chan int
)

//// interface type  changes
// old
var (
	V9  interface{ M() }
	V10 interface{ M() }
	V11 interface{ M() }
)

// new
var (
	// i V9: changed from interface{M()} to interface{}
	V9 interface{}
	// i V10: changed from interface{M()} to interface{M(); M2()}
	V10 interface {
		M2()
		M()
	}
	// i V11: changed from interface{M()} to interface{M(int)}
	V11 interface{ M(int) }
)

//// struct type changes
// old
var (
	VS1 struct{ A, B int }
	VS2 struct{ A, B int }
	VS3 struct{ A, B int }
	VS4 struct {
		A int
		u1
	}
)

// new
var (
	// i VS1: changed from struct{A int; B int} to struct{B int; A int}
	VS1 struct{ B, A int }
	// i VS2: changed from struct{A int; B int} to struct{A int}
	VS2 struct{ A int }
	// i VS3: changed from struct{A int; B int} to struct{A int; B int; C int}
	VS3 struct{ A, B, C int }
	VS4 struct {
		A int
		u2
	}
)

//////////////// Types

// old
const C5 = 3

type (
	A1 [1]int
	A2 [2]int
	A3 [C5]int
)

// new
const C5 = 4

type (
	A1 [1]int
	// i A2: changed from [2]int to [2]bool
	A2 [2]bool
	// i A3: changed from [3]int to [4]int
	A3 [C5]int
)

// old
type (
	Sl []int
	P1 *int
	P2 *u1
)

// new
type (
	// i Sl: changed from []int to []string
	Sl []string
	// i P1: changed from *int to **bool
	P1 **bool
	P2 *u2 // OK: u1 corresponds to u2
)

// old
type (
	M1 map[string]int
	M2 map[string]int
	M3 map[string]int
)

// new
type (
	M1 map[string]int
	// i M2: changed from map[string]int to map[int]int
	M2 map[int]int
	// i M3: changed from map[string]int to map[string]string
	M3 map[string]string
)

// old
type (
	Ch1 chan int
	Ch2 <-chan int
	Ch3 chan int
	Ch4 <-chan int
)

// new
type (
	// i Ch1, element type: changed from int to bool
	Ch1 chan bool
	// i Ch2: changed direction
	Ch2 chan<- int
	// i Ch3: changed direction
	Ch3 <-chan int
	// c Ch4: removed direction
	Ch4 chan int
)

// old
type I1 interface {
	M1()
	M2()
}

// new
type I1 interface {
	// M1()
	// i I1.M1: removed
	M2(int)
	// i I1.M2: changed from func() to func(int)
	M3()
	// i I1.M3: added
	m()
	// i I1.m: added unexported method
}

// old
type I2 interface {
	M1()
	m()
}

// new
type I2 interface {
	M1()
	// m() Removing an unexported method is OK.
	m2() // OK, because old already had an unexported method
	// c I2.M2: added
	M2()
}

// old
type I3 interface {
	io.Reader
	M()
}

// new
type I3 interface {
	M()
	Read([]byte) (int, error)
}

// 	// This tests merging types + aliases, which is not supported yet.
// 	// I4 interface {
// 	// 	Write([]byte) (int, error)
// 	// }

// 	I5 io.Writer

// 	I6 = io.Writer
// )

//////////////// Functions

// old
func F1(a int, b string) map[u1]A { return nil }
func F2(int)                      {}
func F3(int)                      {}
func F4(int) int                  { return 0 }
func F5(int) int                  { return 0 }
func F6(int)                      {}
func F7(interface{})              {}

// new
func F1(c int, d string) map[u2]AA { return nil } //OK: same (since u1 corresponds to u2)

// i F2: changed from func(int) to func(int) bool
func F2(int) bool { return true }

// i F3: changed from func(int) to func(int, int)
func F3(int, int) {}

// i F4: changed from func(int) int to func(bool) int
func F4(bool) int { return 0 }

// i F5: changed from func(int) int to func(int) string
func F5(int) string { return "" }

// i F6: changed from func(int) to func(...int)
func F6(...int) {}

// i F7: changed from func(interface{}) to func(interface{x()})
func F7(a interface{ x() }) {}

// old
func F8(bool) {}

// new
// c F8: changed from func to var
var F8 func(bool)

//////////////// Structs

// old
type S1 struct {
	A int
	B string
	C bool
	d float32
}

// new
type S1 = s1

type s1 struct {
	C chan int
	// i S1.C: changed from bool to chan int
	A int
	// i S1.B: removed
	x []int
	// i S1: old is comparable, new is not
	d float32
	E bool
	// c S1.E: added
}

// old
type embed struct {
	E string
}

type S2 struct {
	A int
	embed
}

// new
type embedx struct {
	E string
}

type S2 struct {
	embedx // OK: the unexported embedded field changed names, but the exported field didn't
	A      int
}

// both
type F int

// old
type S3 struct {
	A int
	embed
}

// new
type embed struct{ F int }

type S3 struct {
	// i S3.E: removed
	embed
	// c S3.F: added
	A int
}

// old
type embed2 struct {
	embed3
	F // shadows embed3.F
}

type embed3 struct {
	F bool
}

type alias = struct{ D bool }

type S4 struct {
	int
	*embed2
	embed
	E int // shadows embed.E
	alias
	A1
	*S4
}

// new
type S4 struct {
	// OK: removed unexported fields
	// D and F marked as added because they are now part of the immediate fields
	D bool
	// c S4.D: added
	E int // OK: same as in old
	F F
	// c S4.F: added
	A1  // OK: same
	*S4 // OK: same (recursive embedding)
}

// Difference between exported selectable fields and exported immediate fields.
// both
type S5 struct{ A int }

// old
// Exported immediate fields: A, S5
// Exported selectable fields: A int, S5 S5
type S6 struct {
	S5 S5
	A  int
}

// new
// Exported immediate fields: S5
// Exported selectable fields: A int, S5 S5.

// i S6.A: removed
type S6 struct {
	S5
}

//////////////// Method sets

// old
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

// new
type SM struct {
	embedm2
	embedm3
	Embedm
	// i SM.A: changed from int to bool
}

// c SMa: added
type SMa = SM

func (SM) V1() {} // OK: same

// func (SM) V2() {}
// i SM.V2: removed

// i SM.V3: changed from func() to func(int)
func (SM) V3(int) {}

// c SM.V5: added
func (SM) V5() {}

func (SM) v(int) {} // OK: unexported method change
func (SM) v2()   {} // OK: unexported method added

func (*SM) P1() {} // OK: same
//func (*SM) P2() {}
// i (*SM).P2: removed

// i (*SM).P3: changed from func() to func(int)
func (*SMa) P3(int) {}

// c (*SM).P5: added
func (*SM) P5() {}

// func (*SM) p() {}  // OK: unexported method removed

// Changing from a value to a pointer receiver or vice versa
// just looks like adding and removing a method.

// i SM.V4: removed
// i (*SM).V4: changed from func() to func(int)
func (*SM) V4(int) {}

// c SM.P4: added
// P4 is not removed from (*SM) because value methods
// are in the pointer method set.
func (SM) P4() {}

type embedm2 int

// i embedm.EV1: changed from func() to func(int)
func (embedm2) EV1(int) {}

// i embedm.EV2, method set of SM: removed
// i embedm.EV2, method set of *SM: removed

// i (*embedm).EP2, method set of *SM: removed
func (*embedm2) EP1() {}

type embedm3 int

func (embedm3) EV3()  {} // OK: compatible with old embedm.EV3
func (*embedm3) EP3() {} // OK: compatible with old (*embedm).EP3

type Embedm struct {
	// i Embedm.A: changed from int to bool
	A bool
}

// i Embedm.FV: changed from func() to func(int)
func (Embedm) FV(int) {}
func (*Embedm) FP()   {}

type RepeatEmbedm struct {
	// i RepeatEmbedm.A: changed from int to bool
	Embedm
}

//////////////// Miscellany

// This verifies that the code works even through
// multiple levels of unexported typed.

// old
var Z w

type w []x
type x []z
type z int

// new
var Z w

type w []x
type x []z

// i z: changed from int to bool
type z bool

// old
type H struct{}

func (H) M() {}

// new
// i H: changed from struct{} to interface{M()}
type H interface {
	M()
}

//////////////// Splitting types

//// Old has one type, K; new has J and K.
// old
type J = K
type K int

// new
// i K: changed from K to K
type J K
type K int

//// OK: in both old and new, {J1, K1, L1} name the same type.
// old
type (
	J1 = K1
	K1 = L1
	L1 int
)

// new
type (
	J1 = K1
	K1 int
	L1 = J1
)

//////////////// Whole-package interface satisfaction

// old
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

// new
type WI1 interface {
	M1()
	m()
}

type WS1 int

func (WS1) M1() {}

// i WS1: no longer implements WI1
//func (WS1) m1() {}

type WI2 interface {
	M2()
	m2()
	// i WS2: no longer implements WI2
	m3()
}

type WS2 int

func (WS2) M2() {}
func (WS2) m2() {}
