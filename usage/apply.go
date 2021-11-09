// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"flag"
	"fmt"
	"strconv"
)

type bindings struct {
	grammar      *grammar
	fields       *Fields
	byExpression map[expression]flag.Value
}

// bind maps from the named entities in a grammar to the fields of the options.
// It returns the discovered bindings, there will be exactly one binding per
// field.
// It is an error if there is any expression for which no field can be found.
func (g *grammar) Bind(fields *Fields) (*bindings, error) {
	b := &bindings{
		grammar:      g,
		byExpression: map[expression]flag.Value{},
	}

	var literalFields, valueFields, flagsFields fieldList
	for _, compare := range fields.all {
		if compare.Type&literalFieldType != 0 {
			literalFields = append(literalFields, compare)
		}
		if compare.Type&valueFieldType != 0 {
			valueFields = append(valueFields, compare)
		}
		if compare.Type&flagFieldType != 0 {
			flagsFields = append(flagsFields, compare)
		}
	}

	// find exactly one field for each named entity
	for _, l := range g.literals {
		fields := fieldsForLiteral(literalFields, l)
		if err := b.assignFields(l, fields); err != nil {
			return nil, err
		}
	}
	for _, v := range g.values {
		fields := fieldsForValue(valueFields, v)
		if err := b.assignFields(v, fields); err != nil {
			return nil, err
		}
	}
	for _, f := range g.flags {
		fields := fieldsForFlags(flagsFields, f)
		if err := b.assignFields(f, fields); err != nil {
			return nil, err
		}
	}
	return b, nil
}

func (b *bindings) assignFields(expr expression, fields fieldList) error {
	switch len(fields) {
	case 0:
		return fmt.Errorf("no field found for %v", expr)
	case 1:
		b.byExpression[expr] = fields[0].Value
		return nil
	default:
		return fmt.Errorf("ambiguous field found for %v", expr)
	}
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

func fieldsForLiteral(fields fieldList, l *literal) fieldList {
	if byName, _ := bestFields(fields, l.name); len(byName) > 0 {
		return byName
	}
	// TODO: should we prefer an exact group match over an approximate name match?
	if l.group != nil {
		if byGroup, _ := bestFields(fields, l.group.title); len(byGroup) > 0 {
			return byGroup
		}
	}
	return nil
}

func fieldsForValue(fields fieldList, v *value) fieldList {
	results, _ := bestFields(fields, v.name)
	return results
}

func fieldsForFlags(fields fieldList, flags *flags) fieldList {
	var results fieldList
	for _, a := range flags.aliases {
		matched, exact := bestFields(fields, a.name)
		if exact {
			return matched
		}
		if results == nil {
			results = matched
		} else {
			results = append(results, matched...)
		}
	}
	if len(results) > 0 {
		return results
	}
	if flags.param != nil {
		if results, _ := bestFields(fields, flags.param.name); len(results) > 0 {
			return results
		}
	}
	return nil
}

func bestFields(fields fieldList, name sname) (fieldList, bool) {
	// search backwards, the last exact match is always the "best" choice
	for i := len(fields) - 1; i >= 0; i-- {
		if fields[i].Name.Full == name.Full {
			// exact match, return just this one
			return fieldList{fields[i]}, true
		}
	}
	// no full match, check the simplified forms, but these may be ambiguous so find them all
	var results fieldList
	for _, compare := range fields {
		if approximateMatch(compare, name) {
			results = append(results, compare)
		}
	}
	return results, false
}

func approximateMatch(field *field, name sname) bool {
	if field.Name.Simple == name.Simple {
		return true
	}
	for _, alias := range field.Aliases {
		if alias.Simple == name.Simple {
			return true
		}
	}
	return false
}
