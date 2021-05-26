// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package event

const Message = stringKey("msg")

type stringKey string

// Of creates a new message Label.
func (k stringKey) Of(msg string) Label {
	return Label{Name: string(k), Value: StringOf(msg)}
}

func (k stringKey) Find(ev *Event) (string, bool) {
	for _, v := range ev.Labels {
		if v.Name == string(k) {
			return v.Value.String(), true
		}
	}
	return "", false
}
