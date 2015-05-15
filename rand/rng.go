// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

// PCGSource is an implementation of a 64-bit permuted congruential
// generator as defined in
//
// 	PCG: A Family of Simple Fast Space-Efficient Statistically Good
// 	Algorithms for Random Number Generation
// 	Melissa E. O’Neill, Harvey Mudd College
// 	http://www.pcg-random.org/pdf/toms-oneill-pcg-family-v1.02.pdf
//
// The generator here is the congruential generator PCG RXS M XS 64
// as found in the software available at http://www.pcg-random.org/.
//
// It is a 64-bit generator with 64 bits of state, so it is represented
// by a single word. It is compact and efficient but not as secure as
// a generator with more internal state.
type PCGSource struct {
	state uint64
}

// Seed uses the provided seed value to initialize the generator to a deterministic state.
func (pcg *PCGSource) Seed(seed uint64) {
	pcg.state = seed
}

const (
	multiplier = 6364136223846793005
	increment  = 1442695040888963407
	permuter   = 12605985483714917081
)

// Uint64 returns a pseudo-random 64-bit unsigned integer as a uint64.
func (pcg *PCGSource) Uint64() uint64 {
	oldstate := pcg.state
	pcg.state = pcg.state*multiplier + increment
	word := ((oldstate >> ((oldstate >> 59) + 5)) ^ oldstate) * permuter
	return (word >> 43) ^ word
}
