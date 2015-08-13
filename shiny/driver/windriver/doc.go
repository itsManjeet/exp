// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package windriver provides the Windows driver for accessing a screen.
package windriver

/*
Implementation Details

On Windows, UI can run on any thread, but any windows created
on a thread must only be manipulated from that thread. In addition,
each thread that hosts UI must handle incoming "window messages"
through a "message pump". When you call Main(), windriver
designates the OS thread that calls Main() as the UI thread and runs
the function you pass to Main() in another goroutine.

Window messages are the preferred way of communicating across
threads on Windows. When you send a window message to a window
on a different thread, Windows will temporarily switch to that thread
to process the message. Therefore, windriver turns any requests to
do things (create windows, draw to them, etc.) into window messages
and sends them along.

A special invisible window is created by Main(). This window is
currently called the "utility window". The utility window handles
tasks such as creating new windows. A better name (the "screen
window" to go along with screen.Screen?) can be used if desired.
Operations on windows can be (TODO(andlabs) actually do this)
sent to the individual windows.

Presently, the actual Windows API work is implemented in C. This is
to encapsulate Windows's data structures, ensure properly handling
signed -> unsigned conversions in constants, and properly handle
the "last error". When a window manager function returns, it returns
whether or not the function succeeded. If it didn't, we call the
GetLastError() function to get the actual reason for the failure.

To simplify error reporting, the GetLastError() calls are wrapped
by another function, lastErrorToHRESULT(). This turns the last error
into the newer HRESULT error code system: the macro
HRESULT_FROM_WIN32() is called to turn the error code into an
HRESULT, which we can get the original value out from on the Go side.
If GetLastError() returns 0 (no error) for whatever reason, we return
the special constant E_FAIL instead, which the Go side sees as
"unknown error". The constant S_OK is used to indicate that there
was no error at all.

The C functions come in pairs:

	HRESULT doSomething(parameters)
	{
		return (HRESULT) SendMessageW(target, message, parameters);
	}

	LRESULT actualDoSomething(parameters)
	{
		// do things
		return (LRESULT) S_OK;
	}

The doSomething() is what Go calls; it simply constructs and sends a
window message to do the actual work. The "window procedure"
of the target window calls the actualDoSomething() function, which
does the actual work and returns the HRESULT error code as a
LRESULT (which is the canonical return type for a window message,
and whose size is >= the size of HRESULT).

TODO(andlabs): clean this up and elaborate things
*/
