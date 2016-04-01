// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package widget provides graphical user interface widgets.
//
// TODO: give an overview and some example code.
package widget // import "golang.org/x/exp/shiny/widget"

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

// Arity describes the number of children a class of nodes can have.
type Arity uint8

const (
	Leaf      = Arity(0) // Leaf nodes have no children.
	Shell     = Arity(1) // Shell nodes have at most one child.
	Container = Arity(2) // Container nodes can have any number of children.
)

// Palette is a set of colors for a theme.
//
// The colors are expressed as *image.Uniform values so that they can be easily
// passed as the src argument to image/draw functions.
type Palette struct {
	// Light, Neutral and Dark are three color tones used to fill in widgets
	// such as buttons, menu bars and panels.
	Light   *image.Uniform
	Neutral *image.Uniform
	Dark    *image.Uniform

	// Accent is the color used to accentuate selections or suggestions.
	Accent *image.Uniform

	// Foreground is the color used for text, dividers and icons.
	Foreground *image.Uniform

	// Background is the color used behind large blocks of text. Short,
	// non-editable label text will typically be on the Neutral color.
	Background *image.Uniform
}

// Theme is a set of colors and font faces.
type Theme interface {
	// Palette returns the color palette for this theme.
	Palette() Palette

	// GetFace returns a font.Face for this theme. PutFace should be called
	// once a widget's measure, layout or paint is done with the font.Face
	// returned.
	//
	// Note that, in general, a font.Face is not safe for concurrent use by
	// multiple goroutines, as its methods may re-use implementation-specific
	// caches and mask image buffers.
	GetFace() font.Face
	PutFace(font.Face)

	// TODO: be able to ask for font face variants, such as bold or embiggened
	// versions for headings vs body text?
}

var (
	// DefaultPalette is the default theme's palette.
	DefaultPalette = Palette{
		Light:      &image.Uniform{C: color.RGBA{0xf5, 0xf5, 0xf5, 0xff}}, // Material Design "Grey 100".
		Neutral:    &image.Uniform{C: color.RGBA{0xee, 0xee, 0xee, 0xff}}, // Material Design "Grey 200".
		Dark:       &image.Uniform{C: color.RGBA{0xe0, 0xe0, 0xe0, 0xff}}, // Material Design "Grey 300".
		Accent:     &image.Uniform{C: color.RGBA{0x21, 0x96, 0xf3, 0xff}}, // Material Design "Blue 500".
		Foreground: &image.Uniform{C: color.RGBA{0x00, 0x00, 0x00, 0xff}}, // Material Design "Black".
		Background: &image.Uniform{C: color.RGBA{0xff, 0xff, 0xff, 0xff}}, // Material Design "White".
	}

	// DefaultTheme is a theme using the default palette and a basic font face.
	DefaultTheme Theme = defaultTheme{}
)

// Note that a basicfont.Face is stateless and safe to use concurrently, so
// defaultTheme.PutFace can be a no-op.

type defaultTheme struct{}

func (defaultTheme) Palette() Palette   { return DefaultPalette }
func (defaultTheme) GetFace() font.Face { return basicfont.Face7x13 }
func (defaultTheme) PutFace(font.Face)  {}

// Class is a class of nodes. For example, all button widgets would be Nodes
// whose Class values are a ButtonClass.
type Class interface {
	// Arity returns the number of children this class of nodes can have.
	Arity() Arity

	// Measure returns the natural size of a specific node (and its children)
	// of this class.
	Measure(n *Node, t *Theme) (size image.Point)

	// Layout lays out a specific node (and its children) of this class.
	//
	// TODO: specify how previous measurements and size constraints are passed
	// down the tree.
	Layout(n *Node, t *Theme)

	// Paint paints a specific node (and its children) of this class onto a
	// destination image.
	//
	// TODO: specify how previous layout is passed down the tree.
	Paint(n *Node, t *Theme, dst *image.RGBA)

	// TODO: add DPI to Measure/Layout/Paint, via the Theme or otherwise.
	// TODO: OnXxxEvent methods.
}

// LeafClassEmbed is designed to be embedded in struct types that implement the
// Class interface and have Leaf arity. It provides default implementations of
// the Class interface's methods.
type LeafClassEmbed struct{}

func (LeafClassEmbed) Arity() Arity                      { return Leaf }
func (LeafClassEmbed) Measure(*Node, *Theme) image.Point { return image.Point{} }
func (LeafClassEmbed) Layout(*Node, *Theme)              {}
func (LeafClassEmbed) Paint(*Node, *Theme, *image.RGBA)  {}

// TODO: ShellClassEmbed, ContainerClassEmbed.

// Node is an element of a widget tree.
//
// Every element of a widget tree is a node, but nodes can be of different
// classes. For example, a Flow node (i.e. one whose Class is FlowClass) can
// contain two Button nodes and an Image node.
type Node struct {
	// Parent, FirstChild, LastChild, PrevSibling and NextSibling describe the
	// widget tree structure.
	Parent, FirstChild, LastChild, PrevSibling, NextSibling *Node

	// Class is what class of node this is.
	Class Class

	// ClassData is class-specific data for this node. For example, a
	// ButtonClass may store an image and some text in this field. A
	// ProgressBarClass may store a numerical percentage.
	ClassData interface{}
}

// AppendChild adds a node c as a child of n.
//
// It will panic if c already has a parent or siblings.
func (n *Node) AppendChild(c *Node) {
	if c.Parent != nil || c.PrevSibling != nil || c.NextSibling != nil {
		panic("widget: AppendChild called for an attached child Node")
	}
	switch n.Class.Arity() {
	case Leaf:
		panic("widget: AppendChild called for a leaf parent Node")
	case Shell:
		if n.FirstChild != nil {
			panic("widget: AppendChild called for a shell parent Node that already has a child Node")
		}
	}
	last := n.LastChild
	if last != nil {
		last.NextSibling = c
	} else {
		n.FirstChild = c
	}
	n.LastChild = c
	c.Parent = n
	c.PrevSibling = last
}

// RemoveChild removes a node c that is a child of n. Afterwards, c will have
// no parent and no siblings.
//
// It will panic if c's parent is not n.
func (n *Node) RemoveChild(c *Node) {
	if c.Parent != n {
		panic("widget: RemoveChild called for a non-child Node")
	}
	if n.FirstChild == c {
		n.FirstChild = c.NextSibling
	}
	if c.NextSibling != nil {
		c.NextSibling.PrevSibling = c.PrevSibling
	}
	if n.LastChild == c {
		n.LastChild = c.PrevSibling
	}
	if c.PrevSibling != nil {
		c.PrevSibling.NextSibling = c.NextSibling
	}
	c.Parent = nil
	c.PrevSibling = nil
	c.NextSibling = nil
}
