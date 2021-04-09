// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package peg_test

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/exp/peg"
)

func ExampleParser_ini_file() {
	const language = `
File    <- Line* :$
Line    <- !$ _ (Section / Assign)? _ Comment? :('\n' / $)
Comment <- '[;#].*'
Section <- '\[([^\]]+)\]'
Assign 	<- Key _ ("=" _ (Quoted / Value))
Key     <- '([\w]+)'
Quoted  <- '"((?:\\.|[^"\\])*)"'
Value   <- '([^;#\n]*)'
_       <- '[ \t]*'?
`
	const raw = `
[Common]
# A section common to all pages
DoThings = true
Title = "Hello from the application"
[Disabled]
# Settings for disabled pages
DoThings = false
`
	// first build the parser for our language
	g, err := peg.NewGrammar(`DocLanguage`, language)
	if err != nil {
		fmt.Println("Language error", err)
		return
	}
	p := peg.NewParser(g)
	p.Process("Section", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("in section %q\n", args[0])
		return args, nil
	})
	p.Process("Assign", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("set %q to value %q\n", args[0], args[1])
		return args, nil
	})
	if err != nil {
		fmt.Println("Langauge error", err)
		return
	}
	// now get ourselves a string input
	input := strings.NewReader(raw)
	// now parse that input using the grammar
	_, err = p.Parse("input", input)
	if err != nil {
		fmt.Println("Unexpected error", err)
	}
	//Output:
	// in section "Common"
	// set "DoThings" to value "true"
	// set "Title" to value "Hello from the application"
	// in section "Disabled"
	// set "DoThings" to value "false"
}

func ExampleParser_tokens() {
	const language = `
Config  <- :Title Line* :$
Title   <- "config" _ :Words :";" _
Line    <- (Section / Assign / Comment) :";" _
Section <- "section" _ :Words
Assign  <- "set" _ :'([^￤]*)' _ :"to" _ :Words
Comment <- "#" _ :Words
Words   <- Word+
Word    <- '([^;￤]*)' _
_       <- :("￤" / $)
`
	const raw = `
config Application ;
section Common ;
# A section common to all pages ;
set DoThings to true ;
set Title to Hello from the application ;
section Disabled ;
# Settings for disabled pages ;
set DoThings to false ;
`
	// first build the parser for our language
	g, err := peg.NewGrammar(`DocLanguage`, language)
	if err != nil {
		fmt.Println("Language error", err)
		return
	}
	p := peg.NewParser(g)
	p.Process("Title", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("processing %q config\n", args[0])
		return args, nil
	})
	p.Process("Section", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("in section %q\n", args[0])
		return args, nil
	})
	p.Process("Assign", func(args ...interface{}) (interface{}, error) {
		var value strings.Builder
		for i, arg := range args[1:] {
			if i > 0 {
				value.WriteString(" ")
			}
			value.WriteString(arg.(string))
		}
		fmt.Printf("set %q to value %q\n", args[0], value.String())
		return args, nil
	})

	// now get ourselves a token input
	input := strings.NewReader(strings.Join(strings.Fields(raw), "￤"))
	// now parse that input using the grammar
	_, err = p.Parse("input", input)
	if err != nil {
		fmt.Println("Unexpected error", err)
	}
	//Output:
	// processing "Application" config
	// in section "Common"
	// set "DoThings" to value "true"
	// set "Title" to value "Hello from the application"
	// in section "Disabled"
	// set "DoThings" to value "false"
}

func ExampleParser_calculator() {
	const language = `
Input       <- _ (Calculation _)* :$
Calculation <- Expression
Expression  <- Term (_ Sum)* _
Term        <- Factor (_ Product)*
Factor      <- "(" Expression ")" / Integer
Sum         <- Add / Subtract
Product     <- Multiply / Divide
Add         <- "+" :Term
Subtract    <- "-" :Term
Multiply    <- "*" :Factor
Divide      <- "/" :Factor
Integer     <- '(-?[0-9]+)'
_           <- '\s*'?
`
	const script = `
9
8+15
9*6/12
`
	// first build the parser for our language
	g, err := peg.NewGrammar(`Calculator`, language)
	if err != nil {
		fmt.Println("Language error", err)
		return
	}
	p := peg.NewParser(g)
	p.Process("Calculation", func(args ...interface{}) (interface{}, error) {
		fmt.Printf("= %v\n", args[0])
		return nil, nil
	})
	p.Process("Integer", func(args ...interface{}) (interface{}, error) {
		i, err := strconv.ParseInt(args[0].(string), 10, 64)
		return i, err
	})
	p.Process("Expression", func(args ...interface{}) (interface{}, error) {
		v := args[0].(int64)
		for _, arg := range args[1:] {
			v = arg.(func(v int64) int64)(v)
		}
		return v, nil
	})
	p.Process("Term", func(args ...interface{}) (interface{}, error) {
		v := args[0].(int64)
		for _, arg := range args[1:] {
			v = arg.(func(v int64) int64)(v)
		}
		return v, nil
	})
	p.Process("Add", func(args ...interface{}) (interface{}, error) {
		rhs := args[0].(int64)
		return func(v int64) int64 { return v + rhs }, nil
	})
	p.Process("Subtract", func(args ...interface{}) (interface{}, error) {
		rhs := args[0].(int64)
		return func(v int64) int64 { return v - rhs }, nil
	})
	p.Process("Multiply", func(args ...interface{}) (interface{}, error) {
		rhs := args[0].(int64)
		return func(v int64) int64 { return v * rhs }, nil
	})
	p.Process("Divide", func(args ...interface{}) (interface{}, error) {
		rhs := args[0].(int64)
		return func(v int64) int64 { return v / rhs }, nil
	})
	// now get ourselves a token input
	input := strings.NewReader(script)
	// now evaluate that input using the grammar
	_, err = p.Parse("input", input)
	if err != nil {
		fmt.Println("Unexpected error", err)
	}
	//Output:
	// = 9
	// = 23
	// = 4
}
