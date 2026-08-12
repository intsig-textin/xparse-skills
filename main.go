package main

import (
	"errors"
	"os"

	cmd "gitlab.intsig.net/xparse/xparse-client/cmd/parse"
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
