package main

import (
	"fmt"
	"os"

	"github.com/midu16/opm-troubleshooting/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCode(err))
	}
}
