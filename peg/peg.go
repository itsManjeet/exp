// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/* Package peg provides a recursive desencent parsing engine and a PEG parser.

For regular grammars, you can build them from PEG syntax using NewGrammar.
You can modify the Grammar as needed at this point, and then build a Parser for
that grammar using NewParser.
The parser can add processors that handle matching nodes to produce the parser
output.
Once you have a parser, you can use it to parse an Input. The recursive descent
engine does not proscribe the types of input it can be used on, and the package
provides a few common types, including both rune based and token based inputs.

The formal grammar of the PEG syntax is self described in the Language constant.
*/
package peg

import (
	"fmt"
	"io"
	"strings"
)

// Reader allows a grammars to operate on various types of inputs.
type Reader interface {
	// Rune scanner allows stepping through runes, as used by the regular
	// expression library
	io.RuneReader
	// Seeker allows jumping around the input, needed for backtracking.
	io.Seeker
}

// Marker
type Marker interface {
	Mark()
}

// Parser is a complete recursive descent parser.
// It processes input using the supplied grammar.
type Parser struct {
	// Grammar is the syntax being parsed by this parser.
	// This should not be modified after the first call to Prepare or Parse.
	Grammar Grammar

	// If trace is not nil, the parser will print progress information to the
	// supplied stream as it attempts to parse an input. Used when debugging your
	// grammar.
	Trace bool

	// If Root is set, it is the name of the rule to use as the root of the parse
	// tree.
	Root string

	processors []namedProcessor
	rules      []preparedRule
}

// Grammar holds a compiled grammar ready to use.
type Grammar []Rule

// Rule is a named expression in a Grammar.
type Rule struct {
	// The Name of this rule in the grammar.
	Name string
	// The Expression that is evaluated for this rule.
	Expression Expression
	// The comment associated with this rule.
	Comment string
}

// Expression represents a node in the grammar expression tree.
type Expression interface {
	// Scan is called to match a node from the input.
	// It is called with the parser state.
	// The input will be left at the furthest point read even in a failed match.
	// Returns the value that represents the match, which depends on the nodes
	// processed.
	// If the node does not match but in a way that allows the parser to continue
	// and try alternatives, it returns NotMatched for its error.
	Scan(state *State) (interface{}, error)

	// Children returns the set of child expressions, if there are any.
	// This is most commonly used when walking the entire expression tree.
	Children() []Expression
}

// NotMatched is the error type returned when a node could not match
// against the input.
var NotMatched error = matchError{}

// Processor is a function that is invoked for rules during the parse.
// This is the primary way of transforming the output results of the parse,
// and can also be used to modify the errors produced.
type Processor func(args ...interface{}) (interface{}, error)

// State is passed into expression match calls during parsing.
type State struct {
	// Parser is the owner of this parse state.
	Parser *Parser
	// Name is the name of the input used in error messages.
	Name string
	// Reader is the source this parser is running on.
	Reader Reader

	// depth is used when printing the parse trace
	depth int
}

// ExpectError is produced when an expected expression is not found during
// parsing, as denoted by a $ in the PEG.
type ExpectError struct {
	// The State being processed when this error was generated.
	State *State
	// The Expression that did not match when it should have done.
	Expression Expression
	// The start and end offsets into the input for the failed match.
	Start, End int64
}

// namedProcessor is a name and processing function pair, used because we need
// an ordered map of Processors.
type namedProcessor struct {
	name    string
	process Processor
}

// preparedRule is a rule that has been fully bound into a parser.
// Primarily this is an optimization to avoid having multiple name keyed maps.
type preparedRule struct {
	index      int
	name       string
	expression Expression
	process    Processor
}

// matchError is the unique type for NotMatched.
type matchError struct{}

func (matchError) Error() string { return "no match" }

