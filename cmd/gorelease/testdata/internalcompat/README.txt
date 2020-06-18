Modules example.com/internalcompat/{a,b} are copies. One could be a fork
of the other. An external package p exposes a type from an internal
package q.

gorelease should not report differences between these packages. The types
are distinct, but that difference isn't interesting for anyone comparing
two modules.
