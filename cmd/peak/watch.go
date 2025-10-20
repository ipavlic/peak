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
func runWatch(dir string, outDir string) error {
	if err := validateDirectory(dir); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Watching directory: %s\n", dir)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop\n\n")

	// Initial compilation
	if err := compileDirectory(dir, outDir); err != nil {
		fmt.Fprintf(os.Stderr, "Initial compilation failed: %v\n", err)
	}

	watcher, ctx, cancel, err := setupWatcher(dir)
	if err != nil {
		return err
	}
	defer watcher.Close()
	defer cancel()

	return watchLoop(ctx, watcher, dir, outDir)
}

// validateDirectory checks if the directory exists
func validateDirectory(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}
	return nil
}

// setupWatcher creates and configures the file watcher with signal handling
func setupWatcher(dir string) (*fsnotify.Watcher, context.Context, context.CancelFunc, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, nil, nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintf(os.Stderr, "\nReceived interrupt signal, shutting down...\n")
		signal.Stop(sigChan)
		cancel()
	}()

	return watcher, ctx, cancel, nil
}

// watchLoop runs the main event loop for file watching
func watchLoop(ctx context.Context, watcher *fsnotify.Watcher, dir string, outDir string) error {
	var debounceTimer *time.Timer

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			debounceTimer = handleFileEvent(ctx, event, dir, outDir, debounceTimer)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
		}
	}
}

// handleFileEvent processes file system events and triggers recompilation
func handleFileEvent(ctx context.Context, event fsnotify.Event, dir string, outDir string, debounceTimer *time.Timer) *time.Timer {
	// Only respond to .peak file changes
	if !strings.HasSuffix(event.Name, peakExtension) {
		return debounceTimer
	}

	// Handle write and create events
	if event.Op&fsnotify.Write != fsnotify.Write && event.Op&fsnotify.Create != fsnotify.Create {
		return debounceTimer
	}

	// Reset debounce timer
	if debounceTimer != nil {
		debounceTimer.Stop()
	}

	return time.AfterFunc(debounceDuration, func() {
		select {
		case <-ctx.Done():
			return
		default:
			fmt.Fprintf(os.Stderr, "\n[%s] Change detected: %s\n",
				time.Now().Format(timeFormat), filepath.Base(event.Name))
			if err := compileDirectory(dir, outDir); err != nil {
				fmt.Fprintf(os.Stderr, "Compilation failed: %v\n", err)
			}
		}
	})
}
