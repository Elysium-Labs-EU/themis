package main

import (
	"errors"
	"fmt"
	"os"

	"codeberg.org/Elysium_Labs/themis/cmd"
	"codeberg.org/Elysium_Labs/themis/internal/ui"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var uerr *ui.UserError
		if errors.As(err, &uerr) {
			fmt.Fprintln(os.Stderr, uerr.Render())
		} else {
			fmt.Fprintf(os.Stderr, "%s %s\n", ui.LabelError.Render("error"), err)
		}
		os.Exit(1)
	}
}
