// Package main is the entry point for the MartMart CLI.
package main

import (
	"fmt"
	"os"

	"github.com/wydrox/martmart-cli/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}
