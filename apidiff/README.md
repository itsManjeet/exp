# Checking Go Package API Compatibility

The `apidiff` package in this directory determines whether two packages, or two
versions of the same package, are compatible. The goal is to help the developer
make an informed choice of semantic version after they have changed the code of
their module.

This package reports two kinds of changes: incompatible ones, which require
incrementing the major part of the semantic version, and compatible ones, which
require a minor version increment. If no API changes are reported but the
code has changed, then the patch version should be incremented.

(At present, the tool compares only packages, not modules. See "Module
Compatibility" below.)

## Compatibility Desiderata

Any tool that checks compatibility can offer only an approximation. No tool can
detect behavioral changes; and even if it could, whether a behavioral change is
a breaking change or not depends on many factors, like whether it closes a
security hole or fixes a bug. Even a change that causes some code to fail to
compile may not be considered a breaking change by the developers or their
users. It may only affect code marked as experimental or unstable, for
example, or the break may only manifest in unlikely cases.

For a tool to be useful, its notion of compatibility must be relaxed enough to
allow reasonable changes, like adding a field to a struct, but strict enough to
catch significant breaking changes. A tool that is too lax will miss important
incompatibilities, and users will stop trusting it; one that is too strict will
generate so much noise that users will ignore it.

To a first approximation, this package reports a change as incompatible if it
could cause code to stop compiling. But, following the [Go 1 Compatibility
Guarantee](https://golang.org/doc/go1compat), it will not mark a change
incompatible if the only code it would break exploits certain properties of the
language.

Here we list six ways in which code may fail to compile after a change. This
package ignores all of these. Three of them are mentioned in the Go 1
Compatibility Guarantee.

### Unkeyed Struct Literals

Code that uses an unkeyed struct literal would fail to compile if a field was
added to the struct. An example:

```
// old
type Point struct { X, Y int }

// new
type Point struct { X, Y, Z int }

// outside
p := pkg.Point{1, 2}
```
Here and below, we provide three snippets: the code in the old version of the
package, the code in the new version, and the code written outside the package,
which refers to it by the name `pkg`. The outside code compiles against the old
code but not the new.

### Embedding and Shadowing

Adding an exported field to a struct can break code that embeds that struct,
because it is an error to have two exported fields with the same name at the
same depth.

```
// old
type Point struct { X, Y int }

// new
type Point struct {X, Y, Z int }

// outside
type z struct { Z int }

struct {
    pkg.Point
    z
}
```
In the new version, the code fails to compile because there are two embedded `Z`
fields, one from `z` and one from `pkg.Point`.


### Writing the Identical Type Externally

If it is possible to write a type expression outside of its package, then
outside code can use it interchangeably with the package type, making any change
to the type an incompatible one.

```
// old
type Point struct { X, Y int }

// new
type Point struct { X, Y, Z int }

// outside
var p struct { X, Y int } = pkg.Point{}
```
Here, the outside code could have used the provided name `Point`, but chose not
to. I'll have more to say about this and related examples later.

### unsafe.Sizeof

Since `unsafe.Sizeof` is a constant expression, it can be used in an array type literal:

```
// old
type S struct{ X int }

// new
type S struct{ X, y int }

// outside
var a [unsafe.Sizeof(pkg.S{})]int = [8]int{}
```
Use of `unsafe.Sizeof` makes any change to the size of a type is an incompatible change.


### Constant Values and Arrays

An change to an integer constant value, or to the length of a string constant,
can affect the type of an array and cause an incompatibility.

```
// old
const C = 1

// new
const C = 2

// outside
var a [C]int = [1]int{}
```
A tool that reports an incompatible change for every change to a constant value
would generate too much noise to be useful, so we ignore this possibility.

### Type Switches

This package allows two different old types to be merged into a single new type.
The only code this can break is a type switch that mentions both types:

```
// old
type T1 int
type T2 int

// new
type T1 int
type T2 = T1

// outside
switch x.(type) {
case T1:
case T2:
}
```

## Some Concepts

In order to describe the operation of this package more precisely, we need some
terminology.

### Exposure

It is obvious that exported symbols have consequences for package compatibility.
But some unexported symbols may be involved as well. Consider:

```
// pkg
type u int
func (u) M() {}
var V u

// outside
V.M()
```

Although `u` can't be written outside the package, it can be used. For example,
outside code can call the `u.M` method.

A name is _exposed_ if it is exported at the package level, or reachable from an exposed name.

### Correspondence

Given an old and new version of a package, we must determine which exposed type
names correspond. Usually this is trivial: type `T` in the old package
corresponds to type `T` in the new package. But type aliases complicate the
picture. Consider:

```
// old
type E int

// new
type u int
type E = u
```
Here old `E` and new `u` correspond.

It should also be possible to rename an unexported name, even if it is exposed.

We say that two defined types _correspond_ if they appear in the same places in the old and
new APIs.

Two or more old defined types can correspond to a single new type: we consider
"merging" two types into one to be a compatible change. As mentioned above,
code that uses both names in a type switch will fail, but we deliberately ignore
this case.

(Merging is rare and we don't want to overlook incompatibilities, so arguably we
should consider it a breaking change.)

### Equivalence

We can use correspondence to define what it means for two types to be the same
across packages. The language's definition of type identity is inadequate,
because it says that two defined types are identical if they arise from the same
definition, and it's unclear what "same" means when talking about two different
packages (or two versions of a single package). Our discussion of correspondence
above demonstrates that we can't weaken this condition to something like name
equality.

The definition of type _equivalence_ parallels the language's definition of type
identity, except when comparing defined types:
1. If one type is from the old package and the other from the new, then the
types are equivalent if and only if they correspond.
2. Otherwise, the types are equivalent if and only if they have the same name and package.

The special check for old and new packages is necessary, because the assumption
of the API checker is that the old and new packages can never be used together.
This example shows an incompatibility that can result if we used correspondence
for all packages:

```
// old
type I = io.Writer

// new
type I io.Writer

// outside
var f func(io.Writer) = func(pkg.I) {}
```
If we said that `io.Writer` corresponded to `pkg.I`, we would treat the two
types as equivalent and would not report an incompatibility.


## Definition of Compatibility

We can now present the definition of compatibility used by this package.

### Package Compatibility

>A new package is compatible with an old one if each exported name in the old
package's scope also appears in the new package's scope, and the old and new
objects (const, var, func or typename) denoted by the name are compatible.
>
>Otherwise the packages are incompatible.

Here we want identity of names. Correspondence will appear later, when we get
to types.

I use the term "object" in the sense of `go/types`.

The tool also finds names in the new package that are not in the old, and
marks them as compatible changes.

### Object Compatibility

This section provides compatibility rules for constants, variables, functions
and typenames.

Any pair of old and new objects not described below is an incompatible one.

#### Constants

>A new exported constant is compatible with an old one of the same name if
>1. Their types are equivalent, or the new type is an untyped form of the old, and
>2. If the old value is representable by a value of type T, so is the new one.


The first clause permits removing a type from a constant declaration. That is
almost always harmless, but it can break code like

```
const C int = 1 // old
const C = 1     // new

// outside
var s uint = 33
float32(pkg.C << s)
```
This code is rare, but so is un-typing a constant, so perhaps we should
consider doing so an incompatible change.

We said above that it would be too noisy to report an incompatibility
whenever an integer constant's value changed. But it seems reasonable to report
value changes that can break assignments like `var x uint8 = pkg.C`. If we tried to
catch all such breakages, we would be back to forbidding any change to a constant value
(e.g. `var x uint8 = pkg.C + 254` breaks if `C` changes from 1 to 2).

#### Variables

>A new exported variable is compatible with an old one of the same name if their types are
>equivalent.

Remember that equivalence stops at names, so this rule does not prevent adding a
field to `MyStruct` if the package declares `var V MyStruct`. It does, however, mean that

```
var V struct { X int }
```
is incompatible with
```
var V struct { X, Y int }
```
I discuss this at length below the the section "Compatibility and Named Types."

#### Functions

>A new exported function or variable is compatible with an old function of the
>same name if their types are equivalent.

This rule captures the fact that, although many signature changes are compatible
for all call sites, none are compatible for assignment:

```
var v func(int) = pkg.F
```
Here, `F` must be of type `func(int)` and not, for instance, `func(...int)`.

Note that the rule permits changing a function to a variable. This is a common
practice, usually done for test stubbing, and cannot break any code at compile
time.

#### Typenames

> A new exported typename is compatible with an old one of the same name if
> their types are equivalent.

This rule seems far too strict. But remember that a typename is the name of a
type; it is not to be confused with a named (or defined) type. In the absence of
aliases, every typename's type was a defined type, and this rule implies only
that those defined types correspond.

Consider:
```
// old
type T struct { X int }

// new
type T struct { X, Y int }
```
This rule does not imply that the struct literals have to be equivalent, only
that the defined types denoted by `T` must be equivalent (that is, correspond).

If one typename is an alias that refers to the corresponding defined type, the
situation is the same:

```
// old
type T struct { X int }

// new
type u struct { X, Y int }
type T = u
```
Here, provided old `T` corresponds to new `u`, the types of the `T`s are
equivalent.

However, the following change is incompatible, because the typenames do not denote equivalent
types:
```
// old
type T = struct { X int }

// new
type T = struct { X, Y int }
```
### Type Compatibility

Only four kinds of types can differ compatibly: defined types, structs,
interfaces and channels. We only consider the compatibility of the last three
when they are the underlying type of a defined type. See "Compatibility and
Named Types" for a rationale.

> A new type is compatible with an old if and only if they are equivalent, or
> one of the cases below applies.

#### Defined Types

> A new defined type is compatible with an old one if all of the following hold:
>1. They correspond.
>2. Their underlying types are compatible.
>3. The new exported value method set is a superset of the old.
>4. The new exported pointer method set is a superset of the old.

An exported method set is a method set with all unexported methods removed.
When comparing methods of a method set, we require identical names and
equivalent signatures.

Removing an exported method is clearly a breaking change. But removing an
unexported one (or changing its signature) can be breaking as well, if it
results in the type no longer implementing an interface. See "Whole-Package
Compatibility," below.

#### Channels

> A new channel type is compatible with an old one if
>  1. The element types are equivalent, and
>  2. Either the directions are the same, or the new type has no direction.

Removing a named channel's direction is an unlikely change, but a harmless one.

#### Interfaces

> A new interface is compatible with an old one if the interface has an
>  unexported method and the new exported method set is a superset of the old.

If an interface has an unexported method, it can only be embedded, so it is safe
to add methods.

If the old interface does not have an unexported method, then you cannot add
one, unexported or not:

```
type I interface { M1() }         // old
type I interface { M1(); M2() }   // new

// outside
type t struct{}
func (t) M1() {}
var i pkg.I = t{}
```
The two interfaces must be equivalent. This case is
captured by the equivalence clause at the beginning of the "Type Compatibility" section.

#### Structs
 
> A new struct is compatible with an old one if all of the following hold:
> 1. The new set of exported fields is a superset of the old.
> 2. The new set of _selectable_ exported fields is a superset of the old.
> 3. If the old struct is comparable, so is the new one.

The set of exported fields refers to the fields immediately defined in the
struct. The set of exported selectable fields is the set of exported fields `F`
such that `x.F` is a valid selector expression for a value `x` of the struct
type. Two fields are the same if they have the same name and equivalent types.

The first clause ensures that struct literals compile; the second, that
selections compile; and the third, that equality expressions and map index
expressions compile.

## Whole-Package Compatibility

The above definition does not consider incompatible changes that involve more
than one type in the package. For example, if you remove an unexported method on
a defined type, it may no longer implement an interface of the package. This can
break outside code:

```
// old
type T int
func (T) m() {}
type I interface { m() }

// new
// same, but without T.m

// outside
var i pkg.I = pkg.T{}
```

Conversely, adding a method to an interface can cause defined types
in the package to stop implementing it.

The definition for package compatibility should include something like the following:

> Every exposed type that implemented an exposed interface in the old package
> should also do so in the new package.

I haven't yet implemented this check.

# Module Compatibility

This tool checks the compatibility of packages, but we ultimately want to check
the compatibility of modules.

I don't have an implementation, but I believe this is the correct definition:

>A new module is compatible with an old one if, for every non-internal,
>non-vendored package in the old module, there is a package in the new module
>with the same import path that is compatible with it.

## Compatibility and Named Types

The above definition of package compatibility states that only types with names
can differ compatibly. The definition permits adding a field to the struct in

```
type T struct { X int }
```
but not
```
var V struct { X int }
```

One reason is that in the second case, outside code could write

```
var v struct { X int } = pkg.V
```
effectively forcing the type of `V` to remain unchanged. But this rationale
is weak, because as we saw, the same can be done for defined types:

```
var x struct { X int } = pkg.T{}
```
You could argue that in the first case, the package provided a name that the
outside developer churlishly refused to use, while in the second case there was
no choice. But that doesn't explain why the definition also disallows changes to
```
type T = struct { X int }
```
The rationale also fails in cases like

```
var V struct { x int }
```
Here, outside code cannot write the type literal, so it cannot do the equivalent
of

```
var v struct { X int } = pkg.V
```
Yet the definition above still disallows any changes.

And then there is the problem that we allow changes to `t` in

```
type t struct { X int }
var V t
```
even though an outside developer would have no choice but to copy the type
literal if they wanted to use `V` more flexibly.

The following alternative rule covers all of these cases:

> A type can be changed compatibly if and only if it has an exported name
> (whether by alias or not), or it cannot be written externally.

That definition would be more generous to package developers, permitting
backwards-compatible changes that the current one does not.

My opinion is that on balance, this change wouldn't be worth the complexity it
would add.

First, the two rules differ only in rare cases. Writing a struct or interface
literal as the type of a variable or as part of a function signature is
uncommon, and (I suspect) so is writing an alias for one. Writing channel type
literals as part of the API is common, but not with unexported element types. So
the extra freedom that the proposed rule grants to package developers is largely
theoretical.

Second, supporting compatible changes to type literals makes it more likely to
introduce incompatibilities across types in the same package. If one were to
write the same literal twice, but change only one instance, an outside developer
could write breaking code:

```
// old
var V1 struct { X, u int }
var V2 struct { X, u int }

// new
var V1 struct { X, u int }
var V2 struct { X, Y, u int }

// outside
V1 = V2
```
With variables, the package programmer could have written a named type in
the first place, or placed both variables in the same declaration. But the
problem is unavoidable with functions, since Go function signatures must be
spelled out in the function declaration:

```
// old
func F(struct { X, u int })
func G(struct { X, u int })

// new
func F(struct { X, u int })
func G(struct { X, Y, u int })

// outside
f := pkg.F
f = pkg.G
```

To report this incompatibility, the API checker would have to verify that all
identical type literals in the package were changed in the same way. 

This also opens the door to call compatibility. If the tool allows the above
change to G's signature because it cannot break outside calls to G, then for
consistency it should also allow call-compatible changes to any signature that
cannot be written outside the package. For example:

```
// old
type u int
func H(u)

// new #1
type u int
func H(...u)

// new #2
type u int
func H(u, ...string)

// new #3? (may affect shifts?)
func H(interface{})
```
These examples suggest that allowing call-compatible changes to a signature
would add significant complexity to the description and implementation of the
tool.
