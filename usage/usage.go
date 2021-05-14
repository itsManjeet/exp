// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/* Package help implements command line flag parsing based on the help text.

To use help you write clear consistent help text and a struct to hold the
results and then hand them both along with your command line arguments to the
Process function.

It is intended to match the standard go way of doing flags (and uses the flag
package underneath to ensure this remains true) but by starting from the help
text and directly filling in a struct it ensures a coherent system.
Help text is better maintained and clean. All help using this system is in a
consistent style. Command line processing is separated from application logic.
Testing your command line and maintaining compatability is easy. More complex
patterns are easy to specify. Errors are more consistently detected and
reported.

help is similar in intent to the docopt system, except that it matches the go
style of command line rather than the python one, and is more opinionated about
the correct style of the help text. See the naval example for an equivalent to
the standard docopt example in help style.

Process proceeds in 5 main phases.
  Compile: Parse the help text pages and compile them into a usage pattern.
  Scan: Scan the options struct for fields that can be set from the command line.
  Bind: Bind the positionals and flags from the usage to the fields found.
  Match: Match the usage pattern against the command line arguments to pick out values.
  Apply: Set the fields using the values found.

Compile:

The help text is supplied as a collection of pages, where each page is intended
to be independently printed.
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

The patterns themselves have a formal langauge, declared in the PEG constant.

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
flag if it was allowed but not present. It is preceded by [default: and
terminated by ], which means you cannot have a closing square bracket in a
default value.
  [default: a string value]


Scan:

The options parameter provided to the Process function must be a pointer to a
struct. The scanning phase walks the fields of that struct recursively looking
for types that it knows how to bind to flags.
During this phase the names found during the compile phase are not considered
at all, every field is considered to be a valid one.
All the primitive types can be bound, values of flag.Flag are a also used.
Fields of type []string are handled specially, where each successive matching
value appends to the slice. Pointers and function fields are skipped, all other
field types are an error.
When recursing into nested structs, the parents name is joined to the child name
by a period, for the purpose of matching to pattern names. If the struct was
embedded however this does not happen.


Bind:

This phase attempts to connect the names discovered during compile to the fields
found during scan.
Names are compared using the alphanumeric runes only, and ascii letters are
compared in a case insensitive way.
This allows names in varying styles to still match, so for instance "my-flag"
as used as a flag name would match "MyFlag" used as a field name.


Match:

The primary usage expression is the expression of a section called "usage".
This is matched against the supplied args during this phase.
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


Apply:

All the values found during the match phase are then applied to the options
struct once matching is complete, in they order they were discovered.
This ensures that the options struct never sees any values other than the final
selected ones.

*/
package usage

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"unicode"

	"golang.org/x/exp/peg"
)

// Text declares the help system for an application.
type Text struct {
	// Pages is the full set of help pages.
	Pages []Page
	// Map holds field remapping declarations.
	// These map from a non existant field name to a real field, and are used to
	// maintain backwards compatability when flags or fields need to be renamed.
	Map map[string]string
}

// Page is a single page of output in the help system.
// Each page can be printed separately, and the first page should be the
// default entry point to the help.
type Page struct {
	// The name of the page.
	Name string
	// The raw printable help text for the page.
	Content []byte
}

// state maintains the internal state of a call to Process.
type state struct {
	Text       Text       // the raw help text pages
	Parser     peg.Parser // the parser compiled from the help text
	Fields     []*field   // the fields discovered from the options
	Remaps     []remap    // the processed field remappings
	Bindings   []*binding // the bindings from the parser to the fields
	Positional []string   // the positional arguments after flag parsing
	ReadPos    int
	Present    map[*binding]struct{}
	Allowed    map[*binding]struct{}
	Results    []result
}

// nameSet is a collection of names.
// The names are stored in a slice to preserve order, and support testing
// whether a name is already present.
type nameSet []name

// name is a string with complex comparison rules.
type name struct {
	print string
	match string
}

// result captures a value with the binding it applies to.
type result struct {
	binding *binding
	value   string
}

// bindingFlag implements flag.Value so that calls to Set capture the value
// into a result list.
// This is used when processing a command line for flags to capture all the
// flags without applying them so that the patterns can be tested.
type bindingFlag struct {
	name    name
	binding *binding
	state   *state
}

// Process takes the help text declaration and a structure of options, combines
// them to form a flag set and then uses the flag package to parse the args with
// that set. The options struct will be updated with the results.
func Process(text Text, options interface{}, args []string) error {
	state := &state{Text: text}
	// compile the help text to the command line parser
	if err := compile(state); err != nil {
		return err
	}
	// find the set of fields from the options struct
	if options == nil {
		return fmt.Errorf("no options struct")
	}
	if err := scan(state, options); err != nil {
		return err
	}
	// bind the parser parameters to the discovered fields
	if err := bind(state); err != nil {
		return err
	}
	// use the generated parser to match against the supplied args
	err := match(state, args)
	// attempt to apply the results even on failure
	// apply all the flag results back to the fields, even when failing
	for _, result := range state.Results {
		result.binding.field.flag.Set(result.value)
	}
	return err
}

