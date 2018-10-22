// Single-line comments beginning with an "i" are expected messages for incompatible changes.
// Those beginning with a "c" are for compatible changes.

package new

import (
	"io"
	"math"
)

type (
	A int // same as old

	Y = A //c Y: added

	B bool //c B: added

	u int //i u: changed from string to int

	u2 int // rename of old u1, no other change

	u3 int

	// This causes u1 to correspond to u2
	Split1 = u2

	// This tries to make u1 correspond to u3
	Split2 = u3 //i Split2: changed from u1 to u3

	// Here both old u2 and old u3 correspond to new u3.
	GoodMerge1 = u3
	GoodMerge2 = u3

	BadMerge1 = u3
	BadMerge2 = u3 //i u4.M: removed
)

const (
	C1     = "1" //i C1: changed from untyped int to untyped string
	C2     = -1  //i C2: changed from int to untyped int
	C3 int = 4   //i C3: changed from untyped int to int
	V8 int = 1   //i V8: changed from var to const
	C4 u2  = 1
	C5     = false  //i C5: changed from bool to untyped bool
	C6     = 10     //i C6: changed from uint to untyped int
	C7     = "7"    //i C7: changed from string to untyped string
	C8     = 3.1    //i C8: changed from float64 to untyped float
	C9     = 1 + 2i //i C9: changed from complex64 to untyped complex

	// representability
	Cr1 = -1                            //i Cr1: can no longer represent uint8
	Cr2 = math.MaxInt8 + 1              //i Cr2: can no longer represent int8
	Cr3 = math.MaxFloat32 * 2           //i Cr3: can no longer represent float32
	Cr4 = complex(0, math.MaxFloat32*2) //i Cr4: can no longer represent complex64

	// value changes that break conversions (unimplemented)
	//Cc1 = 1.1    //----i Cc1: float constant no longer an integer value
	//Cc2 = 1 + 1i //----i Cc2: complex constant no longer a float value
)

var (
	V1 []string //i V1: changed from string to []string
	V2 u
	V3 A
	V4 u
	V5 u2
	V7 chan int //i V7: changed from <-chan int to chan int

	V9  interface{} //i V9: changed from interface{M()} to interface{}
	V10 interface { //i V10: changed from interface{M()} to interface{M(); M2()}
		M2()
		M()
	}
	V11 interface{ M(int) } //i V11: changed from interface{M()} to interface{M(int)}

	VS1 struct{ B, A int }    //i VS1: changed from struct{A int; B int} to struct{B int; A int}
	VS2 struct{ A int }       //i VS2: changed from struct{A int; B int} to struct{A int}
	VS3 struct{ A, B, C int } //i VS3: changed from struct{A int; B int} to struct{A int; B int; C int}
	VS4 struct {
		A int
		u2
	}
)

type (
	A1 [1]int
	A2 [C3]int //i A2: changed from [3]int to [4]int
	A3 [2]bool //i A3: changed from [2]int to [2]bool

	Sl []string //i Sl: changed from []int to []string

	P1 **bool //i P1: changed from *int to **bool
	P2 *u2

	M1 map[string]int
	M2 map[int]int       //i M2: changed from map[string]int to map[int]int
	M3 map[string]string //i M3: changed from map[string]int to map[string]string

	Ch1 chan bool  //i Ch1, element type: changed from int to bool
	Ch2 chan<- int //i Ch2: changed direction
	Ch3 <-chan int //i Ch3: changed direction
	Ch4 chan int   //c Ch4: removed direction

	I1 interface {
		M2(int)
		M3()
		m()
	}
	//i I1.M1: removed
	//i I1.M2: changed from func() to func(int)
	//i I1.M3: added
	//i I1.m: added unexported method

	I2 interface {
		M1()
		// m() Removing an unexported method is OK.
		m2() // OK, because old already had an unexported method
		M2() //c I2.M2: added
	}

	I3 interface {
		M()
		Read([]byte) (int, error)
	}

	// I4 = io.Writer // OK, because I4 was distinct before and now it isn't (unimplemented)

	I5 interface {
		Write([]byte) (int, error)
	}
	// OK: I5 is distinct from io.Writer in both.

	I6 io.Writer
	//i I6: changed from io.Writer to I6
	// e.g. var f func(io.Writer) = func(pkg.I6) {}
)

