// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
)

type matchState struct {
	args    []string
	index   int
	allowed []*flags
	results []result
}

type matchError struct {
	state    matchState
	expected expression
}

// resultFlag implements flag.Value so that calls to Set capture the value
// into a result list.
// This is used when processing a command line for flags to capture all the
// flags without applying them so that the patterns can be tested.
type resultFlag struct {
	name  sname
	state *matchState
	flags *flags
}

type result struct {
	expr  expression
	value string
}

func match(g *grammar, args []string) ([]result, error) {
	state := &matchState{args: args}
	// parse the command line arguments picking out the flags
	if err := matchFlags(state, g); err != nil {
		return state.results, err
	}

	// now attempt to match the positionals against the usage patterns
	state, err := matchExpression(state, g.usage)
	if err != nil {
		return state.results, err
	}
	if state.index < len(state.args) {
		return state.results, fmt.Errorf("unexpected %q", args[state.index])
	}

	var present []*flags
	// now validate that all the flags that were present were allowed
	for _, r := range state.results {
		if flags, ok := r.expr.(*flags); ok {
			if !flagsInList(state.allowed, flags) {
				return state.results, fmt.Errorf("flag %q present but not allowed", flags.String())
			}
			present = append(present, flags)
		}
	}

	// add flags with defaults that were allowed but not present
	for _, flags := range state.allowed {
		if flags.default_ == "" {
			continue
		}
		if !flagsInList(present, flags) {
			state.add(flags, flags.default_)
		}
	}

	return state.results, nil
}

// matchFlags parses all the flags from the args, returning the remaining
// positional args.
func matchFlags(state *matchState, g *grammar) error {
	// first build the flagset
	flagset := flag.NewFlagSet("", flag.ContinueOnError)
	flagset.SetOutput(ioutil.Discard)
	for _, f := range g.flags {
		for _, alias := range f.aliases {
			rf := &resultFlag{name: alias.name, state: state, flags: alias.flags}
			flagset.Var(rf, rf.name.Full, "")
		}
	}

	// in order to allow flags to move around the verbs, we repeat the parse
	// until we are confident there are no more flags
	positional := []string{}
	for args := state.args; len(args) > 0; {
		switch {
		case args[0] == "--":
			//accept -- as a terminator for this process, to allow flag like
			//positional arguments when needed.
			positional = append(positional, args[1:]...)
			args = nil
		case strings.HasPrefix(args[0], "-"):
			// first arg is a flag, parse to remove all flags from the front
			if err := flagset.Parse(args); err != nil {
				return err
			}
			args = flagset.Args()
		default:
			// first arg is a positional, remove it in case there are more flags
			positional = append(positional, args[0])
			args = args[1:]
		}
	}
	state.args = positional
	return nil
}

func matchExpression(state *matchState, expr expression) (rs *matchState, re error) {
	var err error
	switch expr := expr.(type) {
	case sequence:
		for _, e := range expr {
			state, err = matchExpression(state, e)
			if err != nil {
				return state, err
			}
		}
		// matched the entire sequence
		return state, nil
	case choice:
		nonFlags := 0
		for _, e := range expr {
			if e, ok := e.(*flags); ok {
				state.allowed = append(state.allowed, e)
			} else {
				nonFlags++
			}
		}
		if nonFlags == 0 {
			// only flags, so we match
			return state, nil
		}
		mark := *state
		// if longest.state.index < 0 there has been no match
		// if longest.expected != nil the match is an error
		longest := matchError{state: matchState{index: -1}}
		for _, e := range expr {
			if _, ok := e.(*flags); ok {
				continue
			}
			state, err = matchExpression(state, e)
			switch err := err.(type) {
			case nil:
				// was a match, check if we are longest or if previous longest was an error
				//TODO: in theory we could shortcut if the longest match consumed all the input
				if longest.state.index < state.index || longest.expected != nil {
					longest.expected = nil
					longest.state.copy(state)
				}
			case *matchError:
				// was a recoverable error
				if longest.state.index < 0 || (longest.state.index < err.state.index && longest.expected != nil) {
					longest.expected = err.expected
					longest.state.copy(&err.state)
				}
			default:
				// was a non recoverable error
				return state, err
			}
			// try the next choice
			*state = mark
		}
		if longest.state.index < 0 {
			// empty choice set, so we match
			return state, nil
		}
		// restore the state of the longest match, even if it was an error
		*state = longest.state
		if longest.expected != nil {
			// none of the choices matched
			return state, &longest
		}
		// longest match was a success
		return state, nil
	case *optional:
		mark := *state
		state, err := matchExpression(state, expr.expression)
		if _, isNotMatch := err.(*matchError); isNotMatch {
			// not matched, backtrack and ignore
			*state = mark
			return state, nil
		}
		return state, err
	case *repeat:
		for {
			mark := *state
			state, err := matchExpression(state, expr.expression)
			if _, isNotMatch := err.(*matchError); isNotMatch {
				// not matched, backtrack to previous match and return
				*state = mark
				return state, nil
			}
		}
	case *section:
		return matchExpression(state, expr.root)
	case *value:
		s, err := state.next()
		if err != nil {
			err.expected = expr
			return state, err
		}
		state.add(expr, s)
		return state, nil
	case *literal:
		expected := expression(expr)
		if expr.group != nil {
			expected = expr.group
		}
		s, err := state.next()
		if err != nil {
			err.expected = expected
			return state, err
		}
		// special case the 0th literal, it is the program name which might not match
		if state.index > 1 && expr.name.Full != s {
			return state, &matchError{state: *state, expected: expected}
		}
		state.add(expr, s)
		return state, nil
	case *flags:
		// flags add to the allowed set but consume nothing
		state.allowed = append(state.allowed, expr)
		return state, nil
	default:
		panic(fmt.Errorf("unknown expression type %T", expr))
	}
}

func (m *matchState) next() (string, *matchError) {
	// we pre-increment so that the captured state on failure is advanced
	m.index++
	if m.index > len(m.args) {
		return "", &matchError{state: *m}
	}
	// we pre-incremented, so the arg we are consuming is behind index
	return m.args[m.index-1], nil
}

func (m *matchState) add(expr expression, v string) {
	m.results = append(m.results, result{
		expr:  expr,
		value: v,
	})
}

func (m *matchState) copy(other *matchState) {
	m.args = other.args
	m.index = other.index
	m.results = append(make([]result, 0, len(other.results)), other.results...)
	m.allowed = append(make([]*flags, 0, len(other.allowed)), other.allowed...)
}

func (r *resultFlag) String() string {
	return r.name.Full
}

func (r *resultFlag) Set(v string) error {
	r.state.add(r.flags, v)
	return nil
}

func (r *resultFlag) IsBoolFlag() bool {
	return r.flags.param == nil
}

func flagsInList(list []*flags, f *flags) bool {
	for _, entry := range list {
		if entry == f {
			return true
		}
	}
	return false
}

func (err *matchError) Error() string {
	if err.state.index < len(err.state.args) {
		return fmt.Sprintf("expected %v got %q", err.expected, err.state.args[err.state.index])
	}
	return fmt.Sprintf("expected %v", err.expected)
}
