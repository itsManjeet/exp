// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package flex provides a container widget that lays out its children
// following the CSS flexbox algorithm.
//
// As the shiny widget model does not provide all of the layout features
// of CSS, the flex package diverges in several ways. There is no item
// inline-axis, no item margins or padding to be accounted for, and the
// container size provided by the outer widget is taken as gospel and
// never expanded.
package flex

import (
	"image"

	"golang.org/x/exp/shiny/widget/node"
	"golang.org/x/exp/shiny/widget/theme"
)

// Flex is a container widget that lays out its children following the
// CSS flexbox algorithm.
type Flex struct {
	node.ContainerEmbed

	Direction    Direction
	Wrap         FlexWrap
	Justify      Justify
	AlignItem    AlignItem
	AlignContent AlignContent
}

// NewFlex returns a new Flex widget.
func NewFlex() *Flex {
	fl := new(Flex)
	fl.Wrapper = fl
	return fl
}

// Direction is the direction in which flex items are laid out.
//
// https://www.w3.org/TR/css-flexbox-1/#flex-direction-propertyy
type Direction uint8

// Possible values of Direction.
const (
	Row Direction = iota
	RowReverse
	Column
	ColumnReverse
)

// FlexWrap controls whether the container is single- or multi-line,
// and the direction in which the lines are laid out.
//
// https://www.w3.org/TR/css-flexbox-1/#flex-wrap-property
type FlexWrap uint8

// Possible values of FlexWrap.
const (
	NoWrap FlexWrap = iota
	Wrap
	WrapReverse
)

// Justify aligns items along the main axis.
//
// It is the 'justify-content' property.
//
// https://www.w3.org/TR/css-flexbox-1/#justify-content-property
type Justify uint8

// Possible values of Justify.
const (
	JustifyStart        Justify = iota // pack to start of line
	JustifyEnd                         // pack to end of line
	JustifyCenter                      // pack to center of line
	JustifySpaceBetween                // even spacing
	JustifySpaceAround                 // even spacing, half-size on each end
)

// AlignItem aligns items along the cross axis.
//
// It is the 'align-items' property when applied to a Flex container,
// and the 'align-self' property when applied to an item in LayoutData.
//
// https://www.w3.org/TR/css-flexbox-1/#align-items-property
// http://www.w3.org/TR/css-flexbox-1/#propdef-align-self
type AlignItem uint8

// Possible values of AlignItem.
const (
	AlignItemAuto AlignItem = iota
	AlignItemStart
	AlignItemEnd
	AlignItemCenter
	AlignItemBaseline // TODO requires introducing inline-axis concept
	AlignItemStretch
)

// AlignContent is the 'align-content' property.
// It aligns container lines when there is extra space on the cross-axis.
//
// https://www.w3.org/TR/css-flexbox-1/#align-content-property
type AlignContent uint8

// Possible values of AlignContent.
const (
	AlignContentStretch AlignContent = iota
	AlignContentStart
	AlignContentEnd
	AlignContentCenter
	AlignContentSpaceBetween
	AlignContentSpaceAround
)

// Basis sets the base size of a flex item.
//
// A default basis of Auto means the flex container uses the
// MeasuredSize of an item. Otherwise a Definite Basis will
// override the MeasuredSize with BasisPx.
//
// TODO: do we (or will we )have a useful notion of Content in the
// widget layout model that is separate from MeasuredSize? If not,
// we could consider completely removing this concept from this
// flex implementation.
type Basis uint8

// Possible values of Basis.
const (
	Auto    Basis = iota
	Content       // TODO
	Definite
)

// LayoutData is the Node.LayoutData type for a Flex's children.
type LayoutData struct {
	MinSize image.Point
	MaxSize *image.Point

	// Grow is the flex grow factor which determines how much a Node
	// will grow relative to its siblings.
	Grow float64

	// Shrink is the flex shrink factor which determines how much a Node
	// will shrink relative to its siblings. If nil, a default shrink
	// factor of 1 is used.
	Shrink *float64

	// Basis determines the initial main size of the of the Node.
	// If set to Definite, the value stored in BasisPx is used.
	Basis   Basis
	BasisPx int // TODO use unit package?

	Align AlignItem

	// BreakAfter forces the next node onto the next flex line.
	BreakAfter bool
}

func (k *Flex) Measure(t *theme.Theme) {
	// As Measure is a bottom-up calculation of natural size, we have no
	// hint yet as to how we should flex. So we ignore Wrap, Justify,
	// AlignItem, AlignContent.
	for c := k.FirstChild; c != nil; c = c.NextSibling {
		c.Wrapper.Measure(t)
		// TODO Measure
	}
}

func (k *Flex) Layout(t *theme.Theme) {
	c := k.FirstChild
	if c != nil {
		c.Rect = k.Rect
	}
	// TODO: implement algorithm
}
