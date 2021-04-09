// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package peg

import (
	"fmt"
	"io"
	"regexp"
)

type lookupExpression struct {
	name     string
	resolved int
}

type sequenceExpression []Expression
type choiceExpression []Expression
type zeroOrMoreExpression struct{ child Expression }
type oneOrMoreExpression struct{ child Expression }
type optionalExpression struct{ child Expression }
type expectExpression struct{ child Expression }
type andPredicateExpression struct{ child Expression }
type notPredicateExpression struct{ child Expression }
type captureExpression struct{ child Expression }
type eofExpression struct{}

type patternExpression struct {
	literal  bool
	raw      string
	compiled *regexp.Regexp
}

// Lookup returns an expression that invokes the named rule from the Grammar.
func Lookup(name string) Expression {
	return &lookupExpression{name: name, resolved: -1}
}

func (l *lookupExpression) Scan(state *State) (interface{}, error) {
	//indirect through the parser to a preparedRule using index
	return state.Parser.rules[l.resolved].Scan(state)
}

func (l *lookupExpression) Children() []Expression { return nil }

func (l *lookupExpression) Format(w fmt.State, _ rune) {
	w.Write([]byte(l.name))
}

// Sequence returns an expression that matches if the supplied expressions match
// in order.
func Sequence(expressions ...Expression) Expression {
	list := make([]Expression, 0, len(expressions))
	for _, child := range expressions {
		if child == nil {
			continue
		}
		if l, ok := child.(sequenceExpression); ok {
			list = append(list, l...)
		} else {
			list = append(list, child)
		}
	}
	switch len(list) {
	case 0:
		return nil
	case 1:
		return list[0]
	default:
		return sequenceExpression(list)
	}
}

func (s sequenceExpression) Scan(state *State) (interface{}, error) {
	var values []interface{}
	for _, e := range s {
		v, err := e.Scan(state)
		if err != nil {
			return values, err
		}
		values = appendResult(values, v)
	}
	return values, nil
}

func (s sequenceExpression) Children() []Expression {
	return s
}

func (s sequenceExpression) Format(w fmt.State, r rune) {
	for i, child := range s {
		if i > 0 {
			writeByte(w, ' ')
		}
		formatChild(w, s, child)
	}
}

// Choice returns an expression that matches any one of the supplied
// expressions. The expressions are checked in order, and the first one that
// matches is accepted.
func Choice(expressions ...Expression) Expression {
	list := make([]Expression, 0, len(expressions))
	for _, child := range expressions {
		if child == nil {
			continue
		}
		if l, ok := child.(choiceExpression); ok {
			list = append(list, l...)
		} else {
			list = append(list, child)
		}
	}
	switch len(list) {
	case 0:
		return nil
	case 1:
		return list[0]
	default:
		return choiceExpression(list)
	}
}

func (c choiceExpression) Scan(state *State) (interface{}, error) {
	for _, e := range c {
		pos := state.position()
		v, err := e.Scan(state)
		if err == nil || err != NotMatched {
			// was either a match, or a failure, either way stop here
			return v, err
		}
		state.seek(pos)
	}
	return nil, NotMatched
}

func (c choiceExpression) Children() []Expression {
	return c
}

func (c choiceExpression) Format(w fmt.State, r rune) {
	for i, child := range c {
		if i > 0 {
			w.Write([]byte(" / "))
		}
		formatChild(w, c, child)
	}
}

// ZeroOrMore returns an expression that matches any number of the provided
// expression. It consumes until the expression no longer matches.
func ZeroOrMore(expression Expression) Expression {
	switch expression := expression.(type) {
	case zeroOrMoreExpression:
		return expression
	case oneOrMoreExpression:
		return zeroOrMoreExpression(expression)
	case optionalExpression:
		return zeroOrMoreExpression(expression)
	default:
		return zeroOrMoreExpression{child: expression}
	}
}

func (z zeroOrMoreExpression) Scan(state *State) (interface{}, error) {
	return consumeAll(state, z.child, nil)
}

