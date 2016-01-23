// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package text lays out paragraphs of text.
//
// A body of text is laid out into a Frame: Frames contain Paragraphs (stacked
// vertically), Paragraphs contain Lines (stacked vertically), and Lines
// contain Boxes (stacked horizontally). Each Box holds a []byte slice of the
// text. For example, to simply print a Frame's text from start to finish:
//
//	var f *text.Frame = etc
//	for p := f.FirstParagraph(); p != nil; p = p.Next() {
//		for l := p.FirstLine(); l != nil; l = l.Next() {
//			for b := l.FirstBox(); b != nil; b = b.Next() {
//				fmt.Print(b.Text(f))
//			}
//		}
//	}
//
// A Frame's structure (the tree of Paragraphs, Lines and Boxes), and its
// []byte text, are not modified directly. Instead, a Frame's maximum width can
// be re-sized, and text can be added and removed via Carets (which implement
// standard io interfaces). For example, to add some words to the end of a
// frame:
//
//	var f *text.Frame = etc
//	c := f.NewCaret()
//	c.Seek(0, text.SeekEnd)
//	c.WriteString("Not with a bang but a whimper.\n")
//	c.Close()
//
// Either way, such modifications can cause re-layout, which can add or remove
// Paragraphs, Lines and Boxes. The underlying memory for such structs can be
// re-used, so pointer values, such as of type *Box, should not be held over
// such modifications.
package text

// These constants are equal to os.SEEK_SET, os.SEEK_CUR and os.SEEK_END,
// understood by the io.Seeker interface, and are provided so that users of
// this package don't have to explicitly import "os".
const (
	SeekSet int = 0
	SeekCur int = 1
	SeekEnd int = 2
)

// Frame holds paragraphs of text.
//
// The zero value is a valid Frame of empty text, which contains one Paragraph,
// which contains one Line, which contains one Box.
type Frame struct {
	first Paragraph

	// last points to the last Paragraph. nil implicitly means &first.
	//
	// Unlike a Paragraph's Lines or a Line's Rows, where the parent points to
	// the start but not the end of the linked list of its children, a Frame
	// explicitly tracks its last Paragraph. Typical Paragraphs have at most
	// tens of Lines, and likewise for Lines and Boxes, so it should be
	// practical to walk the (short) linked list forward from the start to the
	// end. A Frame might have hundreds or even thousands of Paragraphs,
	// though, so it can be worth explicitly tracking the last Paragraph.
	//
	// It is also relatively cheap in terms of memory: a single pointer (per
	// Frame). In comparison, if we wanted to explicitly track the last Box in
	// a Line, and a Frame held ten thousand Lines, then that would cost ten
	// thousand (redundant) pointers.
	last *Paragraph

	len  int
	text []byte
}

// FirstParagraph returns the first paragraph of this frame.
func (f *Frame) FirstParagraph() *Paragraph {
	return &f.first
}

// lastParagraph returns the last paragraph of this frame.
//
// It is not exported, as users of this package are not expected to call this
// directly. Nonetheless, it is useful when seeking a caret to the end of a
// frame.
func (f *Frame) lastParagraph() *Paragraph {
	if f.last != nil {
		return f.last
	}
	return &f.first
}

// Len returns the number of bytes in the frame's text.
func (f *Frame) Len() int {
	return f.len
}

// NewCaret returns a new caret at the start of this frame.
func (f *Frame) NewCaret() *Caret {
	panic("TODO")
}

// TODO: be able to set a frame's max width, and font face.

// Paragraph holds lines of text.
type Paragraph struct {
	first Line
	next  *Paragraph
	prev  *Paragraph
}

// FirstLine returns the first line of this paragraph.
func (p *Paragraph) FirstLine() *Line {
	return &p.first
}

// lastLine returns the last line of this paragraph.
//
// It is not exported, as users of this package are not expected to call this
// directly. Nonetheless, it is useful when seeking a caret to the end of a
// frame.
func (p *Paragraph) lastLine() *Line {
	l := &p.first
	for l.next != nil {
		l = l.next
	}
	return l
}

// Next returns the next paragraph after this one in the frame.
func (p *Paragraph) Next() *Paragraph {
	return p.next
}

// Prev returns the previous paragraph before this one in the frame.
func (p *Paragraph) Prev() *Paragraph {
	return p.prev
}

// Line holds boxes of text.
type Line struct {
	first Box
	next  *Line
	prev  *Line
}

// FirstBox returns the first box of this line.
func (l *Line) FirstBox() *Box {
	return &l.first
}

// lastBox returns the last box of this line.
//
// It is not exported, as users of this package are not expected to call this
// directly. Nonetheless, it is useful when seeking a caret to the end of a
// frame.
func (l *Line) lastBox() *Box {
	b := &l.first
	for b.next != nil {
		b = b.next
	}
	return b
}

// Next returns the next line after this one in the paragraph.
func (l *Line) Next() *Line {
	return l.next
}

// Prev returns the previous line before this one in the paragraph.
func (l *Line) Prev() *Line {
	return l.prev
}

// Box holds a contiguous run of text.
type Box struct {
	next *Box
	prev *Box

	// Frame.text[i:j] holds this box's text.
	i, j int
}

// Next returns the next box after this one in the line.
func (b *Box) Next() *Box {
	return b.next
}

// Prev returns the previous box before this one in the line.
func (b *Box) Prev() *Box {
	return b.prev
}

// Text returns the box's text. f is the Frame that contains the box.
func (b *Box) Text(f *Frame) []byte {
	return f.text[b.i:b.j:b.j]
}