func (err *ExpectError) Error() string {
	err.State.seek(err.Start)
	return fmt.Sprintf("%s:%d:%d: expect %v got %q",
		err.State.Name, err.Start, err.End, err.Expression,
		err.State.debugPrefix())
}

// Rule returns the rule that matches the name, if there is one.
func (g Grammar) Rule(name string) *Rule {
	for i := range g {
		if g[i].Name == name {
			return &g[i]
		}
	}
	return nil
}

// Scan allows Grammar to also implements Expression.
// It uses the expression of the zeroth rule by default.
func (g Grammar) Scan(state *State) (interface{}, error) {
	return g[0].Expression.Scan(state)
}

// Children returns all the rules expressions, used when walking the entire
// Grammar.
func (g Grammar) Children() []Expression {
	c := make([]Expression, 0, len(g))
	for _, r := range g {
		if r.Expression == nil {
			continue
		}
		c = append(c, r.Expression)
	}
	return c
}

// Format attempts to pretty print a grammar in standard PEG format such that
// it could be rebuilt from the output.
// This is best effort only and intended mostly for human readability or tests,
// not guaranteed.
func (g Grammar) Format(w fmt.State, r rune) {
	// work out the rule width so we can align them
	const padding = "                                                           "
	width := 0
	for _, rule := range g {
		if width < len(rule.Name) {
			width = len(rule.Name)
		}
	}
	if width > len(padding) {
		width = len(padding)
	}
	for i, rule := range g {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if rule.Name != "" {
			// we don't use the rule native formatting because we want to do alignment
			fmt.Fprint(w, rule.Name)
			align := width - len(rule.Name)
			if align > 0 {
				fmt.Fprint(w, padding[:align])
			}
			fmt.Fprintf(w, " <- %v", rule.Expression)
		}
		if rule.Comment != "" {
			fmt.Fprintf(w, "#%s", rule.Comment)
		}
	}
}

// NewParser builds a new parser.
func NewParser(g Grammar) *Parser {
	return &Parser{Grammar: g}
}