func F1(c int, d string) map[u2]Y { return nil }
func F2(int) bool                 { return true } //i F2: changed from func(int) to func(int) bool
func F3(int, int)                 {}              //i F3: changed from func(int) to func(int, int)
func F4(bool) int                 { return 0 }    //i F4: changed from func(int) int to func(bool) int
func F5(int) string               { return "" }   //i F5: changed from func(int) int to func(int) string
func F6(...int)                   {}              //i F6: changed from func(int) to func(...int)
func F7(a interface{ x() })       {}              //i F7: changed from func(interface{}) to func(interface{x()})

var F8 func(bool) //c F8: changed from func to var

type S1 = s1

type s1 struct {
	C chan int //i S1.C: changed from bool to chan int
	A int
	//i S1.B: removed
	x []int //i S1: old is comparable, new is not
	d float32
	E bool //c S1.E: added
}

type embedx struct {
	E string
}

type S2 struct {
	embedx // fine: the unexported embedded field changed names, but the exported field does not
	A      int
}

type embed struct{ F int }

type S3 struct {
	//i S3.E: removed
	//c S3.F: added
	embed
	A int
}

type F int

type S4 struct {
	// OK: removed unexported fields
	// D and F marked as added because they are now part of the immediate fields
	D   bool //c S4.D: added
	E   int  // OK: same as in old
	F   F    //c S4.F: added
	A1       // OK: same
	*S4      // OK: same (recursive embedding)

}

// Difference between exported field set and exported shape.
type S5 struct {
	A int
}

// Exported fields: A int, S5 S5.
// Exported literal keys: S5
//i S6.A: removed
type S6 struct {
	S5
}

// Method sets.

type SM struct {
	embedm2
	embedm3
	Embedm //i SM.A: changed from int to bool
}

type SMa = SM //c SMa: added

func (SM) V1() {} // OK: same
/* func (SM) V2() {} */ //i SM.V2: removed
func (SM) V3(int)       {} //i SM.V3: changed from func() to func(int)
func (SM) V5()          {} //c SM.V5: added
func (SM) v(int)        {} // OK: unexported method change
func (SM) v2()          {} // OK: unexported method added

func (*SM) P1() {} // OK: same
/* func (*SM) P2() {} */ //i (*SM).P2: removed
func (*SMa) P3(int)      {} //i (*SM).P3: changed from func() to func(int)
func (*SM) P5()          {} //c (*SM).P5: added
/* func (*SM) p() {} */ // OK: unexported method removed

// Changing from a value to a pointer receiver or vice versa
// just looks like adding and removing a method.

//i SM.V4: removed
//i (*SM).V4: changed from func() to func(int)
func (*SM) V4(int) {}

//c SM.P4: added
// P4 is not removed from (*SM) because value methods
// are in the pointer method set.
func (SM) P4() {}

type embedm2 int

func (embedm2) EV1(int) {} //i embedm.EV1: changed from func() to func(int)

//i embedm.EV2, method set of SM: removed
//i embedm.EV2, method set of *SM: removed

//i (*embedm).EP2, method set of *SM: removed
func (*embedm2) EP1() {}

type embedm3 int

func (embedm3) EV3()  {} // OK: compatible with old embedm.EV3
func (*embedm3) EP3() {} // OK: compatible with old (*embedm).EP3

type Embedm struct {
	A bool //i Embedm.A: changed from int to bool
}

//i Embedm.FV: changed from func() to func(int)
func (Embedm) FV(int) {}
func (*Embedm) FP()   {}

type RepeatEmbedm struct {
	Embedm //i RepeatEmbedm.A: changed from int to bool
}

var Z w

type w []x
type x []z
type z bool //i z: changed from int to bool

//i H: changed from struct{} to interface{M()}
type H interface {
	M()
}

//type Rem int //i Rem: removed

// The whole-package interface satisfaction test.

type WI1 interface {
	M1()
	m()
}

type WS1 int

func (WS1) M1() {}

//func (WS1) m1() {} //i WS1: no longer implements WI1

type WI2 interface {
	M2()
	m2()
	m3() //i WS2: no longer implements WI2
}

type WS2 int

func (WS2) M2() {}
func (WS2) m2() {}
