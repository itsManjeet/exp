// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/* Package usage implements command line flag parsing based on the usage text.

To use usage you write clear consistent help text, and then pass it along with
a set of output values and your command line arguments to the Process function.

The expected way to build a set of help pages is to write the plain text files
and then use embed to build them into your binary. You can use the Load function
to process the embedded file system into a Page slice. It will add a page for
every file found, and the pages name will be the base name of the file.

To make a Fields object for the output you can use FieldsOf and pass it a
pointer to a struct. FieldsOf walks the fields of that struct recursively
looking for types that it knows how to bind to flags.
All the primitive types can be bound, values of flag.Flag are a also used.
Fields of type []string are handled specially, where each successive matching
value appends to the slice. Pointers and function fields are skipped, all other
field types are an error.
When recursing into nested structs, the parents name is joined to the child name
by a period, for the purpose of matching to pattern names. If the struct was
embedded however this does not happen.

Process takes these and attempts to bind them together. First it compiles the
usage text, and then binds all the productions found to the named fields.
It prefers perfect matches of names, but will fall back to case insensitive
comparisons using only alphanumeric characters if one is not found.
This allows names in varying styles to still match, so for instance "my-flag"
as used as a flag name would match "MyFlag" used as a field name.

The primary usage expression is the expression of a section called "usage".
This is matched against the supplied args.
The flags are parsed using the standard library flag package repeatedly.
This process is terminated when a -- argument is found, all arguments after
that are considered to be positional.
This process allows flags to be moved around the command line no matter where
they appear in the original pattern. Doing this does require that a flag name
has a single meaning across all patterns.
Once all flags have been extracted, the remaining list of arguments is processed
by applying the pattern expression to match the positional arguments against
values and literals. Once a full path through the pattern has been selected,
the flags found are verified to be allowed by that pattern, and any flags that
were allowed but not present are checked for potential default values that
should be bound.

All the values found are then applied to the fields, in they order they were
discovered.
This ensures that the fields never see any values other than the final
selected ones.


usage is similar in intent to the docopt system, except that it matches the go
style of command line rather than the python one, and is more opinionated about
the correct style of the help text. See the naval example for an equivalent to
the standard docopt example in help style.

Help language

Each page can be broken into sections separated by a blank line, and each
section can have a title and a set of patterns. A section title is a single name
followed by a colon that occurs at the very start of the line.
Within a section, any line that starts with exactly two spaces is a pattern line
and the pattern is terminated by either the end of the line or two or more
spaces. All other text is considered to be a comment, and is processed only for
picking out default values.
As we compile the pages sections with the same title are merged as if they had
been written in a single page, the intent is that pages are a presentation
choice rather than something that affects the pattern itself.

The patterns themselves have a fairly strict formal langauge.

A sequence is a space separated list of expressions
    expression expression
A mutually exclusive expression is a pipe separated list of expressions, where
only one of the expressions is matched.
    thing | another
An optional expression is surrounded by brackets, the expression can
occur 0 or 1 times
    [optional]
A repeated expression is followed by an ellipsis, the expression can
occur 1 or more times
    repeatable...
A group is an expresssion surrounded by parens, and is used when the expression
would otherwise be ambiguous (for instance to group a choice within a sequence)
    (grouped)
A flag expression has a collection of flag names separated by commas where each
flag name must start with a hyphen. It is optionally followed by a parameter
name.
    -flag,-alias=param
A positional value is a name surrounded by angle brackets, it matches any single
command line argument and maps it to a field that matches the name.
    <name>
A literal value is a name with no decoration, it matches only command line
arguments with the value of the name, and maps it to a field of the same name.
Literal values have a special case however, if they have the same name as a
help section, they are instead replaced by the pattern for that section.
A section pattern is the collection of all the patterns from that selection
treated as a mutually exclusive pattern.
    name

The one extra expression does not occur during normal pattern processing, it is
the default value. This occurs only in comment text, and is associated with the
most recently parsed flag expresssion. It specifies a value to use for that
flag if it was allowed but not present. It is preceded by (default and
terminated by ), which means you cannot have a closing brace in a default value.
  (default "a string value")

*/
package usage

import (
	"strings"
	"unicode"
)

// Page is a single page of output in the usage system.
// Pages are intended to be printed separately, but are considered as a single
// document when compiling to the grammar.
type Page struct {
	// The name of the page.
	Name string
	// The filepath of the page, used when printing parse errors.
	Path string
	// The raw printable help text for the page.
	Content string
}

// Pages is the type for a set of usage pages that together make up the help
// text for an application.
type Pages []Page

func (b *bindings) Process(args []string) error {
	// use the generated parser to match against the supplied args
	results, matchErr := match(b.grammar, args)
	// attempt to apply the results even if match failed
	// this allows the caller to do a better job of explaining the error to the user
	// we have to track the first error seen to do this
	// and apply the results to those bindings
	applyErr := b.apply(results)
	//now we finally check the match error, because that supercedes any apply error
	if matchErr != nil {
		return matchErr
	}
	return applyErr
}

type sname struct {
	Full   string
	Simple string
}

func makeSName(full string) sname {
	return sname{
		Full:   full,
		Simple: strings.Map(simpleLetters, full),
	}
}

func (n sname) String() string { return n.Full }

func simpleLetters(r rune) rune {
	if unicode.IsLetter(r) {
		return unicode.ToLower(r)
	}
	if unicode.IsDigit(r) {
		return r
	}
	return -1
}

// stringList is an implementation of flag.Value that supports lists of strings.
type stringList struct {
	list *[]string
}

func (f stringList) Set(v string) error {
	*(f.list) = append(*(f.list), v)
	return nil
}

func (f stringList) String() string {
	return strings.Join(*(f.list), ",")
}
