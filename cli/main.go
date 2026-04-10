package main

import (
	"errors"
	"os"

	"github.com/intsig-textin/xparse-skills/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
