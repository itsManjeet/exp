// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·BGRA(SB),NOSPLIT,$0-24
	MOVQ	p+0(FP), SI
	MOVQ	len+8(FP), DI

	// Do nothing if the length isn't a multiple of 4.
	MOVQ	DI, AX
	ANDQ	$3, AX
	CMPQ	AX, $0
	JNE	done

	// Make the shuffle control mask (16-byte register X0) look like this,
	// where the low order byte comes first:
	//
	// 02 01 00 03  06 05 04 07  0a 09 08 0b  0e 0d 0c 0f
	//
	// Load the bottom 8 bytes into X0, the top into X1, then interleave them
	// into X0.
	MOVQ	$0x0704050603000102, AX
	MOVQ	AX, X0
	MOVQ	$0x0f0c0d0e0b08090a, AX
	MOVQ	AX, X1
	PUNPCKLQDQ	X1, X0

	// Store the original end point (p + len) in BX.
	MOVQ	DI, BX
	ADDQ	SI, BX

	// Handle 16-byte blocks. Store loop16's end point (p + (len&^3)) in DI.
	ANDQ	$0xfffffffffffffff0, DI
	ADDQ	SI, DI
loop16:
	CMPQ	SI, DI
	JEQ	done16

	MOVOU	(SI), X1
	PSHUFB	X0, X1
	MOVOU	X1, (SI)

	ADDQ	$16, SI
	JMP	loop16
done16:

	// Handle any trailing 4-byte blocks between DI and BX.
	MOVQ	DI, SI
	MOVQ	BX, DI
loop:
	CMPQ	SI, DI
	JEQ	done

	MOVB	0(SI), AX
	MOVB	2(SI), BX
	MOVB	BX, 0(SI)
	MOVB	AX, 2(SI)

	ADDQ	$4, SI
	JMP	loop
done:
	RET
