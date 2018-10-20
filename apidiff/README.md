# Checking Go Package API Compatibility

The `apidiff` tool in this directory determines whether two versions of the same
package are compatible. The goal is to help the developer make an informed
choice of semantic version after they have changed the code of their module.

`apidiff` reports two kinds of changes: incompatible ones, which require
incrementing the major part of the semantic version, and compatible ones, which
require a minor version increment. If no API changes are reported but the
code has changed, then the patch version should be incremented.

The tool ignores a package's import path in determining API compatibility, so it
can also display the API changes between two packages. For instance, it can be
used to see what has changed between two major versions of a package (e.g.
`golang.org/x/oauth` and `golang.org/x/oauth/v2`), or between two forks of a
package.

The current version of `apidiff` compares only packages, not modules.


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
p := pkg.Point{1, 2} // fails in new because there are more fields than expressions
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

_ = v.Z // fails in new
```
In the new version, the last line fails to compile because there are two embedded `Z`
fields at the same depth, one from `z` and one from `pkg.Point`.


### Using an Identical Type Externally

If it is possible for client code to write a type expression representing the
underlying type of a defined type in a package, then external code can use it in
assignments involving the package type, making any change to that type incompatible.
```
// old
type Point struct { X, Y int }

// new
type Point struct { X, Y, Z int }

// client
var p struct { X, Y int } = pkg.Point{} // fails in new because of Point's extra field
```
Here, the external code could have used the provided name `Point`, but chose not
to. I'll have more to say about this and related examples later.

### unsafe.Sizeof and Friends

Since `unsafe.Sizeof`, `unsafe.Offsetof` and `unsafe.Alignof` are a constant
expressions, they can be used in an array type literal:

```
// old
type S struct{ X int }

// new
type S struct{ X, y int }

// client
var a [unsafe.Sizeof(pkg.S{})]int = [8]int{} // fails in new because S's size is not 8
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
var a [C]int = [1]int{} // fails in new because [2]int and [1]int are different types
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
} // fails in new because two cases have the same type
```
This sort of incompatibility is sufficiently esoteric to ignore; the tool allows
merging types.

## First Attempt at a Definition

Our first attempt at defining compatibility captures the idea that all the
exported names in the old package must have compatible equivalents in the new
package.

A new package is compatible with an old if and only if:
- For every exported package-level name in the old, the same name occurs in the
  new, and
- the names denote the same kind of object (e.g. both are variables), and
- the types of the objects are compatible.

We will work out the details (and make some corrections) below, but it is clear
already that we will need to determine what makes two types compatible. And
whatever the defintion of type compatibility, it's certainly true that if two
types are the same, they are compatible. So we will need to decide what makes an
old and new type the same. We will call this sameness relation _correspondence_.

## Type Correspondence

Go already has a definition of when two types are the same:
[type identity](https://golang.org/ref/spec#Type_identity).
But identity isn't adequate for our purpose: it says that two defined
types are identical if they arise from the same definition, but it's unclear
what "same" means when talking about two different packages (or two versions of
a single package).

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
Second, an unexported type can be renamed:

```
// old
type u1 int
var V u1

