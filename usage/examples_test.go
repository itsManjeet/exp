// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package usage_test

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/exp/usage"
)

var navalHelp = usage.Text{
	Pages: []usage.Page{{Content: []byte(`
Naval Fate.

Usage:
  naval_fate ship new <name>...
  naval_fate ship <name> move <x> <y> [-speed=kn]
  naval_fate ship shoot <x> <y>
  naval_fate mine (set|remove) <x> <y> [-moored|-drifting]
  naval_fate -h,-help
  naval_fate -version

Options:
  -h,-help    Show this screen.
  -version     Show version.
  -speed=kn    Speed in knots [default: 10].
  -moored      Moored (anchored) mine.
  -drifting    Drifting mine.
`)}},
	Map: map[string]string{
		"ship":   "entity",
		"mine":   "entity",
		"move":   "action",
		"new":    "action",
		"shoot":  "action",
		"set":    "action",
		"remove": "action",
	},
}

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
	if err := usage.Process(navalHelp, options, args); err != nil {
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
