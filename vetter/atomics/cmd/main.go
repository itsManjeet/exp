package main

import (
	"golang.org/x/exp/vetter/atomics"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(atomics.Analyzer)
}
