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
	all fieldList
}

type fieldList []*field

type fieldType byte

const (
	literalFieldType = 1 << iota
	valueFieldType
	flagFieldType
	lastFieldType
)
const allFieldTypes = lastFieldType - 1

type field struct {
	Type    fieldType
	Name    sname
	Aliases []sname
	Value   flag.Value
}

// Scan a structure for all it's fields and add them to the field set.
func (f *Fields) Scan(options interface{}) error {
	// find the set of fields from the options struct
	if options == nil {
		return fmt.Errorf("no options struct")
	}
	v := reflect.ValueOf(options)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("bind must be given a pointer to a struct")
	}
	// scan for all the fields
	if err := f.discover("", v); err != nil {
		return err
	}
	return nil
}

// Literal adds a field to the set that is allowed to match a literal production
// in the grammar.
func (fields *Fields) Literal(name string, value interface{}) {
	fields.all = append(fields.all, createField(name, literalFieldType, reflect.ValueOf(value)))
}

// Value adds a field to the set that is allowed to match a positional value
// production in the grammar.
func (fields *Fields) Value(name string, value interface{}) {
	fields.all = append(fields.all, createField(name, valueFieldType, reflect.ValueOf(value)))
}

// Flag adds a field to the set that is allowed to match a flag production
// in the grammar.
func (fields *Fields) Flag(name string, value interface{}) {
	fields.all = append(fields.all, createField(name, flagFieldType, reflect.ValueOf(value)))
}

// Alias adds a an alias to an existing field.
func (fields *Fields) Alias(name string, alias string) {
	for i := len(fields.all) - 1; i >= 0; i-- {
		if fields.all[i].Name.Full == name {
			fields.all[i].Aliases = append(fields.all[i].Aliases, makeSName(alias))
		}
	}
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
		fullname, shortname := f.Name, ""
		if len(parent) > 0 {
			shortname = fullname
			fullname = parent + "." + fullname
		}

		field := createField(fullname, allFieldTypes, child)
		if field.Value != nil {
			if shortname != "" {
				field.Aliases = []sname{makeSName(shortname)}
			}
			fields.all = append(fields.all, field)
			continue
		}

		switch child.Elem().Kind() {
		case reflect.Struct:
			scope := fullname
			if f.Anonymous {
				scope = parent
			}
			if err := fields.discover(scope, child); err != nil {
				return err
			}
		case reflect.Ptr, reflect.Func, reflect.Map:
			// we skip over pointer function and map fields
			// they are allowed to be present but not bound
		default:
			// anything else we cannot recurse and did not build a field for, so we complain
			return fmt.Errorf("cannot handle field type %v", child.Elem().Type())
		}
	}
	return nil
}

var (
	boolPtrT    = reflect.TypeOf((*bool)(nil))
	int64PtrT   = reflect.TypeOf((*int64)(nil))
	intPtrT     = reflect.TypeOf((*int)(nil))
	uintPtrT    = reflect.TypeOf((*uint)(nil))
	uint64PtrT  = reflect.TypeOf((*uint64)(nil))
	float64PtrT = reflect.TypeOf((*float64)(nil))
	stringPtrT  = reflect.TypeOf((*string)(nil))
)

func createField(name string, typ fieldType, v reflect.Value) *field {
	f := &field{
		Type: typ,
		Name: makeSName(name),
	}
	if !v.IsValid() {
		return f
	}
	set := flag.FlagSet{}
	switch r := v.Interface().(type) {
	case flag.Value:
		set.Var(r, name, name)
	case *time.Duration:
		set.DurationVar(r, name, 0, name)
	case *[]string:
		set.Var(stringList{r}, name, name)
	default:
		switch v.Elem().Kind() {
		case reflect.Bool:
			set.BoolVar(v.Convert(boolPtrT).Interface().(*bool), name, false, name)
		case reflect.Int64:
			set.Int64Var(v.Convert(int64PtrT).Interface().(*int64), name, 0, name)
		case reflect.Int:
			set.IntVar(v.Convert(intPtrT).Interface().(*int), name, 0, name)
		case reflect.Uint:
			set.UintVar(v.Convert(uintPtrT).Interface().(*uint), name, 0, name)
		case reflect.Uint64:
			set.Uint64Var(v.Convert(uint64PtrT).Interface().(*uint64), name, 0, name)
		case reflect.Float64:
			set.Float64Var(v.Convert(float64PtrT).Interface().(*float64), name, 0, name)
		case reflect.String:
			set.StringVar(v.Convert(stringPtrT).Interface().(*string), name, "", name)
		}
	}
	if flag := set.Lookup(name); flag != nil {
		f.Value = flag.Value
	}
	return f
}

func isBool(v flag.Value) bool {
	if t, ok := v.(interface{ IsBoolFlag() bool }); ok {
		return t.IsBoolFlag()
	}
	return false
}

func (f *Fields) Format(w fmt.State, r rune) {
	for _, entry := range f.all {
		fmt.Fprint(w, entry.Name)
		if len(entry.Aliases) > 0 {
			fmt.Fprint(w, " <= ")
			for _, a := range entry.Aliases {
				fmt.Fprint(w, a)
			}
		}
		fmt.Fprint(w, "\n")
	}
}

func (n sname) Format(w fmt.State, r rune) {
	fmt.Fprint(w, n.Full)
	if n.Full != n.Simple && w.Flag('+') {
		fmt.Fprintf(w, "[%s]", n.Simple)
	}
}
