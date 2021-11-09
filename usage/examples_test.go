// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.16
// +build go1.16

package usage_test

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/exp/usage"
)

//go:embed help/naval_fate help/_naval_fate
var navalHelp embed.FS

type navalOptions struct {
	NavalFate string
	Entity    string
	Name      string
	Action    string
	Speed     float64
	X, Y      uint64
	Moored    bool
	Drifting  bool

	Help    bool
	Version bool
}

func ExampleProcess_naval() {
	options := &navalOptions{}
	args := []string{"naval_fate", "ship", "indomitable", "move", "12", "15"}
	help, err := usage.Load(navalHelp)
	if err != nil {
		fmt.Println(err)
		return
	}
	fields, err := usage.FieldsOf(options)
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := usage.Process(help, fields, args); err != nil {
		fmt.Println(err)
		return
	}
	buf, _ := json.MarshalIndent(options, "", "  ")
	os.Stdout.Write(buf)
	//Output:
	// {
	//   "NavalFate": "naval_fate",
	//   "Entity": "ship",
	//   "Name": "indomitable",
	//   "Action": "move",
	//   "Speed": 10,
	//   "X": 12,
	//   "Y": 15,
	//   "Moored": false,
	//   "Drifting": false,
	//   "Help": false,
	//   "Version": false
	// }
}
