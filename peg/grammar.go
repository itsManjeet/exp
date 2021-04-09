// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package peg

import (
	"strings"
	"sync"
)

// NewGrammar builds a new Grammar as described by source.
// The source must be in the peg langauge as declared in Language.
func NewGrammar(name string, source string) (Grammar, error) {
	p := NewParser(LanguageParser())
	pegPrepare(p)
	// this uses a the PEG langauge parser to parse the grammar description in source
	v, err := p.Parse(name, strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	rules := v.([]interface{})
	g := make(Grammar, len(rules))
	for i, v := range rules {
		g[i] = v.(Rule)
	}
	return g, nil
}

var initOnce sync.Once

// LanguageParser returns a Grammar object for the PEG language itself.
// This builds the grammar by hand to bootstrap, but a test verifies that
// it matches the contents of the Language declared above.
func LanguageParser() Grammar {
	initOnce.Do(func() {
		initGrammar()
	})
	return builderGrammar
}

var builderGrammar Grammar

func initGrammar() {
	ws := Lookup("_")
	builderGrammar = Grammar{
		Rule{Comment: ` Copyright 2020 The Go Authors. All rights reserved.`},
		Rule{Comment: ` Use of this source code is governed by a BSD-style`},
		Rule{Comment: ` license that can be found in the LICENSE file.`},
		Rule{Comment: ``},
		Rule{Name: "PEG", Expression: Sequence(
			ZeroOrMore(Lookup("Line")),
			ws, Expect(EOF))},
		Rule{Name: "Line", Expression: Sequence(
			NotPredicate(EOF),
			ws, Optional(Lookup("Rule")),
			ws, Optional(Lookup("Comment")),
			Expect(Choice(mustPattern(`\n`), EOF)))},
		Rule{Name: "Comment", Expression: Sequence(
			Literal(`#`), mustPattern(`(.*)`))},
		Rule{Name: "Rule", Expression: Sequence(
			Lookup("Identifier"),
			ws, Expect(Literal(`<-`)),
			ws, Lookup("Expression"))},
		Rule{Name: "Lookup", Expression: Sequence(
			Lookup("Identifier"),
			NotPredicate(Sequence(ws, Literal(`<-`))))},
		Rule{Name: "Expression", Expression: Lookup("Choice")},
		Rule{Name: "Choice", Expression: Sequence(
			Lookup("Sequence"),
			ZeroOrMore(Sequence(
				ws, Literal(`/`),
				ws, Expect(Lookup("Sequence")))))},
		Rule{Name: "Sequence", Expression: Sequence(
			Lookup("Compound"),
			ZeroOrMore(Sequence(ws, Lookup("Compound"))))},
		Rule{Name: "Compound", Expression: Sequence(
			Choice(
				Sequence(Lookup("Prefix"), Expect(Lookup("Atom"))),
				Lookup("Atom"),
			), Lookup("Postfix"))},
		Rule{Name: "Prefix", Expression: mustPattern(`([&!:]+)`)},
		Rule{Name: "Atom", Expression: Choice(
			Lookup("Literal"),
			Lookup("Pattern"),
			Lookup("EOF"),
			Lookup("Lookup"),
			Lookup("Group"))},
		Rule{Name: "Postfix", Expression: Optional(mustPattern(`([+*?]*)`))},
		Rule{Name: "Literal", Expression: mustPattern(`"((?:\\.|[^"\\])*)"`)},
		Rule{Name: "Pattern", Expression: mustPattern(`\'((?:\\.|[^\'\\])*)\'`)},
		Rule{Name: "EOF", Expression: Literal(`$`)},
		Rule{Name: "Group", Expression: Sequence(
			Literal(`(`),
			ws, Optional(Lookup("Expression")),
			ws, Expect(Literal(`)`)))},
		Rule{Name: "Identifier", Expression: mustPattern(`(\w+)`)},
		Rule{Name: "_", Expression: Optional(mustPattern(`[ \t]*`))},
	}
}

func pegPrepare(p *Parser) {
	p.Process("Line", func(args ...interface{}) (interface{}, error) {
		switch len(args) {
		case 0:
			return Rule{}, nil
		case 1:
			return args[0], nil
		case 2:
			// we have a rule with a comment, combine them
			r := args[0].(Rule)
			c := args[1].(Rule)
			r.Comment = c.Comment
			return r, nil
		default:
			panic("grammar does not match rules")
		}
	})
	p.Process("Comment", func(args ...interface{}) (interface{}, error) {
		return Rule{Comment: args[0].(string)}, nil
	})
	p.Process("Rule", func(args ...interface{}) (interface{}, error) {
		return Rule{Name: args[0].(string), Expression: args[1].(Expression)}, nil
	})
	p.Process("Literal", func(args ...interface{}) (interface{}, error) {
		return Literal(args[0].(string)), nil
	})
	p.Process("Pattern", func(args ...interface{}) (interface{}, error) {
		return Pattern(args[0].(string))
	})
	p.Process("EOF", func(args ...interface{}) (interface{}, error) {
		return EOF, nil
	})
	p.Process("Lookup", func(args ...interface{}) (interface{}, error) {
		return Lookup(args[0].(string)), nil
	})
	p.Process("Compound", func(args ...interface{}) (interface{}, error) {
		if len(args) == 0 {
			return nil, nil
		}
		prefix, hasPrefix := args[0].(string)
		if hasPrefix {
			args = args[1:]
		}
		if len(args) == 0 {
			return nil, nil
		}
		node := args[0].(Expression)
		if len(args) == 2 {
			postfix := args[1].(string)
			for _, p := range postfix {
				switch p {
				case '+':
					node = OneOrMore(node)
				case '*':
					node = ZeroOrMore(node)
				case '?':
					node = Optional(node)
				}
			}
		}
		for i := len(prefix) - 1; i >= 0; i-- {
			switch prefix[i] {
			case '&':
				node = AndPredicate(node)
			case '!':
				node = NotPredicate(node)
			case ':':
				node = Expect(node)
			}
		}
		return node, nil
	})
	p.Process("Choice", func(args ...interface{}) (interface{}, error) {
		nodes := make([]Expression, len(args))
		for i, v := range args {
			nodes[i] = v.(Expression)
		}
		return Choice(nodes...), nil
	})
	p.Process("Sequence", func(args ...interface{}) (interface{}, error) {
		nodes := make([]Expression, len(args))
		for i, v := range args {
			nodes[i] = v.(Expression)
		}
		return Sequence(nodes...), nil
	})
}
