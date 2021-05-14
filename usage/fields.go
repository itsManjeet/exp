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

// binding is a thing that can occur on the command line and bound to one or
// more fields of the options.
// It implements flag.Flag which can be used to apply the values.
type binding struct {
	defaults []string
	flags    nameSet
	params   nameSet
	values   nameSet
	field    *field
}

type remap struct {
	name name
	to   *field
}

// field in the options structure that can be bound to the help
type field struct {
	name name
	flag flag.Value // used to modify the field from a string value
}

// scan scans fields of structs recursively to find things all
// exported fields that should be bound to flags.
func scan(state *state, o interface{}) error {
	v := reflect.ValueOf(o)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("bind must be given a pointer to a struct")
	}
	// scan for all the fields
	if err := walkFields(state, name{}, v); err != nil {
		return err
	}
	// now handle any remaps
	for text, fname := range state.Text.Map {
		rm := remap{name: toName(text)}
		if fname == "" {
			// a blackhole mapping
			var ignore string
			rm.to = createField(rm.name, reflect.ValueOf(&ignore))
		} else {
			// must find an existing field
			fname := toName(fname)
			for _, f := range state.Fields {
				if !f.name.Equivalent(fname) {
					continue
				}
				rm.to = f
			}
			if rm.to == nil {
				return fmt.Errorf("no field found for remap %v => %v", rm.name, fname)
			}
		}
		state.Remaps = append(state.Remaps, rm)
	}
	return nil
}

func walkFields(state *state, parent name, v reflect.Value) error {
	// we know v is a pointer to struct here
	st := v.Elem().Type()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		// is it a field we are allowed to reflect on?
		if f.PkgPath != "" {
			continue
		}
		child := v.Elem().Field(i).Addr()
		name := parent.Extend(f.Name)

		field := createField(name, child)
		if field != nil {
			state.Fields = append(state.Fields, field)
			continue
		}

		switch child.Elem().Kind() {
		case reflect.Struct:
			scope := name
			if f.Anonymous {
				scope = parent
			}
			if err := walkFields(state, scope, child); err != nil {
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

func createField(name name, v reflect.Value) *field {
	set := flag.FlagSet{}
	ns := name.String()
	switch v := v.Interface().(type) {
	case flag.Value:
		set.Var(v, ns, ns)
	case *bool:
		set.BoolVar(v, ns, false, ns)
	case *time.Duration:
		set.DurationVar(v, ns, 0, ns)
	case *float64:
		set.Float64Var(v, ns, 0, ns)
	case *int64:
		set.Int64Var(v, ns, 0, ns)
	case *int:
		set.IntVar(v, ns, 0, ns)
	case *string:
		set.StringVar(v, ns, "", ns)
	case *uint:
		set.UintVar(v, ns, 0, ns)
	case *uint64:
		set.Uint64Var(v, ns, 0, ns)
	case *[]string:
		set.Var(stringList{v}, ns, ns)
	default:
		return nil
	}
	flag := set.Lookup(ns)
	field := &field{
		name: name,
		flag: flag.Value,
	}
	return field
}

func bind(state *state) error {
	// go through the bindings in order for reproducible results
	for _, b := range state.Bindings {
		// find exactly one field for each binding
		if bindOne(state, b, b.flags) != nil {
			continue
		}
		if bindOne(state, b, b.params) != nil {
			continue
		}
		if bindOne(state, b, b.values) != nil {
			continue
		}
		// all bindings must match a field
		return fmt.Errorf("no field found for %q", b.String())
	}
	return nil
}

func bindOne(state *state, binding *binding, names nameSet) *field {
	//TODO: should we check for ambiguous binding to field mappings?
	if binding.field != nil {
		return binding.field
	}
	for _, r := range state.Remaps {
		if !names.Contains(r.name) {
			continue
		}
		// apply the remap
		binding.field = r.to
		return r.to
	}
	for _, f := range state.Fields {
		if !names.Contains(f.name) {
			continue
		}
		binding.field = f
		return f
	}
	return nil
}

func (b *binding) String() string {
	if len(b.flags) > 0 {
		return b.flags[0].String()
	}
	if len(b.params) > 0 {
		return b.params[0].String()
	}
	if len(b.values) > 0 {
		return b.values[0].String()
	}
	if b.field != nil {
		return b.field.name.String()
	}
	return "??"
}
