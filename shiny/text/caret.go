// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text

// Caret is a location in a frame's text, and is the mechanism for adding and
// removing bytes of text. Conceptually, a caret and a frame's text is like an
// int c and a []byte t such that the text before and after that caret is t[:c]
// and t[c:]. That byte-count location remains unchanged even when a frame is
// re-sized and laid out into a new tree of paragraphs, lines and boxes.
//
// Multiple carets for the one frame are not safe to use concurrently, but it
// is valid to interleave such operations sequentially. For example, if two
// carets c0 and c1 for the one frame are positioned at the 10th and 20th byte,
// and 4 bytes are written to c0, inserting what becomes the equivalent of
// text[10:14], then c1's position is updated to be the 24th byte.
type Caret struct {
	// TODO: implement.
}

// TODO: many Caret methods.
