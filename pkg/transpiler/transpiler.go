// Package transpiler provides transpilation from Peak to Apex.
//
// The transpiler processes entire directories of .peak files and works in four phases:
// 1. Collect generic class definitions (templates)
// 2. Collect generic instantiations (usages)
// 3. Generate output for non-template files
// 4. Generate concrete class files from templates
package transpiler

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ipavlic/peak/pkg/parser"
)

// FileResult represents the transpilation result for a single file
type FileResult struct {
	OriginalPath string
	OutputPath   string
	Content      string
	IsTemplate   bool  // true if this file contains a generic class definition
	Error        error // error encountered during transpilation
}

// Transpiler handles transpilation of Peak files to Apex
type Transpiler struct {
	templates     map[string]*parser.GenericClassDef // Generic class definitions
	templatePaths map[string]string                  // Template name to file path
	usages        map[string]*parser.GenericExpr     // Generic instantiations
	outputPathFn  func(string) (string, error)       // Function to resolve output paths
}

// NewTranspiler creates a new transpiler with a custom output path resolver.
// If outputPathFn is nil, uses default co-located behavior.
func NewTranspiler(outputPathFn func(string) (string, error)) *Transpiler {
	if outputPathFn == nil {
		// Default: co-located .cls files (backwards compatible)
		outputPathFn = func(sourcePath string) (string, error) {
			return strings.TrimSuffix(sourcePath, ".peak") + ".cls", nil
		}
	}

	return &Transpiler{
		templates:     make(map[string]*parser.GenericClassDef),
		templatePaths: make(map[string]string),
		usages:        make(map[string]*parser.GenericExpr),
		outputPathFn:  outputPathFn,
	}
}

// TranspileFiles processes multiple files and generates concrete classes
func (t *Transpiler) TranspileFiles(files map[string]string) ([]FileResult, error) {
	var results []FileResult

	// Phase 1: Collect all generic class definitions (templates)
	hasErrors := t.collectTemplates(files, &results)

	// Phase 2: Collect all generic instantiations
	hasErrors = t.collectUsages(files, &results) || hasErrors

	// If there were errors in parsing, return now with error results
	if hasErrors {
		return results, nil
	}

	// Phase 3: Generate output for each file
	for path, content := range files {
		result, err := t.transpileFile(path, content)
		if err != nil {
			result.Error = err
		}
		results = append(results, result)
	}

	// Phase 4: Generate concrete class files
	concreteClasses := t.generateConcreteClasses()
	results = append(results, concreteClasses...)

	return results, nil
}

// collectTemplates scans all files for generic class definitions (Phase 1)
func (t *Transpiler) collectTemplates(files map[string]string, results *[]FileResult) bool {
	hasErrors := false
	for path, content := range files {
		p := parser.NewParser(content)
		p.SetFileName(path)
		defs, err := p.FindGenericClassDefinitions()
		if err != nil {
			hasErrors = true
			*results = append(*results, FileResult{
				OriginalPath: path,
				Error:        err,
			})
			continue
		}

		for className, def := range defs {
			t.templates[className] = def
			t.templatePaths[className] = path
		}
	}
	return hasErrors
}

// collectUsages scans all files for generic instantiations (Phase 2)
func (t *Transpiler) collectUsages(files map[string]string, results *[]FileResult) bool {
	hasErrors := false
	for path, content := range files {
		contentToScan := t.getContentToScan(content)

		p := parser.NewParser(contentToScan)
		p.SetFileName(path)
		generics, err := p.FindGenerics()
		if err != nil {
			hasErrors = true
			t.recordError(path, err, results)
			continue
		}

		for original, expr := range generics {
			if _, isTemplate := t.templates[expr.BaseType]; isTemplate {
				t.usages[original] = expr
			}
		}
	}
	return hasErrors
}

// getContentToScan determines what content to scan for generic usages
func (t *Transpiler) getContentToScan(content string) string {
	p := parser.NewParser(content)
	defs, _ := p.FindGenericClassDefinitions()

	if len(defs) > 0 {
		// Template file - scan only class bodies to avoid treating
		// "class Queue<T>" as a usage of Queue<T>
		var bodies []string
		for _, def := range defs {
			bodies = append(bodies, def.Body)
		}
		return strings.Join(bodies, "\n")
	}

	return content
}

