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

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// Arity is the number of children a class of nodes can have.
type Arity uint8

const (
	Leaf      = Arity(0) // Leaf nodes have no children.
	Shell     = Arity(1) // Shell nodes have at most one child.
	Container = Arity(2) // Container nodes can have any number of children.
)

// Axis is zero, one or both of the horizontal and vertical axes. For example,
// a widget may be scrollable in one of the four AxisXxx values.
type Axis uint8

const (
	AxisNone       = Axis(0)
	AxisHorizontal = Axis(1)
	AxisVertical   = Axis(2)
	AxisBoth       = Axis(3) // AxisBoth equals AxisHorizontal | AxisVertical.
)

// Class is a class of nodes. For example, all button widgets would be Nodes
// whose Class values have the buttonClass type.
type Class interface {
	// Arity returns the number of children a specific node can have.
	Arity(n *Node) Arity

	// Measure sets n.MeasuredSize to the natural size, in pixels, of a
	// specific node (and its children) of this class.
	Measure(n *Node, t *Theme)

	// Layout lays out a specific node (and its children) of this class,
	// setting the Node.Rect fields of each child. The n.Rect field should have
	// previously been set during the parent node's layout.
	Layout(n *Node, t *Theme)

	// Paint paints a specific node (and its children) of this class onto a
	// destination image. origin is the parent widget's origin with respect to
	// the dst image's origin; n.Rect.Add(origin) will be n's position and size
	// in dst's coordinate space.
	//
	// TODO: add a clip rectangle? Or rely on the RGBA.SubImage method to pass
	// smaller dst images?
	Paint(n *Node, t *Theme, dst *image.RGBA, origin image.Point)

	// TODO: OnXxxEvent methods.
}

// LeafClassEmbed is designed to be embedded in struct types that implement the
// Class interface and have Leaf arity. It provides default implementations of
// the Class interface's methods.
type LeafClassEmbed struct{}

func (LeafClassEmbed) Arity(n *Node) Arity { return Leaf }

func (LeafClassEmbed) Measure(n *Node, t *Theme) { n.MeasuredSize = image.Point{} }

func (LeafClassEmbed) Layout(n *Node, t *Theme) {}

func (LeafClassEmbed) Paint(n *Node, t *Theme, dst *image.RGBA, origin image.Point) {}

// ShellClassEmbed is designed to be embedded in struct types that implement
// the Class interface and have Shell arity. It provides default
// implementations of the Class interface's methods.
type ShellClassEmbed struct{}

func (ShellClassEmbed) Arity(n *Node) Arity { return Shell }

func (ShellClassEmbed) Measure(n *Node, t *Theme) {
	if c := n.FirstChild; c != nil {
		c.Measure(t)
		n.MeasuredSize = c.MeasuredSize
	} else {
		n.MeasuredSize = image.Point{}
	}
}

func (ShellClassEmbed) Layout(n *Node, t *Theme) {
	if c := n.FirstChild; c != nil {
		c.Rect = n.Rect.Sub(n.Rect.Min)
		c.Layout(t)
	}
}

func (ShellClassEmbed) Paint(n *Node, t *Theme, dst *image.RGBA, origin image.Point) {
	if c := n.FirstChild; c != nil {
		c.Paint(t, dst, origin.Add(n.Rect.Min))
	}
}

// ContainerClassEmbed is designed to be embedded in struct types that
// implement the Class interface and have Container arity. It provides default
// implementations of the Class interface's methods.
type ContainerClassEmbed struct{}

func (ContainerClassEmbed) Arity(n *Node) Arity { return Container }

func (ContainerClassEmbed) Measure(n *Node, t *Theme) {
	mSize := image.Point{}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		c.Measure(t)
		if mSize.X < c.MeasuredSize.X {
			mSize.X = c.MeasuredSize.X
		}
		if mSize.Y < c.MeasuredSize.Y {
			mSize.Y = c.MeasuredSize.Y
		}
	}
	n.MeasuredSize = mSize
}

func (ContainerClassEmbed) Layout(n *Node, t *Theme) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		c.Rect = image.Rectangle{Max: c.MeasuredSize}
		c.Layout(t)
	}
}

func (ContainerClassEmbed) Paint(n *Node, t *Theme, dst *image.RGBA, origin image.Point) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		c.Paint(t, dst, origin.Add(n.Rect.Min))
	}
}

// NodeWrapper wraps a Node. It is typically implemented by struct types that
// embed the Node type.
type NodeWrapper interface {
	WrappedNode() *Node
}

