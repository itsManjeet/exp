# Checking Go Package API Compatibility

The `apidiff` package in this directory determines whether two packages, or two
versions of the same package, are compatible. The goal is to help the developer
make an informed choice of semantic version after they have changed the code of
their module.

This tool reports two kinds of changes: incompatible ones, which require
incrementing the major part of the semantic version, and compatible ones, which
require a minor version increment. If no API changes are reported but the
code has changed, then the patch version should be incremented.

(At present, the tool compares only packages, not modules. See "Module
Compatibility" below.)

## Compatibility Desiderata

Any tool that checks compatibility can offer only an approximation. No tool can
detect behavioral changes; and even if it could, whether a behavioral change is
a breaking change or not depends on many factors, such as whether it closes a
security hole or fixes a bug. Even a change that causes some code to fail to
compile may not be considered a breaking change by the developers or their
users. It may only affect code marked as experimental or unstable, for
example, or the break may only manifest in unlikely cases.

For a tool to be useful, its notion of compatibility must be relaxed enough to
allow reasonable changes, like adding a field to a struct, but strict enough to
catch significant breaking changes. A tool that is too lax will miss important
incompatibilities, and users will stop trusting it; one that is too strict will
generate so much noise that users will ignore it.

To a first approximation, this tool reports a change as incompatible if it
could cause code to stop compiling. But, following the [Go 1 Compatibility
Guarantee](https://golang.org/doc/go1compat), it will not mark a change
incompatible if the only code it would break exploits certain properties of the
language.

Here we list six ways in which code may fail to compile after a change. This
tool ignores all of these. Three of them are mentioned in the Go 1
Compatibility Guarantee.

### Unkeyed Struct Literals

Code that uses an unkeyed struct literal would fail to compile if a field was
added to the struct, making any such addition an incompatible change. An example:

```
// old
type Point struct { X, Y int }

// new
type Point struct { X, Y, Z int }

// client
p := pkg.Point{1, 2}
```
Here and below, we provide three snippets: the code in the old version of the
package, the code in the new version, and the code written in a client of the package,
which refers to it by the name `pkg`. The client code compiles against the old
code but not the new.

### Embedding and Shadowing

Adding an exported field to a struct can break code that embeds that struct,
because the newly added field may conflict with an identically named field
at the same struct depth. A selector referring to the latter would become
ambiguous and thus erroneous.


```
// old
type Point struct { X, Y int }

// new
type Point struct { X, Y, Z int }

// client
type z struct { Z int }

var v struct {
    pkg.Point
    z
}

_ = v.Z
```
In the new version, the last line fails to compile because there are two embedded `Z`
fields, one from `z` and one from `pkg.Point`.


### Writing the Identical Type Externally

If it is possible for client code to write a type expression that is the
underlying type of a defined type in a package, then external code can use it in
assignments involving the package type, making any change to the type an
incompatible one.

```
// old
type Point struct { X, Y int }

// new
type Point struct { X, Y, Z int }

// client
var p struct { X, Y int } = pkg.Point{}
```
Here, the external code could have used the provided name `Point`, but chose not
to. I'll have more to say about this and related examples later.

### unsafe.Sizeof and Others

Since `unsafe.Sizeof`, `unsafe.Offsetof` and `unsafe.Alignof` are a constant
expressions, the can be used in an array type literal:

```
// old
type S struct{ X int }

// new
type S struct{ X, y int }

// client
var a [unsafe.Sizeof(pkg.S{})]int = [8]int{}
```
Use of these operations would make many changes to a type potentially incompatible.


### Constant Values and Arrays

A change to a numeric constant value, or to the length of a string constant,
can affect the type of an array and cause an incompatibility.

```
// old
const C = 1

// new
const C = 2

// client
var a [C]int = [1]int{}
```
A tool that reports an incompatible change for every change to a constant value
would generate too much noise to be useful, so we ignore this possibility.

### Type Switches

A package change that merges two different types (with same underlying type)
into a single new type may break type switches in clients that refer to both
original types:

```
// old
type T1 int
type T2 int

// new
type T1 int
type T2 = T1

// client
switch x.(type) {
case T1:
case T2:
}
```
This sort of incompatibility is sufficiently esoteric to ignore; the tool allows
merging types.



## Some Concepts

In order to describe the operation of this tool more precisely, we need some
terminology.

### Exposure

It is obvious that exported symbols have consequences for package compatibility.
But some unexported symbols may be involved as well. Consider:

```
// pkg
type t int
func (t) M() {}
var V t

// client
V.M()
```

Although `t` can't be written client the package, it can be used. For example,
client code can call the `t.M` method via the exported variable `V`.

A name is _exposed_ if it is exported at the package level, or reachable from an exposed name.

### Correspondence

As part of determining compatibility, a tool needs to decide whether an old and
new type are the same or not. The Go spec has a definition of type identity, but
it isn't adequate for this purpose, because it says that two defined types are
identical if they arise from the same definition, and it's unclear what "same"
means when talking about two different packages (or two versions of a single
package).

The obvious change to the definition of identity is to require that old and new defined
types have the same name instead. But that doesn't work either, for two reasons.
First, type aliases can equate two types with different names:

```
// old
type E int

// new
type t int
type E = t
```
Second, an unexported but exposed type can be renamed:

```
// old
type u1 int
var V u1

// new
type u2 int
var V u2
```
We will say that two defined types _correspond_ if they are used in the same places
in the old and new APIs. In the first example, old `E` and new `t` correspond.
In the second, old `u1` and new `u2` correspond.

Two or more old defined types can correspond to a single new type: we consider
"merging" two types into one to be a compatible change. As mentioned above,
code that uses both names in a type switch will fail, but we deliberately ignore
this case.

(Merging is rare and we don't want to overlook incompatibilities, so arguably we
should consider it a breaking change.)

### Equivalence

We can use correspondence to define what it means for two types to be the same
across packages. Roughly speaking, two types are equivalent if they are
identical, up to correspondence.

More precisely, the definition of type _equivalence_ parallels the language's
definition of type identity, except when comparing defined types:
1. If one type is from the old package and the other from the new, then the
types are equivalent if and only if they correspond.
2. Otherwise, the types are equivalent if and only if they have the same name and package.

The special check in clause 1 for old and new packages is necessary, because the
assumption of the API checker is that the old and new packages can never be used
together. This example shows an incompatibility that can result if we used
correspondence for all packages:

```
// old
type I = io.Writer

// new
type I io.Writer

// client
var f func(io.Writer) = func(pkg.I) {}
```
If we said that `io.Writer` corresponded to `pkg.I`, we would treat the two
types as equivalent and would not report an incompatibility.


## Definition of Compatibility

We can now present the definition of compatibility used by this tool.

### Package Compatibility

> A new package is compatible with an old one if:
1. Each exported name in the old
package's scope also appears in the new package's scope, and the old and new
objects (const, var, func or typename) denoted by the name are compatible; and
2. For every exposed type that implements an exposed interface in the old package,
> its corresponding type should implement the corresponding interface in the new package.
>
>Otherwise the packages are incompatible.

In clause 1 we want identity of names, not correspondence.

I use the term "object" in the sense of `go/types`: a named entity created by a declaration.

The tool also finds exported names in the new package that are not exported in the old, and
marks them as compatible changes.

Like any tool, the checker has to pick a configuration when loading packages.
Changes to GOOS and GARCH, as well as the presence of build tags, may affect the
configuration.

Clause 2 is discussed further in "Whole-Package Compatibility."

### Object Compatibility

This section provides compatibility rules for constants, variables, functions
and typenames.

Any pair of old and new objects not described below is an incompatible one.

#### Constants

>A new exported constant is compatible with an old one of the same name if
>1. Their types are equivalent, and
>2. If the old value is representable by a value of type T, so is the new one.

It is tempting to allow changing a typed constant to an untyped one. That may
seem harmless, but it can break code like this:

```
// old
const C int64 = 1

// new
const C = 1

// client
x := C // old type is int64, new is int
var y int64 = x
```

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
I discuss this at length below in the section "Compatibility and Named Types."

#### Functions

>A new exported function or variable is compatible with an old function of the
>same name if their types are equivalent.

This rule captures the fact that, although many signature changes are compatible
for all call sites, none are compatible for assignment:

```
var v func(int) = pkg.F
```
Here, `F` must be of type `func(int)` and not, for instance, `func(...int)` or `func(interface{})`.

Note that the rule permits changing a function to a variable. This is a common
practice, usually done for test stubbing, and cannot break any code at compile
time.

#### Typenames

> A new exported typename is compatible with an old one of the same name if
> their types are equivalent.

This rule seems far too strict. But remember that a typename is the name of a
type; it is not to be confused with a defined type. In the absence of
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

Removing a named channel's direction is an unlikely change, but almost certainly
a harmless one.

#### Interfaces

> A new interface is compatible with an old one if:
> 1. The interface has an unexported method and the new exported method set is a
>    superset of the old, or
> 2. The interface does not have an unexported method, and the old and new
>    interfaces are equivalent (have the same method set).

If an interface has an unexported method, it can only be embedded, so it is safe
to add methods.

If the old interface does not have an unexported method, then you cannot add
one, unexported or not:

```
type I interface { M1() }         // old
type I interface { M1(); M2() }   // new

// client
type t struct{}
func (t) M1() {}
var i pkg.I = t{}
```


#### Structs

> A new struct is compatible with an old one if all of the following hold:
> 1. The new set of exported fields is a superset of the old.
> 2. The new set of _selectable_ exported fields is a superset of the old.
> 3. If the old struct is comparable, so is the new one.

The set of exported fields refers to the fields immediately defined in the
struct.

The set of exported selectable fields is the set of exported fields `F`
such that `x.F` is a valid selector expression for a value `x` of the struct
type.

Two fields are the same if they have the same name and equivalent types.

The first clause ensures that struct literals compile; the second, that
selections compile; and the third, that equality expressions and map index
expressions compile.

## Whole-Package Compatibility

Some changes that are compatible for a single type are not compatible when the
package is considered as a whole. For example, if you remove an unexported
method on a defined type, it may no longer implement an interface of the
package. This can break client code:

```
// old
type T int
func (T) m() {}
type I interface { m() }

// new
type T int // no method m anymore

// client
var i pkg.I = pkg.T{}
```

Similarly, adding a method to an interface can cause defined types
in the package to stop implementing it.

The second clause in the definition for package compatibility handles these cases.

Other incompatibilities that involve more than one type in the package can arise
whenever two identical types exist in the old or new package. Here, a change
"splits" an identical type into two, breaking assignments:

```
// old
type B struct { X int }
type C struct { X int }

// new
type B struct { X int }
type C struct { X, Y int }

// client
var b B
var c C
b = c
```
Finally, changes that are compatible for the package in which they occur can
break downstream packages. That can happen even if they involve unexported
methods, thanks to embedding.

The definition given here doesn't account for these sorts of problems.


# Module Compatibility

This tool checks the compatibility of packages, but we ultimately want to check
the compatibility of modules.

I don't have an implementation, but I believe this is the correct definition:

>A new module is compatible with an old one if and only if, for every non-internal,
>non-vendored package in the old module, there is a package in the new module
>with the same import path that is compatible with it.

## Compatibility and Named Types

The above definition of package compatibility states that only types with names
can differ compatibly. The definition permits adding a field to the struct in

```
type T struct { X int }
```
but not in
```
var V struct { X int }
```

One reason is that in the second case, client code could write

```
var v struct { X int } = pkg.V
```
effectively forcing the type of `V` to remain unchanged. But this rationale
is weak, because as we saw, the same can be done for defined types:

```
var x struct { X int } = pkg.T{}
```
You could argue that in the first case, the package provided a name that the
client developer churlishly refused to use, while in the second case there was
no choice. But that doesn't explain why the definition also disallows changes to
```
type T = struct { X int }
```
The rationale also fails in cases like

```
var V struct { x int }
```
Here, client code cannot write the type literal, so it cannot do the equivalent
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
even though an client developer would have no choice but to copy the type
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
write the same literal twice, but change only one instance, an client developer
could write breaking code:

```
// old
var V1 struct { X, u int }
var V2 struct { X, u int }

// new
var V1 struct { X, u int }
var V2 struct { X, Y, u int }

// client
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

// client
f := pkg.F
f = pkg.G
```

To report this incompatibility, the API checker would have to verify that all
identical type literals in the package were changed in the same way.

This also opens the door to call compatibility. If the tool allows the above
change to G's signature because it cannot break client calls to G, then for
consistency it should also allow call-compatible changes to any signature that
cannot be written client the package. For example:

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
