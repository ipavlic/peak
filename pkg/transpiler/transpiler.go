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

	"peak/pkg/parser"
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
}

// NewTranspiler creates a new transpiler
func NewTranspiler() *Transpiler {
	return &Transpiler{
		templates:     make(map[string]*parser.GenericClassDef),
		templatePaths: make(map[string]string),
		usages:        make(map[string]*parser.GenericExpr),
	}
}

// TranspileFiles processes multiple files and generates concrete classes
func (t *Transpiler) TranspileFiles(files map[string]string) ([]FileResult, error) {
	var results []FileResult
	hasErrors := false

	// Phase 1: Collect all generic class definitions (templates)
	for path, content := range files {
		p := parser.NewParser(content)
		p.SetFileName(path)
		defs, err := p.FindGenericClassDefinitions()
		if err != nil {
			hasErrors = true
			results = append(results, FileResult{
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

	// Phase 2: Collect all generic instantiations
	for path, content := range files {
		// Check if this file defines templates
		p := parser.NewParser(content)
		defs, _ := p.FindGenericClassDefinitions()

		var contentToScan string
		if len(defs) > 0 {
			// This file defines templates - scan only the class bodies, not the declarations
			// This prevents "class Queue<T>" from being treated as a usage of Queue<T>
			var bodies []string
			for _, def := range defs {
				bodies = append(bodies, def.Body)
			}
			contentToScan = strings.Join(bodies, "\n")
		} else {
			// Not a template file - scan the entire content
			contentToScan = content
		}

		// Collect generic usages from the content
		p = parser.NewParser(contentToScan)
		p.SetFileName(path)
		generics, err := p.FindGenerics()
		if err != nil {
			hasErrors = true
			// Check if we already have an error for this file
			found := false
			for i, r := range results {
				if r.OriginalPath == path && r.Error != nil {
					// Append to existing error
					results[i].Error = err
					found = true
					break
				}
			}
			if !found {
				results = append(results, FileResult{
					OriginalPath: path,
					Error:        err,
				})
			}
			continue
		}

		for original, expr := range generics {
			// Only track usages of our defined templates
			if _, isTemplate := t.templates[expr.BaseType]; isTemplate {
				t.usages[original] = expr
			}
		}
	}

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

	// Generate output path
	outputPath := strings.TrimSuffix(path, ".peak") + ".cls"

	return FileResult{
		OriginalPath: path,
		OutputPath:   outputPath,
		Content:      output,
		IsTemplate:   false,
	}, nil
}

// replaceGenericUsages replaces all generic template usages in content with concrete class names.
// It sorts generics by length (longest first) to handle nested generics correctly.
func (t *Transpiler) replaceGenericUsages(content string, generics map[string]*parser.GenericExpr) string {
	// Extract and sort keys by length (longest first) to handle nested generics
	sortedKeys := make([]string, 0, len(generics))
	for key := range generics {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		return len(sortedKeys[i]) > len(sortedKeys[j])
	})

	output := content
	for _, original := range sortedKeys {
		expr := generics[original]
		// Only replace if it's a usage of a known template
		if _, isTemplate := t.templates[expr.BaseType]; isTemplate {
			concrete := parser.GenerateConcreteClassName(expr)
			output = strings.ReplaceAll(output, original, concrete)
		}
	}
	return output
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
		templateDir := filepath.Dir(templatePath)

		// Generate concrete class content
		content := t.instantiateTemplate(template, expr)
		concreteName := parser.GenerateConcreteClassName(expr)
		outputPath := filepath.Join(templateDir, concreteName+".cls")

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