// Node is an element of a widget tree.
//
// Every element of a widget tree is a node, but nodes can be of different
// classes. For example, a Flow node (i.e. one whose Class is FlowClass) can
// contain two Button nodes and an Image node.
type Node struct {
	// Parent, FirstChild, LastChild, PrevSibling and NextSibling describe the
	// widget tree structure.
	//
	// These fields are exported to enable walking the node tree, but they
	// should not be modified directly. Instead, call the AppendChild and
	// RemoveChild methods, which keeps the tree structure consistent.
	Parent, FirstChild, LastChild, PrevSibling, NextSibling *Node

	// Class is class-specific data and behavior for this node. For example, a
	// buttonClass-typed value may store an image and some text in this field.
	// A progressBarClass-typed value may store a numerical percentage.
	// Different class types would paint their nodes differently.
	Class Class

	// LayoutData is layout-specific data for this node. Its type is determined
	// by its parent node's class. For example, each child of a Flow may hold a
	// FlowLayoutData in this field.
	LayoutData interface{}

	// TODO: add commentary about the Measure / Layout / Paint model, and about
	// the lifetime of the MeasuredSize and Rect fields, and when user code can
	// access and/or modify them. At some point a new cycle begins, a call to
	// measure is necessary, and using MeasuredSize is incorrect (unless you're
	// trying to recall something about the past).

	// MeasuredSize is the widget's natural size, in pixels, as calculated by
	// the most recent Class.Measure call.
	MeasuredSize image.Point

	// Rect is the widget's position and actual (as opposed to natural) size,
	// in pixels, as calculated by the most recent Class.Layout call on its
	// parent node. A parent may lay out a child at a size different to its
	// natural size in order to satisfy a layout constraint, such as a row of
	// buttons expanding to fill a panel's width.
	//
	// The position (Rectangle.Min) is relative to its parent node. This is not
	// necessarily the same as relative to the screen's, window's or image
	// buffer's origin.
	Rect image.Rectangle
}

// WrappedNode returns the node itself. This method makes struct types that
// embed the Node type automatically implement the NodeWrapper interface, so
// they can be passed to AppendChild and RemoveChild.
func (n *Node) WrappedNode() *Node {
	return n
}

// Arity calls n.Class.Arity with n as its first argument.
func (n *Node) Arity() Arity {
	return n.Class.Arity(n)
}

// Measure calls n.Class.Measure with n as its first argument.
func (n *Node) Measure(t *Theme) {
	n.Class.Measure(n, t)
}

// Layout calls n.Class.Layout with n as its first argument.
func (n *Node) Layout(t *Theme) {
	n.Class.Layout(n, t)
}

// Paint calls n.Class.Paint with n as its first argument.
func (n *Node) Paint(t *Theme, dst *image.RGBA, origin image.Point) {
	n.Class.Paint(n, t, dst, origin)
}

// AppendChild adds a node c as a child of n.
//
// It will panic if c already has a parent or siblings.
func (n *Node) AppendChild(c NodeWrapper) {
	m := c.WrappedNode()
	if m.Parent != nil || m.PrevSibling != nil || m.NextSibling != nil {
		panic("widget: AppendChild called for an attached child Node")
	}
	switch n.Arity() {
	case Leaf:
		panic("widget: AppendChild called for a leaf parent Node")
	case Shell:
		if n.FirstChild != nil {
			panic("widget: AppendChild called for a shell parent Node that already has a child Node")
		}
	}
	last := n.LastChild
	if last != nil {
		last.NextSibling = m
	} else {
		n.FirstChild = m
	}
	n.LastChild = m
	m.Parent = n
	m.PrevSibling = last
}

// RemoveChild removes a node c that is a child of n. Afterwards, c will have
// no parent and no siblings.
//
// It will panic if c's parent is not n.
func (n *Node) RemoveChild(c NodeWrapper) {
	m := c.WrappedNode()
	if m.Parent != n {
		panic("widget: RemoveChild called for a non-child Node")
	}
	if n.FirstChild == m {
		n.FirstChild = m.NextSibling
	}
	if m.NextSibling != nil {
		m.NextSibling.PrevSibling = m.PrevSibling
	}
	if n.LastChild == m {
		n.LastChild = m.PrevSibling
	}
	if m.PrevSibling != nil {
		m.PrevSibling.NextSibling = m.NextSibling
	}
	m.Parent = nil
	m.PrevSibling = nil
	m.NextSibling = nil
}
