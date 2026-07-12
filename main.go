package main

import (
	"fmt"
	"os"

	"codeberg.org/Elysium_Labs/themis/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
