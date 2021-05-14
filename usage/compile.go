// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"

	"golang.org/x/exp/peg"
)

//go:embed usage.peg
var grammar string

// title is a section title expression
type title name

// literal is an expression that matches a constant string.
// If the literal is the same as a section title, it matches one of the set of
// expressions from that section instead.
// If it is the program name, it matches anything.
type literal struct {
	name    name
	child   peg.Expression
	binding *binding
}

// value is an expression that matches an arbitrary positional argument and
// assigns it to a named value.
type value struct {
	name    name
	binding *binding
}

// flagDecl is a flag declaration expression.
// It has a set of known flag aliases and the parameter name.
// It may take multiple flagDecl statements from multiple pages to know the
// full set of aliases, this happens at the binding level.
type flagDecl struct {
	aliases  []name
	param    name
	binding  *binding
	defaults []string
}

// defaultValue is extracted from comments and specifies the default value of
// a flag that is not present but is allowed.
type defaultValue string

// flagSet is a set of flag declarations.
// It is an implied construct, being resolved from a section directly
// containing flag declarations.
// It matches if any of it's flags match, and allows all of its flags to match.
type flagSet []peg.Expression

var (
	pegOnce   sync.Once
	pegParser *peg.Parser
	pegError  error
)

// language returns the syntax parser for the usage text langauge
func language() (*peg.Parser, error) {
	pegOnce.Do(func() {
		g, err := peg.NewGrammar(`USAGE`, grammar)
		if err != nil {
			pegError = err
			return
		}
		pegParser = peg.NewParser(g)
		pegParser.Process("Name", func(args ...interface{}) (interface{}, error) {
			return toName(args[0].(string)), nil
		})
		pegParser.Process("Title", func(args ...interface{}) (interface{}, error) {
			return title(args[0].(name)), nil
		})
		pegParser.Process("Named", func(args ...interface{}) (interface{}, error) {
			return &value{name: args[0].(name)}, nil
		})
		pegParser.Process("Literal", func(args ...interface{}) (interface{}, error) {
			return &literal{name: args[0].(name)}, nil
		})
		pegParser.Process("Flag", func(args ...interface{}) (interface{}, error) {
			var f *flagDecl
			//if we have a flag value pick it off the end of args
			if p, ok := args[len(args)-1].(*flagDecl); ok {
				f = p
				args = args[:len(args)-1]
			} else {
				f = &flagDecl{}
			}
			for _, v := range args {
				f.aliases = append(f.aliases, v.(name))
			}
			return f, nil
		})
		pegParser.Process("FlagParameter", func(args ...interface{}) (interface{}, error) {
			return &flagDecl{param: args[0].(name)}, nil
		})
		pegParser.Process("Default", func(args ...interface{}) (interface{}, error) {
			return defaultValue(args[0].(string)), nil
		})
		pegParser.Process("Sequence", func(args ...interface{}) (interface{}, error) {
			exprs := make([]peg.Expression, len(args))
			for i, v := range args {
				exprs[i] = v.(peg.Expression)
			}
			return peg.Sequence(exprs...), nil
		})
		pegParser.Process("Choice", func(args ...interface{}) (interface{}, error) {
			exprs := make([]peg.Expression, len(args))
			for i, v := range args {
				exprs[i] = v.(peg.Expression)
			}
			return choice(exprs), nil
		})
		pegParser.Process("Optional", func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				return nil, nil
			}
			return optional(args[0].(peg.Expression)), nil
		})
		pegParser.Process("Repeat", func(args ...interface{}) (interface{}, error) {
			if len(args) == 0 {
				// empty repeat, can happen with constructs like ()...
				return nil, nil
			}
			return peg.ZeroOrMore(args[0].(peg.Expression)), nil
		})
	})
	return pegParser, pegError
}

// parse applies the usage text langauge parser to the supplied help text pages.
func parse(h *Text) ([][]interface{}, error) {
	language, err := language()
	if err != nil {
		return nil, err
	}
	result := make([][]interface{}, len(h.Pages))
	for i, page := range h.Pages {
		results, err := language.Parse(page.Name, bytes.NewReader(page.Content))
		if err != nil {
			return nil, err
		}
		result[i] = results.([]interface{})
	}
	return result, nil
}

// compile parses the usage text, and complies the result down to a command line
// parser for the patterns it describes.
func compile(state *state) error {
	// first parse the usage text
	parsed, err := parse(&state.Text)
	if err != nil {
		return err
	}
	// unify the sections
	unified := []section{}
	for _, page := range parsed {
		// start with the default section
		var sectionName name
		var lastFlag *flagDecl
		var sec *section
		for _, line := range page {
			switch e := line.(type) {
			case title:
				sectionName = name(e)
				lastFlag = nil
				sec = nil
			case defaultValue:
				if lastFlag == nil {
					return fmt.Errorf("default value %q with no flag to bind to", e)
				}
				lastFlag.defaults = append(lastFlag.defaults, string(e))
			case peg.Expression:
				if !sectionName.IsValid() {
					continue
				}
				if sec == nil {
					for i, s := range unified {
						if s.title.Equivalent(sectionName) {
							sec = &unified[i]
						}
					}
					if sec == nil {
						unified = append(unified, section{title: sectionName})
						sec = &unified[len(unified)-1]
					}
				}
				sec.expressions = append(sec.expressions, e)
				peg.Walk(e, func(e peg.Expression) error {
					if f, ok := e.(*flagDecl); ok {
						lastFlag = f
					}
					return nil
				})
			default:
				return fmt.Errorf("invalid line type %T", line)
			}
		}
	}
	//build a grammar from the resolved sections
	grammar := make(peg.Grammar, len(unified))
	for i, section := range unified {
		grammar[i] = peg.Rule{
			Name:       section.title.match,
			Expression: choice(section.expressions),
		}
	}
	state.Parser.Grammar = grammar
	if state.Parser.Grammar.Rule("usage") == nil {
		return fmt.Errorf("no usage")
	}
	// now gather all the expressions that need bindings
	if err := peg.Walk(state.Parser.Grammar, func(e peg.Expression) error {
		switch e := e.(type) {
		case *literal:
			// decide what kind of literal we are, program, string or section
			if grammar.Rule(e.name.match) != nil {
				e.child = peg.Lookup(e.name.match)
			} else {
				e.binding = state.bind(binding{values: []name{e.name}})
			}
		case *value:
			e.binding = state.bind(binding{values: []name{e.name}})
		case *flagDecl:
			e.binding = state.bind(binding{
				flags:    e.aliases,
				params:   []name{e.param},
				defaults: e.defaults,
			})
		}
		return nil
	}); err != nil {
		return err
	}
	// finalize the parser
	if err := state.Parser.Prepare(); err != nil {
		return err
	}
	return nil
}

