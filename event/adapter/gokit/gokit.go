// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gokit provides a go-kit logger for events.
package gokit

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kit/kit/log"
	"golang.org/x/exp/event"
	"golang.org/x/exp/event/keys"
)

type logger struct {
}

// NewLogger returns a logger.
func NewLogger() log.Logger {
	return &logger{}
}

// Log writes a structured log message.
// If the first argument is a context.Context, it is used
// to find the exporter to which to deliver the message.
// Otherwise, the default exporter is used.
func (l *logger) Log(keyvals ...interface{}) error {
	ctx := context.Background()
	if len(keyvals) > 0 {
		if c, ok := keyvals[0].(context.Context); ok {
			ctx = c
			keyvals = keyvals[1:]
		}
	}
	t := event.To(ctx)
	labels := allocLabels(len(keyvals) / 2)
	defer labels.free()
	var msg string
	for i := 0; i < len(keyvals); i += 2 {
		key := keyvals[i].(string)
		value := keyvals[i+1]
		if key == "msg" || key == "message" {
			msg = fmt.Sprint(value)
		} else {
			labels.add(keys.Value(key).Of(value))
		}
	}
	t.Log(msg, labels.slice...)
	return nil
}

// TODO: consider making this part of the event API.

type labelList struct {
	slice []event.Label
	array [labelSize]event.Label
}

const labelSize = 10

var labelPool = sync.Pool{New: func() interface{} { return &labelList{} }}

func allocLabels(n int) *labelList {
	l := labelPool.Get().(*labelList)
	l.slice = l.array[:0]
	return l
}

func (ls *labelList) add(l event.Label) {
	ls.slice = append(ls.slice, l)
}

func (ls *labelList) free() {
	labelPool.Put(ls)
}
