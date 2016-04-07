// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package widget provides graphical user interface widgets.
//
// TODO: give an overview and some example code.
package widget // import "golang.org/x/exp/shiny/widget"

import (
	"image"
)

// Arity describes the number of children a class of nodes can have.
type Arity uint8

const (
	Leaf      = Arity(0) // Leaf nodes have no children.
	Shell     = Arity(1) // Shell nodes have at most one child.
	Container = Arity(2) // Container nodes can have any number of children.
)

// Class is a class of nodes. For example, all button widgets would be Nodes
// whose Class values are a ButtonClass.
type Class interface {
	// Arity returns the number of children this class of nodes can have.
	Arity() Arity

	// Measure returns the natural size of a specific node (and its children)
	// of this class.
	Measure(n *Node, t Theme) (size image.Point)

	// Layout lays out a specific node (and its children) of this class.
	//
	// TODO: specify how previous measurements and size constraints are passed
	// down the tree.
	Layout(n *Node, t Theme)

	// Paint paints a specific node (and its children) of this class onto a
	// destination image.
	//
	// TODO: specify how previous layout is passed down the tree.
	Paint(n *Node, t Theme, dst *image.RGBA)

	// TODO: add DPI to Measure/Layout/Paint, via the Theme or otherwise.
	// TODO: OnXxxEvent methods.
}

// LeafClassEmbed is designed to be embedded in struct types that implement the
// Class interface and have Leaf arity. It provides default implementations of
// the Class interface's methods.
type LeafClassEmbed struct{}

func (LeafClassEmbed) Arity() Arity                     { return Leaf }
func (LeafClassEmbed) Measure(*Node, Theme) image.Point { return image.Point{} }
func (LeafClassEmbed) Layout(*Node, Theme)              {}
func (LeafClassEmbed) Paint(*Node, Theme, *image.RGBA)  {}

// ShellClassEmbed is designed to be embedded in struct types that implement
// the Class interface and have Shell arity. It provides default
// implementations of the Class interface's methods.
type ShellClassEmbed struct{}

func (ShellClassEmbed) Arity() Arity { return Shell }

func (ShellClassEmbed) Measure(n *Node, t Theme) image.Point {
	if c := n.FirstChild; c != nil {
		return c.Class.Measure(c, t)
	}
	return image.Point{}
}

func (ShellClassEmbed) Layout(n *Node, t Theme) {
	if c := n.FirstChild; c != nil {
		c.Class.Layout(c, t)
	}
}

func (ShellClassEmbed) Paint(n *Node, t Theme, dst *image.RGBA) {
	if c := n.FirstChild; c != nil {
		c.Class.Paint(c, t, dst)
	}
}

// ContainerClassEmbed is designed to be embedded in struct types that
// implement the Class interface and have Container arity. It provides default
// implementations of the Class interface's methods.
type ContainerClassEmbed struct{}

func (ContainerClassEmbed) Arity() Arity { return Container }

func (ContainerClassEmbed) Measure(n *Node, t Theme) (ret image.Point) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p := c.Class.Measure(c, t)
		if ret.X < p.X {
			ret.X = p.X
		}
		if ret.Y < p.Y {
			ret.Y = p.Y
		}
	}
	return ret
}

func (ContainerClassEmbed) Layout(n *Node, t Theme) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		c.Class.Layout(c, t)
	}
}

func (ContainerClassEmbed) Paint(n *Node, t Theme, dst *image.RGBA) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		c.Class.Paint(c, t, dst)
	}
}

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
