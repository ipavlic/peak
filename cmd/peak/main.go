// Package main provides the Peak to Apex transpiler CLI.
//
// The CLI supports two modes:
//   - Compile mode: transpile all .peak files in a directory once
//   - Watch mode: continuously monitor and recompile on changes
//
// Usage:
//
//	peak [directory] [--watch]
package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	watchMode := false
	dir := "."

	// Parse arguments: [directory] [--watch]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--watch" || arg == "-w" {
			watchMode = true
		} else if dir == "." {
			// First non-flag argument is the directory
			dir = arg
		} else {
			// Too many arguments
			fmt.Fprintf(os.Stderr, "Error: too many arguments\n\n")
			printUsage()
			os.Exit(1)
		}
	}

	// Run in watch or compile mode
	var err error
	if watchMode {
		err = runWatch(dir)
	} else {
		err = runFolder(dir)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Peak to Apex Transpiler\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s [directory] [--watch]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Examples:\n")
	fmt.Fprintf(os.Stderr, "  %s                                # Compile current directory\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s examples/                      # Compile specific directory\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s . --watch                      # Watch current directory\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s examples/ --watch              # Watch specific directory\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Output .cls files are generated in source directories\n")
}
