// Package config provides configuration management for the Peak transpiler.
//
// Configuration can be loaded from:
// 1. Config file (peakconfig.json) in the target directory
// 2. CLI flags (highest priority)
// 3. Defaults (backwards compatible)
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Instantiate holds structured instantiation configuration
type Instantiate struct {
	// Classes maps template class names to type arguments
	// Example: {"Queue": ["Integer", "String"], "Optional": ["Double"]}
	Classes map[string][]string `json:"classes,omitempty"`

	// Methods maps "ClassName.methodName" to type arguments
	// Example: {"SObjectCollection.groupBy": ["String", "Decimal", "Boolean"]}
	Methods map[string][]string `json:"methods,omitempty"`
}

// CompilerOptions contains compiler-specific configuration options
type CompilerOptions struct {
	// RootDir is the root directory for preserving directory structure
	// When set, output paths preserve structure relative to this root
	RootDir string `json:"rootDir,omitempty"`

	// OutDir is the output directory relative to the source directory
	// Empty string means co-located with source (default behavior)
	OutDir string `json:"outDir,omitempty"`

	// ApiVersion is the Salesforce API version for generated .cls-meta.xml files
	// Default: "65.0"
	ApiVersion string `json:"apiVersion,omitempty"`

	// Verbose enables detailed logging (default: false)
	Verbose bool `json:"verbose,omitempty"`

	// Instantiate provides structured instantiation for classes and methods
	Instantiate *Instantiate `json:"instantiate,omitempty"`
}

// ConfigFile represents the structure of peak.config.json
type ConfigFile struct {
	CompilerOptions CompilerOptions `json:"compilerOptions,omitempty"`
}

// Config represents the runtime configuration for the transpiler
type Config struct {
	RootDir     string       // Root directory for structure preservation (absolute path, empty = use SourceDir)
	SourceDir   string       // Directory to compile (from CLI or current dir)
	OutDir      string       // Output directory (absolute path, empty = co-located)
	ApiVersion  string       // Salesforce API version for .cls-meta.xml files (default: "65.0")
	Watch       bool         // Watch mode enabled
	Verbose     bool         // Enable verbose logging
	Instantiate *Instantiate // Structured instantiation for classes and methods
}

// CLIFlags represents command-line flags
type CLIFlags struct {
	RootDir    string
	OutDir     string
	ApiVersion string
	Watch      bool
	Verbose    bool
}

// LoadConfig loads configuration for a specific source directory.
// Priority: CLI flags > Config file > Defaults
func LoadConfig(sourceDir string, flags CLIFlags) (*Config, error) {
	// Convert source directory to absolute path
	absSourceDir, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("invalid source directory: %w", err)
	}

	// Start with defaults (backwards compatible behavior)
	config := &Config{
		RootDir:    "",      // Empty = use SourceDir for relative paths
		SourceDir:  absSourceDir,
		OutDir:     "",      // Empty = co-located with source
		ApiVersion: "65.0",  // Default Salesforce API version
		Watch:      false,
		Verbose:    false,
	}

	// Try to load config file from source directory (optional)
	if configFile := findConfigFile(absSourceDir); configFile != "" {
		if err := loadConfigFile(configFile, config); err != nil {
			return nil, fmt.Errorf("error loading config file %s: %w", configFile, err)
		}
	}

	// Override with CLI flags (highest priority)
	if flags.RootDir != "" {
		config.RootDir = flags.RootDir
	}
	if flags.OutDir != "" {
		config.OutDir = flags.OutDir
	}
	if flags.ApiVersion != "" {
		config.ApiVersion = flags.ApiVersion
	}
	if flags.Watch {
		config.Watch = true
	}
	if flags.Verbose {
		config.Verbose = true
	}

	// Normalize root directory to absolute path
	if config.RootDir != "" {
		// If RootDir is relative, make it relative to source directory
		if !filepath.IsAbs(config.RootDir) {
			config.RootDir = filepath.Join(absSourceDir, config.RootDir)
		}
		config.RootDir = filepath.Clean(config.RootDir)
	}

	// Normalize output directory to absolute path
	if config.OutDir != "" {
		// If OutDir is relative, make it relative to source directory
		if !filepath.IsAbs(config.OutDir) {
			config.OutDir = filepath.Join(absSourceDir, config.OutDir)
		}
		config.OutDir = filepath.Clean(config.OutDir)
	}

	return config, nil
}

// findConfigFile looks for peakconfig.json in the specified directory only.
// Returns empty string if no config file is found.
func findConfigFile(dir string) string {
	path := filepath.Join(dir, "peakconfig.json")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return "" // No config file found
}

// loadConfigFile reads and parses a JSON config file
func loadConfigFile(path string, config *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply compiler options to config
	opts := configFile.CompilerOptions
	if opts.RootDir != "" {
		config.RootDir = opts.RootDir
	}
	if opts.OutDir != "" {
		config.OutDir = opts.OutDir
	}
	if opts.ApiVersion != "" {
		config.ApiVersion = opts.ApiVersion
	}
	config.Verbose = opts.Verbose
	config.Instantiate = opts.Instantiate

	return nil
}

// ResolveOutputPath determines the output path for a source file based on config
func (c *Config) ResolveOutputPath(sourcePath string, outputExtension string) (string, error) {
	// Get the base name without extension
	base := filepath.Base(sourcePath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	// Backwards compatible: no config = co-located
	if c.OutDir == "" {
		dir := filepath.Dir(sourcePath)
		return filepath.Join(dir, name+outputExtension), nil
	}

	// Determine the base directory for relative path calculation
	// If RootDir is set, use it; otherwise use SourceDir (backwards compatible)
	baseDir := c.SourceDir
	if c.RootDir != "" {
		baseDir = c.RootDir
	}

	// With output directory configured, preserve directory structure relative to base
	relPath, err := filepath.Rel(baseDir, sourcePath)
	if err != nil {
		// If we can't get relative path, fall back to flat output
		return filepath.Join(c.OutDir, name+outputExtension), nil
	}

	// Preserve directory structure in output
	outputDir := filepath.Join(c.OutDir, filepath.Dir(relPath))
	return filepath.Join(outputDir, name+outputExtension), nil
}

// GenerateMetaXML generates the content for a .cls-meta.xml file
func (c *Config) GenerateMetaXML() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>%s</apiVersion>
    <status>Active</status>
</ApexClass>
`, c.ApiVersion)
}