// recordError adds or updates an error for a file in the results
func (t *Transpiler) recordError(path string, err error, results *[]FileResult) {
	// Check if we already have an error for this file
	for i, r := range *results {
		if r.OriginalPath == path && r.Error != nil {
			(*results)[i].Error = err
			return
		}
	}
	// No existing error, add new one
	*results = append(*results, FileResult{
		OriginalPath: path,
		Error:        err,
	})
}

// transpileFile processes a single file, replacing generic usages with concrete class names.
func (t *Transpiler) transpileFile(path, content string) (FileResult, error) {
	// Check if this file contains a generic template definition
	p := parser.NewParser(content)
	defs, err := p.FindGenericClassDefinitions()
	if err != nil {
		return FileResult{OriginalPath: path, Error: err}, err
	}

	if len(defs) > 0 {
		// This is a template file - don't generate output
		return FileResult{
			OriginalPath: path,
			IsTemplate:   true,
		}, nil
	}

	// Find and replace generic usages with concrete class names
	p = parser.NewParser(content)
	generics, err := p.FindGenerics()
	if err != nil {
		return FileResult{OriginalPath: path, Error: err}, err
	}

	output := t.replaceGenericUsages(content, generics)

	// Generate output path using configured resolver
	outputPath, err := t.outputPathFn(path)
	if err != nil {
		return FileResult{OriginalPath: path, Error: err}, err
	}

	return FileResult{
		OriginalPath: path,
		OutputPath:   outputPath,
		Content:      output,
		IsTemplate:   false,
	}, nil
}

