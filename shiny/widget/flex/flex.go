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
	"fmt"
	"image"
	"math"

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
		if d, ok := c.LayoutData.(LayoutData); ok {
			_ = d
			// TODO Measure
		}
	}
}

func (k *Flex) Layout(t *theme.Theme) {
	var children []element
	for c := k.FirstChild; c != nil; c = c.NextSibling {
		children = append(children, element{
			flexBaseSize: float64(k.flexBaseSize(c)),
			n:            c,
		})
	}

	containerMainSize := float64(k.mainSize(k.Rect.Size()))
	containerCrossSize := float64(k.crossSize(k.Rect.Size()))

	// §9.3.5 collect children into flex lines
	var lines []flexLine
	if k.Wrap == NoWrap {
		line := flexLine{child: make([]*element, len(children))}
		for i := range children {
			child := &children[i]
			line.child[i] = child
			line.mainSize += child.flexBaseSize
		}
		lines = []flexLine{line}
	} else {
		var line flexLine

		for i := range children {
			child := &children[i]
			if line.mainSize > 0 && line.mainSize+child.flexBaseSize > containerMainSize {
				lines = append(lines, line)
				line = flexLine{}
			}
			line.child = append(line.child, child)
			line.mainSize += child.flexBaseSize

			if d, ok := child.n.LayoutData.(LayoutData); ok && d.BreakAfter {
				lines = append(lines, line)
				line = flexLine{}
			}
		}
		if len(line.child) > 0 {
			lines = append(lines, line)
		}

		if k.Wrap == WrapReverse {
			for i := 0; i < len(lines)/2; i++ {
				lines[i], lines[len(lines)-i-1] = lines[len(lines)-i-1], lines[i]
			}
		}
	}

	// §9.3.6 resolve flexible lengths (details in section §9.7)
	for lineNum := range lines {
		line := &lines[lineNum]
		grow := line.mainSize < containerMainSize // §9.7.1

		// §9.7.2 freeze inflexible children.
		for _, child := range line.child {
			mainSize := k.mainSize(child.n.MeasuredSize)
			if grow {
				if growFactor(child.n) == 0 || k.flexBaseSize(child.n) > mainSize {
					child.frozen = true
					child.mainSize = float64(mainSize)
				}
			} else {
				if shrinkFactor(child.n) == 0 || k.flexBaseSize(child.n) < mainSize {
					child.frozen = true
					child.mainSize = float64(mainSize)
				}
			}
		}

		// §9.7.3 calculate initial free space
		initFreeSpace := float64(k.mainSize(k.Rect.Size()))
		for _, child := range line.child {
			if child.frozen {
				initFreeSpace -= child.mainSize
			} else {
				initFreeSpace -= float64(k.flexBaseSize(child.n))
			}
		}

		// §9.7.4 flex loop
		for {
			// Check for flexible items.
			allFrozen := true
			for _, child := range line.child {
				if !child.frozen {
					allFrozen = false
					break
				}
			}
			if allFrozen {
				break
			}

			// Calculate remaining free space.
			remFreeSpace := float64(k.mainSize(k.Rect.Size()))
			unfrozenFlexFactor := 0.0
			for _, child := range line.child {
				if child.frozen {
					remFreeSpace -= child.mainSize
				} else {
					remFreeSpace -= float64(k.flexBaseSize(child.n))
					if grow {
						unfrozenFlexFactor += growFactor(child.n)
					} else {
						unfrozenFlexFactor += shrinkFactor(child.n)
					}
				}
			}
			if unfrozenFlexFactor < 1 {
				p := initFreeSpace * unfrozenFlexFactor
				if math.Abs(p) < math.Abs(remFreeSpace) {
					remFreeSpace = p
				}
			}

			// Distribute free space proportional to flex factors.
			if grow {
				for _, child := range line.child {
					if child.frozen {
						continue
					}
					r := growFactor(child.n) / unfrozenFlexFactor
					child.mainSize = float64(k.flexBaseSize(child.n)) + r*remFreeSpace
				}
			} else {
				sumScaledShrinkFactor := 0.0
				for _, child := range line.child {
					if child.frozen {
						continue
					}
					scaledShrinkFactor := float64(k.flexBaseSize(child.n)) * shrinkFactor(child.n)
					sumScaledShrinkFactor += scaledShrinkFactor
				}
				for _, child := range line.child {
					if child.frozen {
						continue
					}
					scaledShrinkFactor := float64(k.flexBaseSize(child.n)) * shrinkFactor(child.n)
					r := float64(scaledShrinkFactor) / sumScaledShrinkFactor
					child.mainSize = float64(k.flexBaseSize(child.n)) - r*math.Abs(float64(remFreeSpace))
				}
			}

			// Fix min/max violations.
			sumClampDiff := 0.0
			for _, child := range line.child {
				// TODO: we work in whole pixels but flex calculations are done in
				// fractional pixels. Take this oppertunity to clamp us to whole
				// pixels and make sure we sum correctly.
				if child.frozen {
					continue
				}
				child.unclamped = child.mainSize
				if d, ok := child.n.LayoutData.(LayoutData); ok {
					minSize := float64(k.mainSize(d.MinSize))
					if minSize > child.mainSize {
						child.mainSize = minSize
					} else if d.MaxSize != nil {
						maxSize := float64(k.mainSize(*d.MaxSize))
						if child.mainSize > maxSize {
							child.mainSize = maxSize
						}
					}
				}
				if child.mainSize < 0 {
					child.mainSize = 0
				}
				sumClampDiff += child.mainSize - child.unclamped
			}

			// Freeze over-flexed items.
			switch {
			case sumClampDiff == 0:
				for _, child := range line.child {
					child.frozen = true
				}
			case sumClampDiff > 0:
				for _, child := range line.child {
					if child.mainSize > child.unclamped {
						child.frozen = true
					}
				}
			case sumClampDiff < 0:
				for _, child := range line.child {
					if child.mainSize < child.unclamped {
						child.frozen = true
					}
				}
			}
		}

		// §9.7.5 set main size
		// At this point, child.mainSize is right.
	}

	// §9.4 determine cross size
	// §9.4.7 calculate hypothetical cross size of each element
	for lineNum := range lines {
		for _, child := range lines[lineNum].child {
			child.crossSize = float64(k.crossSize(child.n.MeasuredSize))
			if child.mainSize < float64(k.mainSize(child.n.MeasuredSize)) {
				if r, ok := aspectRatio(child.n); ok {
					child.crossSize = child.mainSize / r
				}
			}
			if d, ok := child.n.LayoutData.(LayoutData); ok {
				minSize := float64(k.crossSize(d.MinSize))
				if minSize > child.crossSize {
					child.crossSize = minSize
				} else if d.MaxSize != nil {
					maxSize := float64(k.crossSize(*d.MaxSize))
					if child.crossSize > maxSize {
						child.crossSize = maxSize
					}
				}
			}
		}
	}
	if len(lines) == 1 {
		// §9.4.8 single line
		switch k.Direction {
		case Row, RowReverse:
			lines[0].crossSize = float64(k.Rect.Size().Y)
		case Column, ColumnReverse:
			lines[0].crossSize = float64(k.Rect.Size().X)
		}
	} else {
		// §9.4.8 multi-line
		for lineNum := range lines {
			line := &lines[lineNum]
			// TODO §9.4.8.1, no concept of inline-axis yet
			max := 0.0
			for _, child := range line.child {
				if child.crossSize > max {
					max = child.crossSize
				}
			}
			line.crossSize = max
		}
	}
	off := 0.0
	for lineNum := range lines {
		line := &lines[lineNum]
		line.crossOffset = off
		off += line.crossSize
	}
	// §9.4.9 align-content: stretch
	remCrossSize := containerCrossSize - off
	if k.AlignContent == AlignContentStretch && remCrossSize > 0 {
		add := remCrossSize / float64(len(lines))
		for lineNum := range lines {
			line := &lines[lineNum]
			line.crossOffset += float64(lineNum) * add
			line.crossSize += add
		}
	}
	// Note: no equiv. to §9.4.10 "visibility: collapse".
	// §9.4.11 align-item: stretch
	for lineNum := range lines {
		line := &lines[lineNum]
		for _, child := range line.child {
			align := k.alignItem(child.n)
			if align == AlignItemStretch && child.crossSize < line.crossSize {
				child.crossSize = line.crossSize
			}
		}
	}

	// §9.5 main axis alignment
	for lineNum := range lines {
		line := &lines[lineNum]
		total := 0.0
		for _, child := range line.child {
			total += child.mainSize
		}

		// TODO: alternate values of k.Justify
		off := 0.0
		for _, child := range line.child {
			child.mainOffset = off
			off += child.mainSize
		}
	}

	// TODO
	// §9.6 cross axis alignment

	// TODO: RowReverse, ColumnReverse

	// Layout complete. Generate child Rect values.
	for lineNum := range lines {
		line := &lines[lineNum]
		for _, child := range line.child {
			switch k.Direction {
			case Row, RowReverse:
				child.n.Rect.Min.X = int(math.Ceil(child.mainOffset))
				child.n.Rect.Max.X = int(math.Ceil(child.mainOffset + child.mainSize))
				child.n.Rect.Min.Y = int(math.Ceil(child.crossOffset))
				child.n.Rect.Max.Y = int(math.Ceil(child.crossOffset + child.crossSize))
			case Column, ColumnReverse:
				child.n.Rect.Min.Y = int(math.Ceil(child.mainOffset))
				child.n.Rect.Max.Y = int(math.Ceil(child.mainOffset + child.mainSize))
				child.n.Rect.Min.X = int(math.Ceil(child.crossOffset))
				child.n.Rect.Max.X = int(math.Ceil(child.crossOffset + child.crossSize))
			default:
				panic(fmt.Sprint("flex: bad direction ", k.Direction))
			}
		}
	}
}

