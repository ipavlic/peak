package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"peak/pkg/parser"
	"peak/pkg/transpiler"
)

// runWatch starts file watching mode for the specified directory.
// It performs an initial compilation, then watches for .peak file changes
// and recompiles automatically with a 500ms debounce delay.
func runWatch(dir string) error {
	// Verify directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	fmt.Fprintf(os.Stderr, "Watching directory: %s\n", dir)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop\n\n")

	// Initial compilation
	if err := compileDirectory(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Initial compilation failed: %v\n", err)
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Add directory to watcher
	if err := watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	// Debounce timer to prevent multiple recompiles on rapid changes
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only respond to .peak file changes (ignore .cls files)
			if !strings.HasSuffix(event.Name, ".peak") {
				continue
			}

			// Handle write and create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					fmt.Fprintf(os.Stderr, "\n[%s] Change detected: %s\n", time.Now().Format("15:04:05"), filepath.Base(event.Name))
					if err := compileDirectory(dir); err != nil {
						fmt.Fprintf(os.Stderr, "Compilation failed: %v\n", err)
					}
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
		}
	}
}

// compileDirectory compiles all .peak files in the specified directory.
// It returns an error if compilation fails for any files.
func compileDirectory(dir string) error {
	startTime := time.Now()

	// Find all .peak files
	peakFiles, err := findPeakFiles(dir)
	if err != nil {
		return fmt.Errorf("error finding .peak files: %w", err)
	}

	if len(peakFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No .peak files found in %s\n", dir)
		return nil
	}

	// Read all files
	files := make(map[string]string, len(peakFiles))
	for _, filePath := range peakFiles {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", filePath, err)
		}
		files[filePath] = string(content)
	}

	// Transpile
	tr := transpiler.NewTranspiler()
	results, err := tr.TranspileFiles(files)
	if err != nil {
		return fmt.Errorf("transpilation error: %w", err)
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
				fmt.Fprintf(os.Stderr, "  ERROR in %s: %v\n", filepath.Base(result.OriginalPath), result.Error)
			}
			continue
		}

		if result.IsTemplate {
			skippedTemplates++
			continue
		}

		if err := os.WriteFile(result.OutputPath, []byte(result.Content), 0644); err != nil {
			return fmt.Errorf("error writing %s: %w", result.OutputPath, err)
		}

		generatedFiles++
	}

	// Report compilation results
	elapsed := time.Since(startTime)
	if errorCount > 0 {
		fmt.Fprintf(os.Stderr, "✗ Compiled %d file(s) (skipped %d template(s)) with %d error(s) in %v\n",
			generatedFiles, skippedTemplates, errorCount, elapsed.Round(time.Millisecond))
		return fmt.Errorf("%d compilation error(s)", errorCount)
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
		if !info.IsDir() && strings.HasSuffix(path, ".peak") {
			peakFiles = append(peakFiles, path)
		}

		return nil
	})

	return peakFiles, err
}