// replaceGenericUsages replaces all generic template usages in content with concrete class names.
// It sorts generics by length (longest first) to handle nested generics correctly.
// Comments are preserved and not modified.
func (t *Transpiler) replaceGenericUsages(content string, generics map[string]*parser.GenericExpr) string {
	// Build replacement map
	replacements := make(map[string]string)
	for original, expr := range generics {
		// Only replace if it's a usage of a known template
		if _, isTemplate := t.templates[expr.BaseType]; isTemplate {
			concrete := parser.GenerateConcreteClassName(expr)
			replacements[original] = concrete
		}
	}

	if len(replacements) == 0 {
		return content
	}

	// Sort keys by length (longest first) to handle nested generics
	sortedKeys := make([]string, 0, len(replacements))
	for key := range replacements {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		return len(sortedKeys[i]) > len(sortedKeys[j])
	})

	// Replace while skipping comments
	var result strings.Builder
	result.Grow(len(content))

	i := 0
	for i < len(content) {
		// Check for single-line comment
		if i < len(content)-1 && content[i] == '/' && content[i+1] == '/' {
			// Copy the entire comment line as-is
			start := i
			for i < len(content) && content[i] != '\n' {
				i++
			}
			if i < len(content) {
				i++ // include the newline
			}
			result.WriteString(content[start:i])
			continue
		}

		// Check for multi-line comment
		if i < len(content)-1 && content[i] == '/' && content[i+1] == '*' {
			// Copy the entire comment as-is
			start := i
			i += 2
			for i < len(content)-1 {
				if content[i] == '*' && content[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			result.WriteString(content[start:i])
			continue
		}

		// Try to match any generic pattern at current position
		matched := false
		for _, original := range sortedKeys {
			if i+len(original) <= len(content) && content[i:i+len(original)] == original {
				// Found a match - replace it
				result.WriteString(replacements[original])
				i += len(original)
				matched = true
				break
			}
		}

		if !matched {
			result.WriteByte(content[i])
			i++
		}
	}

	return result.String()
}

// generateConcreteClasses creates concrete class files from templates by instantiating
// each template with its concrete type arguments.
func (t *Transpiler) generateConcreteClasses() []FileResult {
	results := make([]FileResult, 0, len(t.usages))

	for _, expr := range t.usages {
		template, exists := t.templates[expr.BaseType]
		if !exists {
			continue
		}

		// Get the directory where the template is located
		templatePath := t.templatePaths[expr.BaseType]

		// Generate concrete class content
		content := t.instantiateTemplate(template, expr)
		concreteName := parser.GenerateConcreteClassName(expr)

		// Create a virtual path for the concrete class (in same dir as template)
		templateDir := filepath.Dir(templatePath)
		virtualPath := filepath.Join(templateDir, concreteName+".peak")

		// Resolve output path using configured resolver
		outputPath, err := t.outputPathFn(virtualPath)
		if err != nil {
			// Fall back to template directory if path resolution fails
			outputPath = filepath.Join(templateDir, concreteName+".cls")
		}

		results = append(results, FileResult{
			OriginalPath: "",
			OutputPath:   outputPath,
			Content:      content,
			IsTemplate:   false,
		})
	}

	return results
}

// instantiateTemplate generates a concrete class by substituting type parameters in a template.
// It performs three substitution passes:
//  1. Replace type parameters (T, K, V) with concrete types
//  2. Replace nested template usages (Queue<Boolean>) with concrete names (QueueBoolean)
//  3. Replace template class name and constructors with concrete name
func (t *Transpiler) instantiateTemplate(template *parser.GenericClassDef, instantiation *parser.GenericExpr) string {
	if len(template.TypeParams) != len(instantiation.TypeArgs) {
		// Mismatch in type parameter count - return error comment
		return fmt.Sprintf("// ERROR: Type parameter mismatch for %s (expected %d, got %d)",
			template.ClassName, len(template.TypeParams), len(instantiation.TypeArgs))
	}

	// Build substitution map for type parameters
	// IMPORTANT: For complex type arguments (e.g., List<Integer>), we must preserve
	// the full generic expression, not flatten it to a concrete class name.
	// This ensures that "T" in "List<T>" becomes "List<Integer>" not "ListInteger".
	substitutions := make(map[string]string, len(template.TypeParams))
	for i, param := range template.TypeParams {
		typeArg := instantiation.TypeArgs[i]
		// Use String() to preserve the generic expression (List<Integer>)
		// instead of GenerateConcreteClassName which would flatten it (ListInteger)
		substitutions[param] = typeArg.String()
	}

	// Pass 1: Replace type parameters with concrete types
	output := template.Body
	for param, concreteType := range substitutions {
		output = replaceTypeParameter(output, param, concreteType)
	}

	// Pass 2: Replace nested generic template usages (e.g., Queue<Boolean> -> QueueBoolean)
	p := parser.NewParser(output)
	if generics, err := p.FindGenerics(); err == nil {
		output = t.replaceGenericUsages(output, generics)
	}

	// Pass 3: Replace class name in declaration and constructors
	concreteName := parser.GenerateConcreteClassName(instantiation)
	// Remove type parameters from class declaration
	output = strings.Replace(output, "<"+strings.Join(template.TypeParams, ", ")+">", "", 1)
	// Replace template class name with concrete name (affects constructors too)
	output = replaceTypeParameter(output, template.ClassName, concreteName)

	// Build final class with concrete name
	return fmt.Sprintf("public class %s %s", concreteName, output)
}

// replaceTypeParameter replaces all occurrences of param with concreteType, respecting word boundaries.
// It ensures that 'T' in "String" is not replaced, only standalone 'T' tokens.
func replaceTypeParameter(input, param, concreteType string) string {
	var result strings.Builder
	result.Grow(len(input)) // Pre-allocate to reduce allocations

	for i := 0; i < len(input); {
		// Check if we found the parameter at this position
		if i+len(param) <= len(input) && input[i:i+len(param)] == param {
			// Verify word boundaries to avoid partial matches
			before := i == 0 || !isIdentifierChar(rune(input[i-1]))
			after := i+len(param) >= len(input) || !isIdentifierChar(rune(input[i+len(param)]))

			if before && after {
				result.WriteString(concreteType)
				i += len(param)
				continue
			}
		}
		result.WriteByte(input[i])
		i++
	}

	return result.String()
}

// isIdentifierChar reports whether r can be part of an Apex identifier.
func isIdentifierChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
