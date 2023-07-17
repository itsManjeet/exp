// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rand

import "unsafe"

const chaChaBufLen = 32

type ChaCha8Source struct {
	idx uint
	buf [chaChaBufLen]uint64
	key [8]uint32
}

//go:noescape
func xorKeyStreamVX(dst, src *[32]uint64, key *[8]uint32, nonce *[3]uint32, counter *uint32)

func (s *ChaCha8Source) Uint64() uint64 {
	if idx := s.idx; idx >= chaChaBufLen {
		return s.buffer()
	} else {
		s.idx++
		return s.buf[idx]
	}
}

func (s *ChaCha8Source) buffer() uint64 {
	var nonce [3]uint32
	var counter uint32
	xorKeyStreamVX(&s.buf, &s.buf, &s.key, &nonce, &counter)
	s.idx = uint(copy(s.buf[:], (*[4]uint64)(unsafe.Pointer(&s.key))[:]))

	out := s.buf[s.idx]
	s.idx++
	return out
}

func (s *ChaCha8Source) Seed(uint64) { panic("unimplemented") }
