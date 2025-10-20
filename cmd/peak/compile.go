package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ipavlic/peak/pkg/config"
	"github.com/ipavlic/peak/pkg/parser"
	"github.com/ipavlic/peak/pkg/transpiler"
)

// runFolder compiles all .peak files in the specified directory.
func runFolder(dir string, rootDir string, outDir string) error {
	return compileDirectory(dir, rootDir, outDir)
}

const (
	filePermission = 0o644   // Standard file permission for generated .cls files
	peakExtension  = ".peak" // Peak source file extension
	apexExtension  = ".cls"  // Apex output file extension

	// ANSI color codes (matching help output style)
	blue     = "\033[34m"
	boldBlue = "\033[1;34m"
	green    = "\033[32m"
	yellow   = "\033[33m"
	red      = "\033[31m"
	gray     = "\033[90m"
	reset    = "\033[0m"
)

// compileDirectory compiles all .peak files in the specified directory.
func compileDirectory(dir string, rootDir string, outDir string) error {
	startTime := time.Now()

	// Load configuration
	cfg, err := config.LoadConfig(dir, config.CLIFlags{
		RootDir: rootDir,
		OutDir:  outDir,
	})
	if err != nil {
		return fmt.Errorf("error loading configuration: %w", err)
	}

	// Find all .peak files recursively
	peakFiles, err := findPeakFiles(cfg.SourceDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory '%s' does not exist\n\nTip: Check the directory path and try again", cfg.SourceDir)
		}
		return fmt.Errorf("error finding .peak files: %w", err)
	}

	if len(peakFiles) == 0 {
		return fmt.Errorf("no .peak files found in '%s'\n\nTip: Make sure the directory contains .peak source files", cfg.SourceDir)
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

	// Create output path resolver function
	outputPathFn := func(sourcePath string) (string, error) {
		return cfg.ResolveOutputPath(sourcePath, apexExtension)
	}

	// Transpile all files
	tr := transpiler.NewTranspiler(outputPathFn)
	if cfg.Instantiate != nil {
		tr.SetInstantiate(cfg.Instantiate)
	}
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
				fmt.Fprintf(os.Stderr, "  %sERROR%s in %s%s%s: %v\n",
					red, reset,
					blue, result.OriginalPath, reset,
					result.Error)
			}
			continue
		}

		if result.IsTemplate {
			skippedTemplates++
			fmt.Fprintf(os.Stderr, "%sSkipped template:%s %s\n", yellow, reset, result.OriginalPath)
			continue
		}

		// Ensure output directory exists
		outputDir := filepath.Dir(result.OutputPath)
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("error creating output directory %s: %w", outputDir, err)
		}

		if err := os.WriteFile(result.OutputPath, []byte(result.Content), filePermission); err != nil {
			return fmt.Errorf("error writing %s: %w", result.OutputPath, err)
		}

		generatedFiles++
		if result.OriginalPath != "" {
			fmt.Fprintf(os.Stderr, "%sGenerated:%s %s%s%s -> %s%s%s\n",
				green, reset,
				gray, result.OriginalPath, reset,
				blue, result.OutputPath, reset)
		} else {
			fmt.Fprintf(os.Stderr, "%sGenerated concrete class:%s %s%s%s\n",
				green, reset,
				blue, result.OutputPath, reset)
		}
	}

	// Report compilation results
	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "\n")

	if errorCount > 0 {
		fmt.Fprintf(os.Stderr, "%s✗%s Compiled %s%d%s file(s) (skipped %s%d%s template(s)) with %s%d error(s)%s in %s%v%s\n",
			red, reset,
			boldBlue, generatedFiles, reset,
			yellow, skippedTemplates, reset,
			red, errorCount, reset,
			gray, elapsed.Round(time.Millisecond), reset)
		return fmt.Errorf("compilation had %d error(s)", errorCount)
	}

	fmt.Fprintf(os.Stderr, "%s✓%s Compiled %s%d%s file(s) (skipped %s%d%s template(s)) in %s%v%s\n",
		green, reset,
		boldBlue, generatedFiles, reset,
		yellow, skippedTemplates, reset,
		gray, elapsed.Round(time.Millisecond), reset)
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
