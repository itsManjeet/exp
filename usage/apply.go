// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type bindings struct {
	byExpression map[expression]flag.Value
}

// bind maps from the named entities in a grammar to the fields of the options.
// It returns the discovered bindings, there will be exactly one binding per
// field.
// It is an error if there is any expression for which no field can be found.
func bind(g *grammar, fields *Fields) (*bindings, error) {
	b := &bindings{
		byExpression: map[expression]flag.Value{},
	}

	// find exactly one field for each named entity
	for _, l := range g.literals {
		field := fieldForLiteral(fields, l)
		if field == nil {
			return nil, fmt.Errorf("no field found for literal %q", l.name)
		}
		b.byExpression[l] = field.Value
	}
	for _, v := range g.values {
		field := fieldForValue(fields, v)
		if field == nil {
			return nil, fmt.Errorf("no field found for value %q", v.name)
		}
		b.byExpression[v] = field.Value
	}
	for _, f := range g.flags {
		field := fieldForFlags(fields, f)
		if field == nil {
			return nil, fmt.Errorf("no field found for flag %q", f.String())
		}
		b.byExpression[f] = field.Value
	}
	return b, nil
}

// apply takes the results of match and uses them to set the fields.
func (b *bindings) apply(results []result) error {
	for _, r := range results {
		v, ok := b.byExpression[r.expr]
		if !ok {
			return fmt.Errorf("result %q that has no binding", r.expr)
		}
		if v == nil {
			// a deliberately ignored value
			return nil
		}
		if isBool(v) {
			if _, err := strconv.ParseBool(r.value); err != nil {
				r.value = "true"
			}
		}
		v.Set(r.value)
	}
	return nil
}

func fieldForLiteral(fields *Fields, l *literal) *field {
	if f := fieldByName(fields.literals, l.name); f != nil {
		return f
	}
	if l.group != nil {
		if f := fieldByName(fields.literals, l.group.title); f != nil {
			return f
		}
	}
	return nil
}

func fieldForValue(fields *Fields, v *value) *field {
	return fieldByName(fields.values, v.name)
}

// fieldForFlags attempts to find a field for a given set of flags.
// It attempts to find the field using any of the flag aliases, or the parameter
// name if the flag has one.
func fieldForFlags(fields *Fields, flags *flags) *field {
	for _, a := range flags.aliases {
		if field := fieldByName(fields.flags, string(a.name)); field != nil {
			return field
		}
	}
	if flags.param != nil {
		if field := fieldByName(fields.flags, string(flags.param.name)); field != nil {
			return field
		}
	}
	return nil
}

// fieldByName attempts to find a field that matches a given name.
func fieldByName(fields []field, name string) *field {
	//all searches are backwards to find the last match

	// first search for a perfecdt match
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].Name == name {
			return &fields[i]
		}
	}

	//see if we are only out by capitalization
	for i := len(fields) - 1; i >= 0; i-- {
		if strings.EqualFold(fields[i].Name, name) {
			return &fields[i]
		}
	}

	//try stripping all non alphanumeric characters
	simpleName := strings.Map(simpleLetters, name)
	for i := len(fields) - 1; i >= 0; i-- {
		if strings.EqualFold(fields[i].Name, simpleName) {
			return &fields[i]
		}
	}

	return nil
}

func simpleLetters(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	return -1
}