// new
type u2 int
var V u2
```
Here, even though `u1` and `u2` are unexported, they are still _exposed_ by the
API: their public fields and methods are visible to clients, so they are in some
sense part of the API. But since the name `u1` is not visible to clients, it can
be changed compatibly.

We will say that an old defined type _corresponds_ to a new one if they have the
same name, or one can be renamed to the other without otherwise changing the
API. In the first example above, old `E` and new `t` correspond. In the second,
old `u1` and new `u2` correspond.

Two or more old defined types can correspond to a single new type: we consider
"merging" two types into one to be a compatible change. As mentioned above,
code that uses both names in a type switch will fail, but we deliberately ignore
this case. However, a single old type can correspond to only one new type.

So far, we've explained what correspondence means for defined types. To extend
the definition to all types, we parallel the language's definition of type
identity. So, for instance, an old and a new slice type correspond if their
element types correspond.

## Definition of Compatibility

We can now present the definition of compatibility used by `apidiff`.

### Package Compatibility

> A new package is compatible with an old one if:
1. Each exported name in the old package's scope also appears in the new
package's scope, and the object denoted by that name in the old package is
compatible with the object denoted by the name in the new package, and
2. For every exposed type that implements an exposed interface in the old package,
> its corresponding type should implement the corresponding interface in the new package.
>
>Otherwise the packages are incompatible.

I use the term "object" in the sense of `go/types`: a named entity created by a
declaration. There are four kinds of objects that can occur at package level:
constants, variables, functions, and typenames.

The tool also finds exported names in the new package that are not exported in the old, and
marks them as compatible changes.

Clause 2 is discussed further in "Whole-Package Compatibility."

### Object Compatibility

This section provides compatibility rules for constants, variables, functions
and typenames.

#### Constants

>A new exported constant is compatible with an old one of the same name if and only if
>1. Their types correspond, and
>2. If the old value is representable by a value of type T, so is the new one.

It is tempting to allow changing a typed constant to an untyped one. That may
seem harmless, but it can break code like this:

```
// old
const C int64 = 1

// new
const C = 1

// client
x := C          // old type is int64, new is int
var y int64 = x // fails in new: different types in assignment
```

We said above that it would be too noisy to report an incompatibility
whenever an integer constant's value changed. But it seems reasonable to report
value changes that can break assignments like `var x uint8 = pkg.C`. If we tried to
catch all such breakages, we would be back to forbidding any change to a constant value
(e.g. `var x uint8 = pkg.C + 254` breaks if `C` changes from 1 to 2).

#### Variables

>A new exported variable is compatible with an old one of the same name if and
>only if their types correspond.

Correspondence stops at names, so this rule does not prevent adding a
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
>same name if and only if their types (signatures) correspond.

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

A typename is the name of a type. Here are three typenames, `A`, `B` and `C`:

```
type A []int
type B = A
type C = []int
```
`A` names a defined type whose underlying type is `[]int`. `B` is an alias for
the defined type named `A`. `C` is an alias for a slice type.

> A new exported typename is compatible with an old one if and only if their
> names are the same and their types correspond.

This rule seems far too strict. But note that a typename is the name of a
type; it is not to be confused with a defined type. In the absence of
aliases, every typename's type was a defined type, and this rule demands only
that those defined types correspond.

Consider:
```
// old
type T struct { X int }

// new
type T struct { X, Y int }
```
The addition of `Y` is a compatible change, because this rule does not require
that the struct literals have to correspond, only that the defined types
denoted by `T` must correspond.

If one typename is an alias that refers to the corresponding defined type, the
situation is the same:

```
// old
type T struct { X int }