func match(state *state, args []string) error {
	state.Present = map[*binding]struct{}{}
	state.Allowed = map[*binding]struct{}{}

	// parse the command line arguments picking out the flags
	positional, err := parseFlags(state, args)
	if err != nil {
		return err
	}
	// attempt to apply the best match even in the presence of previous errors
	// this allows the caller to do a better job of explaining the error to the user
	// we have to track the first error seen to do this
	var firstError error
	//build the map of flags we have seen
	for _, r := range state.Results {
		state.Present[r.binding] = struct{}{}
	}

	// now match the args against the discovered patterns
	state.Positional = positional
	state.ReadPos = 0
	value, err := state.Parser.Parse("command-line", state)
	if err != nil && firstError == nil {
		firstError = err
	}
	// convert the match to a full result set
	err = toResults(state, value)
	if err != nil && firstError == nil {
		firstError = err
	}
	// now check we do not have any flags left that do not match the pattern
	if firstError == nil {
		for b := range state.Present {
			if _, ok := state.Allowed[b]; !ok {
				firstError = fmt.Errorf("flag %v present but not allowed", b)
			}
		}
	}
	// also mark the positionals as present
	for _, r := range state.Results {
		state.Present[r.binding] = struct{}{}
	}
	// apply the defaults for flags that were allowed but not present
	for _, b := range state.Bindings {
		//skip defaults that were not in the allowed set
		if _, ok := state.Allowed[b]; !ok {
			continue
		}
		// don't set a default if we have a real value
		if _, ok := state.Present[b]; ok {
			continue
		}
		if len(b.defaults) > 0 {
			for _, d := range b.defaults {
				b.field.flag.Set(d)
			}
		}
	}
	return firstError
}

// parseFlags parses all the flags from the args, returning the remaining
// positional args.
func parseFlags(state *state, args []string) ([]string, error) {
	// first build the flagset
	flagset := flag.NewFlagSet("", flag.ContinueOnError)
	flagset.SetOutput(ioutil.Discard)
	for _, b := range state.Bindings {
		seen := map[string]struct{}{}
		for _, name := range b.flags {
			if _, ok := seen[name.print]; ok {
				continue
			}
			seen[name.print] = struct{}{}
			flagset.Var(&bindingFlag{name: name, binding: b, state: state}, name.print, "")
		}
	}

	// in order to allow flags to move around the verbs, we repeat the parse
	// until we are confident there are no more flags
	positional := []string{}
	for len(args) > 0 {
		switch {
		case args[0] == "--":
			//accept -- as a terminator for this process, to allow flag like
			//positional arguments when needed.
			positional = append(positional, args[1:]...)
			args = nil
		case strings.HasPrefix(args[0], "-"):
			// first arg is a flag, parse to remove all flags from the front
			if err := flagset.Parse(args); err != nil {
				return positional, err
			}
			args = flagset.Args()
		default:
			// first arg is a positional, remove it in case there are more flags
			positional = append(positional, args[0])
			args = args[1:]
		}
	}
	return positional, nil
}

// toResults converts from the output of matching the compiled grammar to
// results stored in the state.
func toResults(state *state, value interface{}) error {
	switch value := value.(type) {
	case result:
		state.Results = append(state.Results, value)
	case []interface{}:
		for _, v := range value {
			switch v := v.(type) {
			case result:
				state.Results = append(state.Results, v)
			case *flagDecl:
				state.Allowed[v.binding] = struct{}{}
			case flagSet:
				for _, e := range v {
					f := e.(*flagDecl)
					state.Allowed[f.binding] = struct{}{}
				}
			default:
				return fmt.Errorf("invalid result type %T", v)
			}
		}
	default:
		return fmt.Errorf("invalid result type %T", value)
	}
	return nil
}

func (b *bindingFlag) String() string {
	return b.name.String()
}

func (b *bindingFlag) Set(v string) error {
	b.state.Results = append(b.state.Results, result{
		binding: b.binding,
		value:   v,
	})
	return nil
}

func (b *bindingFlag) IsBoolFlag() bool {
	return isBoolField(b.binding.field)
}

func isBoolField(f *field) bool {
	if t, ok := f.flag.(interface{ IsBoolFlag() bool }); ok {
		return t.IsBoolFlag()
	}
	return false
}

// stringList is an implementation of flag.Value that supports lists of strings
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

func toName(text string) name {
	return name{print: text, match: canonicalName(text)}
}

func canonicalName(value string) string {
	return strings.Map(func(r rune) rune {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, value)
}

func (n name) Equivalent(other name) bool {
	return n.match == other.match
}

func (n name) String() string {
	return n.print
}

func (n name) IsValid() bool {
	return n.match != ""
}

func (n name) Extend(suffix string) name {
	c := canonicalName(suffix)
	if n.IsValid() {
		n.print += n.print + "."
	}
	n.print += c
	n.match += c
	return n
}

func (s nameSet) Contains(n name) bool {
	for _, check := range s {
		if check.Equivalent(n) {
			return true
		}
	}
	return false
}

func (t *Text) WriteSection(w io.Writer, name string) error {
	for _, p := range t.Pages {
		if p.Name == name {
			_, err := w.Write(p.Content)
			return err
		}
	}
	return fmt.Errorf("no help section %q", name)
}

func (s *state) ReadRune() (rune, int, error) { panic("never called") }

func (s *state) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		s.ReadPos = int(offset)
	case io.SeekCurrent:
		s.ReadPos += int(offset)
	case io.SeekEnd:
		return 0, errors.New("seek from end not supported")
	default:
		return 0, errors.New("invalid whence")
	}
	return int64(s.ReadPos), nil
}
