package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	debounceDuration = 500 * time.Millisecond // Debounce delay for file changes
	timeFormat       = "15:04:05"             // Time format for change detection messages
)

// runWatch starts file watching mode for the specified directory.
// It performs an initial compilation, then watches for .peak file changes
// and recompiles automatically with a 500ms debounce delay.
// Gracefully handles Ctrl+C (SIGINT) and SIGTERM signals.
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

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived interrupt signal, shutting down...\n")
		cancel()
	}()

	// Debounce timer to prevent multiple recompiles on rapid changes
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only respond to .peak file changes (ignore .cls files)
			if !strings.HasSuffix(event.Name, peakExtension) {
				continue
			}

			// Handle write and create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					// Check if context is still active before compiling
					select {
					case <-ctx.Done():
						return
					default:
						fmt.Fprintf(os.Stderr, "\n[%s] Change detected: %s\n", time.Now().Format(timeFormat), filepath.Base(event.Name))
						if err := compileDirectory(dir); err != nil {
							fmt.Fprintf(os.Stderr, "Compilation failed: %v\n", err)
						}
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
