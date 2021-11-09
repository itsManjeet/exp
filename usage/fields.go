// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage

import (
	"flag"
	"fmt"
	"reflect"
	"time"
)

// Fields is the complete set of output values that Process can write into when
// it matches the args.
// It maintains separate field sets for the different production types that can
// produce an output.
type Fields struct {
	literals []field
	values   []field
	flags    []field
}

type field struct {
	Name  string
	Value flag.Value
}

// FieldsOf scans fields of structs recursively to find things all
// exported fields that could be bound to flags.
func FieldsOf(options interface{}) (*Fields, error) {
	// find the set of fields from the options struct
	if options == nil {
		return nil, fmt.Errorf("no options struct")
	}
	v := reflect.ValueOf(options)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("bind must be given a pointer to a struct")
	}
	// scan for all the fields
	fields := &Fields{}
	if err := fields.discover("", v); err != nil {
		return nil, err
	}
	return fields, nil
}

// Literal adds a field to the set that is allowed to match a literal production
// in the grammar.
func (fields *Fields) Literal(name string, value interface{}) {
	fields.literals = append(fields.literals, createField(name, reflect.ValueOf(value)))
}

// Value adds a field to the set that is allowed to match a positional value
// production in the grammar.
func (fields *Fields) Value(name string, value interface{}) {
	fields.values = append(fields.values, createField(name, reflect.ValueOf(value)))
}

// Flag adds a field to the set that is allowed to match a flag production
// in the grammar.
func (fields *Fields) Flag(name string, value interface{}) {
	fields.flags = append(fields.flags, createField(name, reflect.ValueOf(value)))
}

func (fields *Fields) discover(parent string, v reflect.Value) error {
	// we know v is a pointer to struct here
	st := v.Elem().Type()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		// is it a field we are allowed to reflect on?
		if f.PkgPath != "" {
			continue
		}
		child := v.Elem().Field(i).Addr()
		name := parent
		if len(name) > 0 {
			name = name + "."
		}
		name = name + f.Name

		field := createField(name, child)
		if field.Value != nil {
			fields.literals = append(fields.literals, field)
			fields.values = append(fields.values, field)
			fields.flags = append(fields.flags, field)
			continue
		}

		switch child.Elem().Kind() {
		case reflect.Struct:
			scope := name
			if f.Anonymous {
				scope = parent
			}
			if err := fields.discover(scope, child); err != nil {
				return err
			}
		case reflect.Ptr:
			// we skip over pointer fields, and assume they should not be bound here
			//TODO: in theory we could instead verify that pointer values are bound once
			//TODO: allow things that are flag values for clever flag types
		case reflect.Func:
			// just ignore function fields
		default:
			return fmt.Errorf("cannot handle field type %v", child.Type())
		}
	}
	return nil
}

func createField(name string, v reflect.Value) field {
	set := flag.FlagSet{}
	switch v := v.Interface().(type) {
	case flag.Value:
		set.Var(v, name, name)
	case *bool:
		set.BoolVar(v, name, false, name)
	case *time.Duration:
		set.DurationVar(v, name, 0, name)
	case *float64:
		set.Float64Var(v, name, 0, name)
	case *int64:
		set.Int64Var(v, name, 0, name)
	case *int:
		set.IntVar(v, name, 0, name)
	case *string:
		set.StringVar(v, name, "", name)
	case *uint:
		set.UintVar(v, name, 0, name)
	case *uint64:
		set.Uint64Var(v, name, 0, name)
	case *[]string:
		set.Var(stringList{v}, name, name)
	default:
		panic(fmt.Errorf("invalid value type %T", v))
	}
	return field{Name: name, Value: set.Lookup(name).Value}
}

func isBool(v flag.Value) bool {
	if t, ok := v.(interface{ IsBoolFlag() bool }); ok {
		return t.IsBoolFlag()
	}
	return false
}
