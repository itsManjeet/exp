// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"fmt"
	"unicode"
)

type parsePage struct {
	grammar *grammar
	section *section
	flags   *flags
}

type parseExpression struct {
	page *parsePage
	expr expression
}

func isSpace(r rune) bool       { return r == ' ' }
func isNotEOL(r rune) bool      { return r != '\n' }
func isDefaultRune(r rune) bool { return r != '[' && r != ']' && r != '\n' }
func isNameRune(r rune) bool {
	return r == '.' || r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func parseHelp(help []Page) (*grammar, error) {
	g := &grammar{}
	for _, p := range help {
		path := p.Path
		if path == "" {
			path = p.Name
		}
		in := newInput(path, string(p.Content))
		err := scan(in, func(in *input) {
			p := &parsePage{grammar: g}
			p.parse(in, g)
		})
		if err != nil {
			return nil, err
		}

	}
	return g, nil
}

func (p *parsePage) parse(in *input, g *grammar) {
	scanOptional(in, p.sectionStart)
	scanOptional(in, p.line)
	for !in.eof() {
		scanMust(in, "expect EOL", func(in *input) { scanRune(in, '\n') })
		scanMust(in, "expect line", p.line)
	}
}

func (p *parsePage) line(in *input) {
	switch {
	case scanOptional(in, p.sectionBreak):
	case scanOptional(in, p.statement):
	case scanOptional(in, p.description):
	}
}

func (p *parsePage) sectionStart(in *input) {
	title := parseTitle(in)
	p.section = p.grammar.findSection(title)
	if p.section == nil {
		p.section = &section{title: title}
		p.grammar.sections = append(p.grammar.sections, p.section)
	}
}

func (p *parsePage) sectionBreak(in *input) {
	scanSkip(in, isSpace)
	scanRune(in, '\n')
	p.sectionStart(in)
}

func parseTitle(in *input) string {
	name := parseName(in)
	scanRune(in, ':')
	scanOptional(in, func(in *input) {
		scanRune(in, ' ')
		scanSkip(in, isNotEOL)
	})
	return name
}

func (p *parsePage) statement(in *input) {
	child := parseExpression{page: p}
	scanString(in, `  `)
	if scanPeek(in, func(in *input) { scanRune(in, ' ') }) {
		// triple space does not match
		panic(errNotMatched)
	}
	scanMust(in, "expect expression", child.scan)
	scanOptional(in, func(in *input) {
		scanString(in, `  `)
		p.description(in)
	})
	switch e := p.section.root.(type) {
	case nil:
		p.section.root = child.expr
	case choice:
		p.section.root = append(e, child.expr)
	default:
		p.section.root = choice{e, child.expr}
	}
}

func (p *parsePage) description(in *input) {
	p.descriptionEntry(in)
	for {
		if !scanOptional(in, p.descriptionEntry) {
			return
		}
	}
}

func (p *parsePage) descriptionEntry(in *input) {
	switch {
	case scanOptional(in, p.defaultValue):
	case scanOptional(in, func(in *input) {
		// one or more non default starter runes
		scanClass(in, isDefaultRune)
		scanSkip(in, isDefaultRune)
	}):
	case scanOptional(in, func(in *input) {
		// skip any single non eol char
		scanClass(in, isNotEOL)
	}):
	default:
		panic(errNotMatched)
	}
}

func (p *parsePage) defaultValue(in *input) {
	scanString(in, `[default:`)
	var value string
	scanOptional(in, func(in *input) {
		scanRune(in, ' ')
		value = scanCapture(in, func(in *input) { scanSkip(in, isDefaultRune) })
	})
	scanMust(in, `expect "]"`, func(in *input) { scanRune(in, ']') })
	if p.flags == nil {
		panic(&parseError{
			In:      *in,
			Message: fmt.Sprintf("default value %q not associated with a flag", value),
			Fatal:   true,
		})
	}
	if p.flags.default_ != "" && p.flags.default_ != value {
		panic(&parseError{
			In:      *in,
			Message: fmt.Sprintf("default value %q conflicts with %q", value, p.flags.default_),
			Fatal:   true,
		})
	}
	p.flags.default_ = value
}

func parseName(in *input) string {
	return scanCapture(in, func(in *input) {
		scanClass(in, unicode.IsLetter)
		scanSkip(in, isNameRune)
	})
}

func (e *parseExpression) scan(in *input) {
	e.sequence(in)
}

func (e *parseExpression) sequence(in *input) {
	sequence := sequence{}
	child := parseExpression{page: e.page}
	child.choice(in)
	if child.expr != nil {
		sequence = append(sequence, child.expr)
	}
	for {
		// double space terminates the sequence
		// single space continues the sequence
		if scanPeek(in, func(in *input) { scanString(in, `  `) }) ||
			!scanOptional(in, func(in *input) { scanRune(in, ' ') }) {
			e.expr = sequence
			return
		}
		scanMust(in, "expect sequence expression", child.choice)
		if child.expr != nil {
			sequence = append(sequence, child.expr)
		}
	}
}

func (e *parseExpression) choice(in *input) {
	choice := choice{}
	child := parseExpression{page: e.page}
	child.repeat(in)
	if child.expr != nil {
		choice = append(choice, child.expr)
	}
	for {
		if !scanOptional(in, func(in *input) {
			scanOptional(in, func(in *input) { scanRune(in, ' ') })
			scanRune(in, '|')
			scanOptional(in, func(in *input) { scanRune(in, ' ') })
		}) {
			e.expr = choice
			return
		}
		scanMust(in, "expect choice expression", child.repeat)
		if child.expr != nil {
			choice = append(choice, child.expr)
		}
	}
}

func (e *parseExpression) repeat(in *input) {
	e.atom(in)
	if scanOptional(in, func(in *input) { scanString(in, `...`) }) {
		r := &repeat{expression: e.expr}
		e.expr = r
	}
}

func (e *parseExpression) atom(in *input) {
	switch {
	case scanOptional(in, e.optional):
	case scanOptional(in, e.group):
	case scanOptional(in, e.flag):
	case scanOptional(in, e.named):
	case scanOptional(in, e.literal):
	default:
		panic(errNotMatched)
	}
}

func (e *parseExpression) optional(in *input) {
	child := parseExpression{page: e.page}
	scanRune(in, '[')
	scanMust(in, "expect optional expression", child.scan)
	scanMust(in, `expect "]"`, func(in *input) { scanRune(in, ']') })
	e.expr = &optional{expression: child.expr}
}

func (e *parseExpression) group(in *input) {
	scanRune(in, '(')
	scanMust(in, "expect group expression", e.scan)
	scanMust(in, `expect ")"`, func(in *input) { scanRune(in, ')') })
}

func (e *parseExpression) flag(in *input) {
	var name string
	scanRune(in, '-')
	scanMust(in, "expect flag name", func(in *input) { name = parseName(in) })
	f := &flags{}
	f.aliases = append(f.aliases, &flagName{
		name:  name,
		flags: f,
	})
	for {
		if !scanOptional(in, func(in *input) { scanRune(in, ',') }) {
			break
		}
		scanSkip(in, isSpace)
		scanMust(in, `expect "-"`, func(in *input) { scanRune(in, '-') })
		scanMust(in, "expect flag name", func(in *input) { name = parseName(in) })
		f.aliases = append(f.aliases, &flagName{
			name:  name,
			flags: f,
		})
	}
	scanOptional(in, func(in *input) {
		scanRune(in, '=')
		scanSkip(in, isSpace)
		scanMust(in, "expect flag parameter", func(in *input) { name = parseName(in) })
		f.param = &parameter{
			name:  name,
			flags: f,
		}
	})
	e.expr = f
	e.page.flags = f
}

func (e *parseExpression) named(in *input) {
	var name string
	scanRune(in, '<')
	scanMust(in, "expect name", func(in *input) { name = parseName(in) })
	scanMust(in, `expect ">"`, func(in *input) { scanRune(in, '>') })
	e.expr = &value{name: name}
}

func (e *parseExpression) literal(in *input) {
	e.expr = &literal{name: parseName(in)}
}
