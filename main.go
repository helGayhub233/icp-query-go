package main

import (
	"os"

	"github.com/imxw/icp-query-go/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if code, ok := cmd.ExitCode(err); ok {
			os.Exit(code)
		}
		os.Exit(1)
	}
}