// Prepare readies a parser for use.
// It is called automatically by Parse, but can also be used to check a parser
// for errors earlier.
// Once Prepare has been called it is an error to modify the Grammar or
// processing functions in any way.
func (p *Parser) Prepare() error {
	if p.rules != nil {
		// already prepared
		return nil
	}
	if len(p.Grammar) == 0 {
		return fmt.Errorf("grammar has no rules")
	}
	// build all the rules
	p.rules = make([]preparedRule, 0, len(p.Grammar))
	for _, r := range p.Grammar {
		if r.Name == "" {
			continue
		}
		p.rules = append(p.rules, preparedRule{
			index:      len(p.rules),
			name:       r.Name,
			expression: r.Expression,
		})
	}
	// and now link any lookup expressions to the right index
	if err := Walk(p.Grammar, func(e Expression) error {
		if l, ok := e.(*lookupExpression); ok {
			rule := p.lookup(l.name)
			if rule == nil {
				return fmt.Errorf("rule %q not declared", l.name)
			}
			if l.resolved < 0 {
				// although this modifies the original rule, it does so that should be consistent
				// for a given grammar across all parsers, so it is safe
				l.resolved = rule.index
			} else if l.resolved != rule.index {
				return fmt.Errorf("rule %q chaned index from %d to %d", l.name, l.resolved, rule.index)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// and now bind any processors to their correct rules
	for _, entry := range p.processors {
		rule := p.lookup(entry.name)
		if rule == nil {
			return fmt.Errorf("no rule %q for processor", entry.name)
		}
		rule.process = entry.process
	}
	return nil
}

// lookup is used to find a prepared rule by name.
// This is used when binding a Lookup node to an rule to run.
func (p *Parser) lookup(name string) *preparedRule {
	for i := range p.rules {
		if p.rules[i].name == name {
			return &p.rules[i]
		}
	}
	return nil
}

// Process adds a node Processor to the parser.
// The name must match an existing rule when Prepare is called.
// The supplied processor will be invoked whenever the matching rule is used,
// whether it matches or not.
func (p *Parser) Process(name string, f Processor) {
	if p.rules != nil {
		panic("peg.Parser.Process called after peg.Parser.Prepare")
	}
	p.processors = append(p.processors, namedProcessor{name: name, process: f})
}

// Parse processes an input using the grammar.
// It uses the supplied root as the name of the first rule to process, if root
// is the empty string it uses the first rule of the grammar.
func (p *Parser) Parse(name string, r Reader) (interface{}, error) {
	if err := p.Prepare(); err != nil {
		return nil, err
	}
	state := State{
		Parser: p,
		Name:   name,
		Reader: r,
	}
	var rule *preparedRule
	if p.Root == "" {
		rule = &p.rules[0]
	} else {
		rule = p.lookup(p.Root)
		if rule == nil {
			return nil, fmt.Errorf("invalid root node %q", p.Root)
		}
	}
	value, err := rule.Scan(&state)
	if err != nil {
		//TODO: build a nicer error message here?
		return nil, err
	}
	return value, err
}

func (r *preparedRule) Scan(state *State) (interface{}, error) {
	state.depth++
	defer func() { state.depth-- }()
	if r.expression == nil {
		return nil, fmt.Errorf("rule %q not defined", r.name)
	}
	state.debugTrace(">", r.name)
	value, err := r.expression.Scan(state)
	if err == nil && r.process != nil {
		switch v := value.(type) {
		case []interface{}:
			value, err = r.process(v...)
		default:
			value, err = r.process(v)
		}
	}
	state.debugTrace("<", r.name)
	return value, err
}

// Walk can be used to walk an Expression tree.
// It invokes the callback for each Expression before recursing into its
// children.
func Walk(root Expression, callback func(e Expression) error) error {
	if err := callback(root); err != nil {
		return err
	}
	for _, c := range root.Children() {
		if err := Walk(c, callback); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) position() int64 {
	pos, err := s.Reader.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}
	return pos
}

func (s *State) seek(pos int64) {
	_, err := s.Reader.Seek(pos, io.SeekStart)
	if err != nil {
		panic(err)
	}
}

func (s *State) readString(at int64, size int) (string, error) {
	pos := s.position()
	defer s.seek(pos)
	s.seek(at)
	switch r := s.Reader.(type) {
	case io.Reader:
		b := make([]byte, size)
		if n, err := io.ReadFull(r, b); err != nil {
			return string(b[:n]), err
		}
		return string(b), nil
	default:
		// the slow path, get the runes one by one
		s := strings.Builder{}
		for size > 0 {
			v, w, err := r.ReadRune()
			if err != nil {
				return s.String(), err
			}
			size -= w
			if _, err = s.WriteRune(v); err != nil {
				return s.String(), err
			}
		}
		return s.String(), nil
	}
}

func (s *State) debugPrefix() string {
	var size = 20
	pos := s.position()
	defer s.seek(pos)
	switch r := s.Reader.(type) {
	case io.Reader:
		b := make([]byte, size)
		n, _ := r.Read(b)
		return string(b[:n])
	default:
		// the slow path, get the runes one by one
		s := strings.Builder{}
		for size > 0 {
			v, w, err := r.ReadRune()
			size -= w
			if err != nil {
				break
			}
			s.WriteRune(v)
		}
		return s.String()
	}
}

func (s *State) debugTrace(direction string, name string) {
	const indent = "......................................................................"
	if !s.Parser.Trace {
		return
	}
	fmt.Print(indent[:s.depth])
	fmt.Print(direction)
	fmt.Print(name)
	fmt.Printf(" [%d]", s.position())
	p := s.debugPrefix()
	if p != "" {
		fmt.Printf(" %q", p)
	}
	fmt.Println()
}

func (r *Rule) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "%s <- %v", r.Name, r.Expression)
}
