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
	"strings"
)

func main() {
	args := os.Args[1:]
	watchMode := false
	rootDir := ""
	outDir := ""
	dir := "."

	// Parse arguments: [directory] [--watch] [--root-dir <dir>] [--out-dir <dir>] [--help]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--help" || arg == "-h" {
			printUsage()
			os.Exit(0)
		} else if arg == "--watch" || arg == "-w" {
			watchMode = true
		} else if arg == "--root-dir" || arg == "-r" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: %s requires a directory argument\n\n", arg)
				printUsage()
				os.Exit(1)
			}
			i++
			rootDir = args[i]
		} else if arg == "--out-dir" || arg == "-o" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: %s requires a directory argument\n\n", arg)
				printUsage()
				os.Exit(1)
			}
			i++
			outDir = args[i]
		} else if !strings.HasPrefix(arg, "-") {
			if dir == "." {
				// First non-flag argument is the directory
				dir = arg
			} else {
				// Too many arguments
				fmt.Fprintf(os.Stderr, "Error: too many arguments\n\n")
				printUsage()
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: unknown flag %s\n\n", arg)
			printUsage()
			os.Exit(1)
		}
	}

	// Run in watch or compile mode
	var err error
	if watchMode {
		err = runWatch(dir, rootDir, outDir)
	} else {
		err = runFolder(dir, rootDir, outDir)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	// ANSI color codes
	const (
		blue     = "\033[34m"
		boldBlue = "\033[1;34m"
		green    = "\033[32m"
		reset    = "\033[0m"
	)

	fmt.Fprintf(os.Stderr, "Peak to Apex Transpiler\n\n")
	fmt.Fprintf(os.Stderr, "%sUSAGE%s\n", boldBlue, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s [directory] [options]\n\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "%sOPTIONS%s\n", boldBlue, reset)
	fmt.Fprintf(os.Stderr, "  %s--help, -h%s                 Display this help message\n", blue, reset)
	fmt.Fprintf(os.Stderr, "  %s--watch, -w%s                Watch for changes and recompile\n", blue, reset)
	fmt.Fprintf(os.Stderr, "  %s--root-dir, -r%s <dir>       Root directory for preserving structure (overrides config)\n", blue, reset)
	fmt.Fprintf(os.Stderr, "  %s--out-dir, -o%s <dir>        Output directory (overrides config file)\n\n", blue, reset)
	fmt.Fprintf(os.Stderr, "%sEXAMPLES%s\n", boldBlue, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s                                        # Compile current directory\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s examples/                              # Compile specific directory\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s --watch                                # Watch current directory\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s --out-dir build/ src/                  # Output to build/\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s --root-dir . --out-dir build/ src/     # Preserve structure from root\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "  %s$ %speak%s --watch --out-dir dist/                # Watch and output to dist/\n\n", green, reset, reset)
	fmt.Fprintf(os.Stderr, "%sCONFIGURATION%s\n", boldBlue, reset)
	fmt.Fprintf(os.Stderr, "  Config file: peakconfig.json in source directory\n")
	fmt.Fprintf(os.Stderr, "  Default: Output .cls files co-located with source .peak files\n")
}
