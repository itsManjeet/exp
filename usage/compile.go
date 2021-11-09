// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"fmt"
	"io"
	"strings"
)

type grammar struct {
	sections []*section
	literals []*literal
	values   []*value
	flags    []*flags
	usage    expression
}

type expression interface{ isExpression() }

type section struct {
	title string
	root  expression
}

// literal is an expression that matches a constant string.
// If the literal is the same as a section title, it matches one of the set of
// expressions from that section instead.
// If it is the program name, it matches anything.
type literal struct {
	name  string
	group *section
}

// value is an expression that matches an arbitrary positional argument and
// assigns it to a named value.
type value struct {
	name string
}

// flags is a flag declaration expression.
// It has a set of known flag aliases and the parameter name.
// It may take multiple flagDeclaration statements from multiple pages to know the
// full set of aliases, this happens at the binding level.
type flags struct {
	aliases  []*flagName
	param    *parameter
	default_ string
}

type flagName struct {
	name  string
	flags *flags
}

type parameter struct {
	name  string
	flags *flags
}

type sequence []expression

type choice []expression

type optional struct {
	expression expression
}

type repeat struct {
	expression expression
}

type literalChoice []literal

// String is used to provide a human name for the flag set.
func (f *flags) String() string { return f.aliases[0].name }

// resolveError indicates an error in resolving.
type resolveError string

func (*section) isExpression()  {}
func (sequence) isExpression()  {}
func (choice) isExpression()    {}
func (*optional) isExpression() {}
func (*repeat) isExpression()   {}
func (*value) isExpression()    {}
func (*literal) isExpression()  {}
func (*flags) isExpression()    {}

func compile(help []Page) (*grammar, error) {
	g, err := parseHelp(help)
	if err != nil {
		return nil, err
	}
	// now resolve all the flag declarations and section references
	if err := g.resolveAll(); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *grammar) findSection(name string) *section {
	for _, s := range g.sections {
		if strings.EqualFold(s.title, name) {
			return s
		}
	}
	return nil
}

func (g *grammar) addValue(v *value) *value {
	for _, test := range g.values {
		if strings.EqualFold(test.name, v.name) {
			return test
		}
	}
	g.values = append(g.values, v)
	return v
}

func (g *grammar) addLiteral(l *literal) *literal {
	for _, test := range g.literals {
		if strings.EqualFold(test.name, l.name) {
			return test
		}
	}
	g.literals = append(g.literals, l)
	return l
}

func (g *grammar) addFlags(f *flags) {
	g.flags = append(g.flags, f)
}

func (g *grammar) resolveAll() (err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case resolveError:
			err = r
		default:
			fmt.Printf("err=%T\n", r)
			panic(err)
		}
	}()
	// resolve all the named entities
	for _, s := range g.sections {
		s.root = g.resolve(s.root)
		detectLiteralGrouping(s)
	}
	// pick out the "usage" section
	s := g.findSection("usage")
	if s == nil {
		return resolveErrorf("no usage")
	}
	g.usage = s.root
	return nil
}

func (g *grammar) resolve(expr expression) expression {
	switch expr := expr.(type) {
	case sequence:
		for i := range expr {
			expr[i] = g.resolve(expr[i])
		}
		if len(expr) == 1 {
			return expr[0]
		}
		return expr
	case choice:
		for i := range expr {
			expr[i] = g.resolve(expr[i])
		}
		if len(expr) == 1 {
			return expr[0]
		}
		return expr
	case *optional:
		expr.expression = g.resolve(expr.expression)
		return expr
	case *repeat:
		expr.expression = g.resolve(expr.expression)
		return expr
	case *value:
		return g.addValue(expr)
	case *literal:
		if s := g.findSection(expr.name); s != nil {
			return s
		}
		return g.addLiteral(expr)
	case *flags:
		if f := g.findFlags(expr.aliases); f != nil {
			// existing flag set, merge the information and substitute
			g.merge(f, expr)
			return f
		}
		// new flagset
		g.addFlags(expr)
		return expr
	default:
		panic(resolveErrorf("unknown expression type %T", expr))
	}
}

func (g *grammar) merge(f *flags, other *flags) {
	for _, n := range other.aliases {
		g.mergeName(f, n)
	}
	if other.param != nil {
		if f.param != nil {
			if f.param.name != other.param.name {
				panic(resolveErrorf("conflicting parameter name %q and %q", f.param.name, other.param.name))
			}
		} else {
			f.param = other.param
		}
	}
	if other.default_ != "" {
		if f.default_ != "" {
			if other.default_ != f.default_ {
				panic(resolveErrorf("conflicting default value %q and %q", f.default_, other.default_))
			}
		} else {
			f.default_ = other.default_
		}
	}
}

func (g *grammar) mergeName(f *flags, name *flagName) {
	for _, n := range f.aliases {
		if n.name == name.name {
			return
		}
	}
	//new name, add it
	f.aliases = append(f.aliases, name)
	name.flags = f
}

func (g *grammar) findFlags(names []*flagName) *flags {
	for _, a := range names {
		alias := a.name
		for _, f := range g.flags {
			for _, n := range f.aliases {
				// flag names use exact match, not equalfold
				if alias == n.name {
					// matched an existing flag exactly
					return n.flags
				}
			}
		}
	}
	// no match
	return nil
}

func (v *value) Format(w fmt.State, r rune) {
	fmt.Fprintf(w, "%q", v.name)
}

func (l *literal) Format(w fmt.State, r rune) {
	io.WriteString(w, "<")
	io.WriteString(w, l.name)
	io.WriteString(w, ">")
}

func detectLiteralGrouping(s *section) {
	choices, ok := s.root.(choice)
	if !ok {
		return
	}
	for _, c := range choices {
		if _, ok := c.(*literal); !ok {
			return
		}
	}
	for _, c := range choices {
		l := c.(*literal)
		l.group = s
	}
}

func resolveErrorf(msg string, args ...interface{}) resolveError {
	return resolveError(fmt.Sprintf(msg, args...))
}

func (err resolveError) Error() string { return string(err) }