func (z zeroOrMoreExpression) Children() []Expression {
	return []Expression{z.child}
}

func (z zeroOrMoreExpression) Format(w fmt.State, r rune) {
	formatChild(w, z, z.child)
	writeByte(w, '*')
}

// OneOrMore  returns an expression that matches at least one of the provided
// expression. It consumes until the expression no longer matches.
func OneOrMore(expression Expression) Expression {
	switch expression := expression.(type) {
	case zeroOrMoreExpression:
		return expression
	case oneOrMoreExpression:
		return expression
	case optionalExpression:
		return zeroOrMoreExpression(expression)
	default:
		return oneOrMoreExpression{child: expression}
	}
}

func (o oneOrMoreExpression) Scan(state *State) (interface{}, error) {
	v, err := o.child.Scan(state)
	if err != nil {
		return nil, err
	}
	return consumeAll(state, o.child, appendResult(nil, v))
}

func (o oneOrMoreExpression) Children() []Expression {
	return []Expression{o.child}
}

func (o oneOrMoreExpression) Format(w fmt.State, r rune) {
	formatChild(w, o, o.child)
	writeByte(w, '+')
}

// Optional returns an expression that matches zero or one of the provided
// expression.
func Optional(expression Expression) Expression {
	switch expression := expression.(type) {
	case zeroOrMoreExpression:
		return expression
	case oneOrMoreExpression:
		return zeroOrMoreExpression(expression)
	case optionalExpression:
		return expression
	default:
		return optionalExpression{child: expression}
	}
}

func (o optionalExpression) Scan(state *State) (interface{}, error) {
	pos := state.position()
	v, err := o.child.Scan(state)
	if err == NotMatched {
		// a normal no match, so return an "empty" match
		state.seek(pos)
		return nil, nil
	}
	return v, err
}

func (o optionalExpression) Children() []Expression {
	return []Expression{o.child}
}

func (o optionalExpression) Format(w fmt.State, r rune) {
	formatChild(w, o, o.child)
	writeByte(w, '?')
}

// Expect returns an expression that fails if the provided expression did not
// match. This is used to provide points in the grammar that prevent
// backtracking, normally used in places where the match is certain, and having
// and expect node both reduces the time wasted on impossible matches and
// improves the quality of the error returned.
func Expect(expression Expression) Expression {
	return expectExpression{child: expression}
}

func (e expectExpression) Scan(state *State) (interface{}, error) {
	start := state.position()
	v, err := e.child.Scan(state)
	if err == NotMatched {
		err = &ExpectError{
			State:      state,
			Expression: e.child,
			Start:      start,
			End:        state.position(),
		}
	}
	if m, ok := state.Reader.(Marker); ok {
		m.Mark()
	}
	return v, err
}

func (e expectExpression) Children() []Expression {
	return []Expression{e.child}
}

func (e expectExpression) Format(w fmt.State, r rune) {
	writeByte(w, ':')
	formatChild(w, e, e.child)
}

// AndPredicate returns a non consuming expression that matches if the supplied
// expression matches.
func AndPredicate(expression Expression) Expression {
	switch expression := expression.(type) {
	case andPredicateExpression:
		return expression
	case notPredicateExpression:
		return expression
	default:
		return andPredicateExpression{child: expression}
	}
}

func (a andPredicateExpression) Scan(state *State) (interface{}, error) {
	pos := state.position()
	_, err := a.child.Scan(state)
	state.seek(pos)
	return nil, err
}

func (a andPredicateExpression) Children() []Expression {
	return []Expression{a.child}
}

func (a andPredicateExpression) Format(w fmt.State, r rune) {
	writeByte(w, '&')
	formatChild(w, a, a.child)
}

// NotPredicate returns a non consuming expression that matches if the supplied
// expression does not match.
func NotPredicate(expression Expression) Expression {
	switch expression := expression.(type) {
	case andPredicateExpression:
		return notPredicateExpression(expression)
	case notPredicateExpression:
		return andPredicateExpression(expression)
	default:
		return notPredicateExpression{child: expression}
	}
}

