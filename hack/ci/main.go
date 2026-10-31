package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "qualify-core" {
		fmt.Fprintln(os.Stderr, "usage: go run ./hack/ci qualify-core")
		os.Exit(2)
	}
	if err := qualifyCore(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
