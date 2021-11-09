// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build nope
// +build nope

package usage

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type parser struct {
	remains       string
	err           error
	grammar       *grammar
	activeSection *section
	lastFlag      *flags
}

type pattern struct{ *regexp.Regexp }

var errNotMatched = errors.New("not matched")

func newPattern(p string) pattern {
	r, err := regexp.Compile("^" + p)
	if err != nil {
		panic(err)
	}
	r.Longest()
	return pattern{r}
}

func (p *parser) setContent(content string) {
	p.remains = content
}

func (p *parser) matched() bool {
	return p.err == nil
}

func (p *parser) mark() parser {
	return *p
}

func (p *parser) backtrack(mark parser) {
	*p = mark
}

func (p *parser) eof() bool {
	return p.remains == ""
}

func (p *parser) pattern(re pattern) string {
	if p.err != nil {
		return ""
	}
	match := re.FindStringIndex(p.remains)
	if len(match) < 2 {
		p.err = errNotMatched
		return ""
	}
	result := p.remains[match[0]:match[1]]
	p.remains = p.remains[match[1]:]
	return result
}

func (p *parser) string(s string) {
	if p.err != nil {
		return
	}
	if !strings.HasPrefix(p.remains, s) {
		p.err = errNotMatched
		return
	}
	p.remains = p.remains[len(s):]
}

func (p *parser) peek(f func()) bool {
	if p.err != nil {
		return false
	}
	mark := p.mark()
	f()
	matched := p.err == nil
	p.backtrack(mark)
	return matched
}

func (p *parser) maybe(f func()) bool {
	if p.err != nil {
		return false
	}
	mark := p.mark()
	f()
	switch p.err {
	case errNotMatched:
		p.backtrack(mark)
		return false
	case nil:
		return true
	default:
		return false
	}
}

func (p *parser) expect(msg string, f func()) {
	if p.err != nil {
		return
	}
	f()
	if p.err == errNotMatched {
		p.fail("expect %s got %q", msg, p.prefix())
	}
}

func (p *parser) expectString(s string) {
	if p.err != nil {
		return
	}
	p.string(s)
	if p.err == errNotMatched {
		p.fail("expect %q got %q", s, p.prefix())
	}
}

func (p *parser) any(choices ...func()) bool {
	if len(choices) < 2 {
		panic("at least two choices required")
	}
	if p.err != nil {
		return false
	}
	mark := p.mark()
	for _, f := range choices {
		f()
		switch p.err {
		case errNotMatched:
			// try the next choice
			p.backtrack(mark)
		case nil:
			return true
		default:
			return false
		}
	}
	// no choices matched
	p.err = errNotMatched
	return false
}

func (p *parser) fail(msg string, args ...interface{}) {
	p.err = fmt.Errorf(msg, args...)
}

func (p *parser) prefix() string {
	prefix := p.remains
	if len(prefix) > 20 {
		prefix = prefix[:20]
	}
	return prefix
}