func (n notPredicateExpression) Scan(state *State) (interface{}, error) {
	pos := state.position()
	_, err := n.child.Scan(state)
	state.seek(pos)
	switch err {
	case nil:
		// a match, so return a no match instead
		return nil, NotMatched
	case NotMatched:
		// no match, so return an empty match instead
		return nil, nil
	default:
		return nil, err
	}
}

func (n notPredicateExpression) Children() []Expression {
	return []Expression{n.child}
}

func (n notPredicateExpression) Format(w fmt.State, r rune) {
	writeByte(w, '!')
	formatChild(w, n, n.child)
}

// EOF is an expression that matches if there is no more input.
var EOF Expression = eofExpression{}

func (a eofExpression) Scan(state *State) (interface{}, error) {
	start := state.position()
	_, _, err := state.Reader.ReadRune()
	if err != io.EOF {
		state.seek(start)
		return nil, NotMatched
	}
	return nil, nil
}

func (a eofExpression) Children() []Expression { return nil }

func (a eofExpression) Format(w fmt.State, r rune) {
	io.WriteString(w, `$`)
}

func Literal(s string) Expression {
	p, err := pattern(s, true)
	if err != nil {
		panic(err)
	}
	return p
}

func Pattern(s string) (Expression, error) {
	p, err := pattern(s, false)
	return p, err
}

func mustPattern(s string) Expression {
	p, err := pattern(s, false)
	if err != nil {
		panic(err)
	}
	return p
}

func pattern(s string, literal bool) (*patternExpression, error) {
	p := s
	if literal {
		p = regexp.QuoteMeta(s)
	}
	r, err := regexp.Compile("^" + p)
	if err != nil {
		return nil, err
	}
	r.Longest()
	return &patternExpression{literal: literal, raw: s, compiled: r}, nil
}

func (p *patternExpression) Scan(state *State) (interface{}, error) {
	pos := state.position()
	match := p.compiled.FindReaderSubmatchIndex(state.Reader)
	if len(match) < 2 || match[0] == match[1] {
		// no match found
		return nil, NotMatched
	}
	var result []interface{}
	if len(match) > 2 {
		// match with capture, build results
		count := (len(match) / 2) - 1
		result = make([]interface{}, count)
		for i := range result {
			start := match[i*2+2]
			end := match[i*2+3]
			var err error
			result[i], err = state.readString(pos+int64(start), end-start)
			if err != nil {
				return nil, err
			}
		}
	}
	state.seek(pos + int64(match[1]))
	return result, nil
}

func (p *patternExpression) Children() []Expression { return nil }

func (p *patternExpression) Format(w fmt.State, r rune) {
	if p.literal {
		fmt.Fprintf(w, `"%s"`, p.raw)
	} else {
		fmt.Fprintf(w, `'%s'`, p.raw)
	}
}

func consumeAll(state *State, expression Expression, values []interface{}) (interface{}, error) {
	for {
		pos := state.position()
		v, err := expression.Scan(state)
		if err != nil {
			if err == NotMatched {
				// no match, return the results found so far, without the no match.
				state.seek(pos)
				return values, nil
			}
			// a failure, so return it.
			return values, err
		}
		to := state.position()
		if pos == to {
			return values, fmt.Errorf("match without progress in %v at %v", expression, state.debugPrefix())
		}
		// a normal match, add it and continue.
		values = appendResult(values, v)
	}
}

func appendResult(values []interface{}, value interface{}) []interface{} {
	switch value := value.(type) {
	case nil:
		return values
	case []interface{}:
		return append(values, value...)
	default:
		return append(values, value)
	}
}

func formatChild(w fmt.State, parent, child Expression) {
	f := "%v"
	switch child.(type) {
	case sequenceExpression:
		if _, isChoice := parent.(choiceExpression); !isChoice {
			f = "(%v)"
		}
	case choiceExpression:
		f = "(%v)"
	}
	fmt.Fprintf(w, f, child)
}

func writeByte(w io.Writer, b byte) {
	var buf [1]byte
	buf[0] = b
	w.Write(buf[:])
}
