package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"peak/pkg/parser"
	"peak/pkg/transpiler"
)

// runFolder compiles all .peak files in the specified directory.
func runFolder(dir string) error {
	return compileDirectory(dir)
}

const (
	filePermission = 0o644   // Standard file permission for generated .cls files
	peakExtension  = ".peak" // Peak source file extension
	apexExtension  = ".cls"  // Apex output file extension
)

// compileDirectory compiles all .peak files in the specified directory.
func compileDirectory(dir string) error {
	startTime := time.Now()

	// Find all .peak files recursively
	peakFiles, err := findPeakFiles(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory '%s' does not exist\n\nTip: Check the directory path and try again", dir)
		}
		return fmt.Errorf("error finding .peak files: %w", err)
	}

	if len(peakFiles) == 0 {
		return fmt.Errorf("no .peak files found in '%s'\n\nTip: Make sure the directory contains .peak source files", dir)
	}

	// Read all input files
	files := make(map[string]string, len(peakFiles))
	for _, peakFile := range peakFiles {
		content, err := os.ReadFile(peakFile)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", peakFile, err)
		}
		files[peakFile] = string(content)
	}

	// Transpile all files
	tr := transpiler.NewTranspiler()
	results, err := tr.TranspileFiles(files)
	if err != nil {
		return fmt.Errorf("error transpiling: %w", err)
	}

	// Write output files and collect statistics
	var generatedFiles, skippedTemplates, errorCount int

	for _, result := range results {
		// Handle errors
		if result.Error != nil {
			errorCount++
			if parseErr, ok := result.Error.(*parser.ParseError); ok {
				fmt.Fprint(os.Stderr, parseErr.FormatError())
			} else {
				fmt.Fprintf(os.Stderr, "  ERROR in %s: %v\n", result.OriginalPath, result.Error)
			}
			continue
		}

		if result.IsTemplate {
			skippedTemplates++
			fmt.Fprintf(os.Stderr, "Skipped template: %s\n", result.OriginalPath)
			continue
		}

		if err := os.WriteFile(result.OutputPath, []byte(result.Content), filePermission); err != nil {
			return fmt.Errorf("error writing %s: %w", result.OutputPath, err)
		}

		generatedFiles++
		if result.OriginalPath != "" {
			fmt.Fprintf(os.Stderr, "Generated: %s -> %s\n", result.OriginalPath, result.OutputPath)
		} else {
			fmt.Fprintf(os.Stderr, "Generated concrete class: %s\n", result.OutputPath)
		}
	}

	// Report compilation results
	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\n")

	if errorCount > 0 {
		fmt.Fprintf(os.Stderr, "✗ Compiled %d file(s) (skipped %d template(s)) with %d error(s) in %v\n",
			generatedFiles, skippedTemplates, errorCount, elapsed.Round(time.Millisecond))
		return fmt.Errorf("compilation had %d error(s)", errorCount)
	}

	fmt.Fprintf(os.Stderr, "✓ Compiled %d file(s) (skipped %d template(s)) in %v\n",
		generatedFiles, skippedTemplates, elapsed.Round(time.Millisecond))
	return nil
}

// findPeakFiles recursively finds all .peak files in a directory
func findPeakFiles(root string) ([]string, error) {
	var peakFiles []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and files
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != root {
			return filepath.SkipDir
		}

		// Collect .peak files
		if !info.IsDir() && strings.HasSuffix(path, peakExtension) {
			peakFiles = append(peakFiles, path)
		}

		return nil
	})

	return peakFiles, err
}
