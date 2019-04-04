// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
)

func Run(in io.Reader, out io.Writer) error {
	if _, err := out.Write([]byte("digraph gomodgraph {\n")); err != nil {
		log.Fatal(err)
	}

	r := bufio.NewScanner(in)
	for {
		if !r.Scan() {
			break
		}

		parts := strings.Fields(r.Text())
		if len(parts) == 0 {
			continue
		}

		if _, err := fmt.Fprintf(out, "\t\"%s\" -> \"%s\"\n", parts[0], parts[1]); err != nil {
			return err
		}
	}

	if _, err := out.Write([]byte("}\n")); err != nil {
		return err
	}

	return nil
}