type element struct {
	n            *node.Embed
	flexBaseSize float64
	frozen       bool
	unclamped    float64
	mainSize     float64
	mainOffset   float64
	crossSize    float64
	crossOffset  float64
}

type flexLine struct {
	mainSize    float64
	crossSize   float64
	crossOffset float64
	child       []*element
}

func (k *Flex) alignItem(n *node.Embed) AlignItem {
	align := k.AlignItem
	if d, ok := n.LayoutData.(LayoutData); ok {
		align = d.Align
	}
	return align
}

// flexBaseSize calculates flex base size as per §9.2.3
func (k *Flex) flexBaseSize(n *node.Embed) int {
	basis := Auto
	if d, ok := n.LayoutData.(LayoutData); ok {
		basis = d.Basis
	}
	switch basis {
	case Definite: // A
		return n.LayoutData.(LayoutData).BasisPx
	case Content:
		// TODO §9.2.3.B
		// TODO §9.2.3.C
		// TODO §9.2.3.D
		panic("flex: flex-basis: content not supported")
	case Auto: // E
		return k.mainSize(n.MeasuredSize)
	default:
		panic(fmt.Sprintf("flex: unknown flex-basis %v", basis))
	}
}

func growFactor(n *node.Embed) float64 {
	if d, ok := n.LayoutData.(LayoutData); ok {
		return d.Grow
	}
	return 0
}

func shrinkFactor(n *node.Embed) float64 {
	if d, ok := n.LayoutData.(LayoutData); ok && d.Shrink != nil {
		return *d.Shrink
	}
	return 1
}

func aspectRatio(n *node.Embed) (ratio float64, ok bool) {
	// TODO: source a formal description of "intrinsic aspect ratio"
	d, ok := n.LayoutData.(LayoutData)
	if ok && d.MinSize.X != 0 && d.MinSize.Y != 0 {
		return float64(d.MinSize.X) / float64(d.MinSize.Y), true
	}
	return 0, false
}

func (k *Flex) mainSize(p image.Point) int {
	switch k.Direction {
	case Row, RowReverse:
		return p.X
	case Column, ColumnReverse:
		return p.Y
	default:
		panic(fmt.Sprint("flex: bad direction ", k.Direction))
	}
}

func (k *Flex) crossSize(p image.Point) int {
	switch k.Direction {
	case Row, RowReverse:
		return p.Y
	case Column, ColumnReverse:
		return p.X
	default:
		panic(fmt.Sprint("flex: bad direction ", k.Direction))
	}
}
