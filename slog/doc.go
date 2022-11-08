// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slog

/*
Package slog provides structured logging with levels.
It defines a type, [Logger], with methods for writing log entries.
Each Logger is a associated with [Handler].
A Logger output method creates a [Record] from the method arguments
and passes it to the Handler.
There is a default Logger accessible through top-level functions
that call the corresponding Logger methods.

A log entry consists of a time, a level, a message, and a set of key-value
pairs, where the keys are strings and the values may be of any type.
As an example,

    slog.Info("hello", "name", "Pat")

has the time of the call, a level of Info, the message "hello", and a single
pair with key "name" and value "Pat".

The [Info] top-level function calls the [Logger.Info] method on the default Logger.
In addition to [Logger.Info], there are methods for Debug, Warn and Error levels.
Besides these convenience methods for common levels,
there is also a [Logger.Log] method which accepts any level.
Each of these methods has a corresponding top-level function that uses the
default logger.

A program consisting solely of the above line will direct the log entry to the
[log] package after adding the level and key-value pairs:

    2022/11/08 15:28:26 INFO hello name=Pat

For more control over the output format, set a handler using [New]:

    logger := slog.New(slog.NewTextHandler(os.Stdout))

[TextHandler] output is a sequence of key=value pairs, easily parsed by machine:

    logger.Info("hello", "name", "Pat")

produces

    time=2022-11-08T15:28:26.000-05:00 level=INFO msg=hello name=Pat

The package also comes with a [JSONHandler], whose output is line-delimited JSON:

    logger := slog.New(slog.NewJSONHandler(os.Stdout))
    logger.Info("hello", "name", "Pat")

results in

    {"time":"2022-11-08T15:28:26.000000000-05:00","level":"INFO","msg":"hello","name":"Pat"}

Setting a logger as the default with

    slog.SetDefault(logger)

will cause the top-level functions like [Info] to use it.
The top-level functions of the [log] package, like [log.Printf], will also use
it, so that packages that use [log] will produce structured output without
needing to be rewritten.

# Attrs and Values

# Levels

# Configuring the built-in handlers

TODO: cover HandlerOptions, Leveler, LevelVar

# Groups

# Contexts

# Advanced topics

## Customizing a type's logging behavior

TODO: discuss LogValuer

## Wrapping output methods

TODO: discuss LogDepth, LogAttrDepth

## Interoperating with other logging packabes

TODO: discuss NewRecord, Record.AddAttrs

## Writing a handler

*/