type section struct {
	title       name
	expressions []peg.Expression
}

func optional(e peg.Expression) peg.Expression {
	switch e := e.(type) {
	case *flagDecl:
		return flagSet{e}
	case flagSet:
		return e
	default:
		return peg.Optional(e)
	}
}

func choice(expressions []peg.Expression) peg.Expression {
	result := flagSet(expressions)
	copied := false
	for i, e := range expressions {
		switch e := e.(type) {
		case *flagDecl:
			if copied {
				result = append(result, e)
			}
		case flagSet:
			// lists of flagSets can be merged, but we have to copy the original
			if !copied {
				result = make(flagSet, i, len(expressions))
				copy(result, expressions)
				copied = true
			}
			result = append(result, e...)
		default:
			// impure so not a flagset
			return peg.Choice(expressions...)
		}
	}
	return result
}

func (v *value) Scan(s *peg.State) (interface{}, error) {
	state := s.Reader.(*state)
	if state.ReadPos >= len(state.Positional) {
		return nil, peg.NotMatched
	}
	r := result{
		binding: v.binding,
		value:   state.Positional[state.ReadPos],
	}
	state.ReadPos++
	return r, nil
}

func (v *value) Children() []peg.Expression { return nil }

func (v *value) Format(w fmt.State, r rune) { fmt.Fprintf(w, "<%v>", v.name.print) }

func (l *literal) Scan(s *peg.State) (interface{}, error) {
	if l.child != nil {
		return l.child.Scan(s)
	}
	state := s.Reader.(*state)
	if state.ReadPos >= len(state.Positional) {
		return nil, peg.NotMatched
	}
	r := result{
		binding: l.binding,
		value:   state.Positional[state.ReadPos],
	}
	// state.Pos == 0 is the special case of the program name, which always matches
	if state.ReadPos != 0 && r.value != l.name.print {
		return nil, peg.NotMatched
	}
	if isBoolField(r.binding.field) {
		r.value = "true"
	}
	state.ReadPos++
	return r, nil
}

func (l *literal) Children() []peg.Expression {
	if l.child != nil {
		return []peg.Expression{l.child}
	}
	return nil
}

func (l *literal) Format(w fmt.State, r rune) {
	if l.child != nil {
		fmt.Fprint(w, l.child)
		return
	}
	fmt.Fprintf(w, "'%v'", l.name.print)
}

func (f *flagDecl) Scan(s *peg.State) (interface{}, error) {
	state := s.Reader.(*state)
	if _, match := state.Present[f.binding]; match {
		return f, nil
	}
	return nil, peg.NotMatched
}

func (f *flagDecl) Children() []peg.Expression { return nil }

func (f *flagDecl) Format(w fmt.State, r rune) {
	fmt.Fprintf(w, "«")
	for i, name := range f.aliases {
		if i > 0 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, "%v", name)
	}
	if len(f.defaults) > 0 {
		fmt.Fprint(w, "=")
		for i, d := range f.defaults {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, "%v", d)
		}
	}
	fmt.Fprint(w, "»")
}

func (f flagSet) Scan(state *peg.State) (interface{}, error) {
	// a flag set always matches, and adds the entire set so that all its flags
	// are marked as allowed
	return f, nil
}

func (f flagSet) Children() []peg.Expression {
	return f
}

func (f flagSet) Format(w fmt.State, r rune) {
	fmt.Fprintf(w, "⸨")
	for i, flag := range f {
		if i > 0 {
			fmt.Fprint(w, " ")
		}
		fmt.Fprintf(w, "%v", flag)
	}
	fmt.Fprint(w, "⸩")
}

func (s *state) bind(b binding) *binding {
	// see if we should merge with an existing binding
	/*TODO: decide what the merging behavior should be, and should it
	happen here or in a separate pass, and is just using the flag names
	sufficient or should we also be using the other names?*/
	var result *binding
	for _, name := range b.flags {
		for _, check := range s.Bindings {
			if check.flags.Contains(name) {
				result = check
			}
		}
	}
	if result == nil {
		s.Bindings = append(s.Bindings, &b)
		return &b
	}
	result.flags = append(result.flags, b.flags...)
	result.params = append(result.params, b.params...)
	result.values = append(result.values, b.values...)
	result.defaults = append(result.defaults, b.defaults...)
	return result
}