// new
type u struct { X, Y int }
type T = u
```
Here, the only requirement is that old `T` corresponds to new `u`, not that the
struct types correspond.

However, the following change is incompatible, because the typenames do not
denote corresponding types:
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

We justify the compatibility rules by enumerating all the ways a kind of type
can be used, and showing that the allowed changes cannot break any code that
uses values of they type in those ways.

Values of all types can be used in assignments (including argument passing and
function return), but we do not require that old and new types are assignment
compatible. That is because we assume that the old and new packages are never
used together: any given binary will link in either the old package or the new.
So in describing how a type can be used in the sections below, we omit
assignment.

Any type can also be used in an assertion or conversion. The changes we allow
below may affect the _outcome_ of these operations, but they cannot affect
whether they compile or not. The only such breaking change would be to change
the type `T` in an assertion `x.T` so that it no longer implements the interface
type of `x`; but the rules for interfaces below will disallow that.

> A new type is compatible with an old if and only if they correspond, or
> one of the cases below applies.

#### Defined Types

The only ways to use a defined type are to access its methods, or to use its the
underlying type. Rule 2 below covers the latter, and rules 3 and 4 cover the
former.

> A new defined type is compatible with an old one if and only if all of the
> following hold:
>1. They correspond.
>2. Their underlying types are compatible.
>3. The new exported value method set is a superset of the old.
>4. The new exported pointer method set is a superset of the old.

An exported method set is a method set with all unexported methods removed.
When comparing methods of a method set, we require identical names and
corresponding signatures.

Removing an exported method is clearly a breaking change. But removing an
unexported one (or changing its signature) can be breaking as well, if it
results in the type no longer implementing an interface. See "Whole-Package
Compatibility," below.

#### Channels

The only ways to use a channel type are to send and receive on it, or use it as
a map key. Rule 1 below ensures that any operations on the values sent or
received will compile, as will uses of the channel value as a map key. Rule 2
captures the fact that any program that compiles with a directed channel must
use either only sends, or only receives, so allowing the other operation cannot
break any code.

> A new channel type is compatible with an old one if
>  1. The element types correspond, and
>  2. Either the directions are the same, or the new type has no direction.

#### Interfaces

> A new interface is compatible with an old one if and only if:
> 1. The interface does not have an unexported method, and the old and new
>    interfaces correspond (have the same method set), or
> 2. The interface has an unexported method and the new exported method set is a
>    superset of the old.

The only ways to use an interface are to implement it, embed it, or call one of
its methods. (Interface values can also be used as map keys, but that cannot
cause a compile-time error.)

Certainly, removing an exported method from an interface could break a client
call, so neither rule allows it.

Rule 1 also disallows adding a method to an interface without an unexported
method. Such an interface can be implemented in client code. If adding a method
were allowed, a type that
implements the old interface could fail to implement the new one:

```
type I interface { M1() }         // old
type I interface { M1(); M2() }   // new

// client
type t struct{}
func (t) M1() {}
var i pkg.I = t{} // fails with new, because t lacks M2
```

Rule 2 is based on the observation that if an interface has an unexported
method, the only way a client can implement it is to embed it in a struct.
Adding a method is compatible in this case, because the embedding struct will
continue to implement the interface. Adding a method also cannot break any call
sites, since no program that compiles could have any such call sites.

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

Two fields are the same if they have the same name and corresponding types.

There are only four ways to use a struct: write a struct literal, select a
field, use the value as a map key, or compare two values for equality. The first
clause ensures that struct literals compile; the second, that selections
compile; and the third, that equality expressions and map index expressions
compile.

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
var i pkg.I = pkg.T{} // fails in new because T lacks m
```

Similarly, adding a method to an interface can cause defined types
in the package to stop implementing it.

The second clause in the definition for package compatibility handles these cases.

Other incompatibilities that involve more than one type in the package can arise
whenever two identical types exist in the old or new package. Here, a change
"splits" an identical type into two, breaking conversions:

```
// old
type B struct { X int }
type C struct { X int }

// new
type B struct { X int }
type C struct { X, Y int }

// client
var b B
_ = C(b) // fails in new: cannot convert B to C
```
Finally, changes that are compatible for the package in which they occur can
break downstream packages. That can happen even if they involve unexported
methods, thanks to embedding.

The definition given here doesn't account for these sorts of problems.


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
even though a client developer would have no choice but to copy the type
literal if they wanted to use `V` more flexibly.

The following alternative rule covers all of these cases:

> A type can be changed compatibly if and only if it has an exported name
> (whether by alias or not), or it cannot be written as a type literal externally.

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
write the same literal twice, but change only one instance, a client developer
could write breaking code:

```
// old
var V1 struct { X, u int }
var V2 struct { X, u int }

// new
var V1 struct { X, u int }
var V2 struct { X, Y, u int }

// client
pkg.V1 = pkg.V2 // fails in new: different types in assignment
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
cannot be written in the client. For example:

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
