// Package parser provides a recursive descent parser for Peak generic syntax.
//
// The parser extracts generic expressions (e.g., Queue<Integer>) and generic
// class definitions (e.g., class Queue<T>) from Peak source code. It handles
// nested generics, multiple type parameters, and distinguishes generic syntax
// from comparison operators.
//
// The parser uses minimal intervention: it only parses generic-related syntax
// and leaves all other Apex code untouched.
package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseError represents a parsing error with location information
type ParseError struct {
	Message string
	Line    int
	Column  int
	File    string
	Source  string // The source line where the error occurred
}

func (e *ParseError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
}

// FormatError returns a user-friendly formatted error with source context
func (e *ParseError) FormatError() string {
	var result strings.Builder

	if e.File != "" {
		result.WriteString(fmt.Sprintf("%s:%d:%d: error: %s\n", e.File, e.Line, e.Column, e.Message))
	} else {
		result.WriteString(fmt.Sprintf("line %d, column %d: error: %s\n", e.Line, e.Column, e.Message))
	}

	if e.Source != "" {
		result.WriteString(e.Source)
		result.WriteString("\n")

		// Add the pointer line with ^
		for i := 0; i < e.Column-1; i++ {
			if i < len(e.Source) && e.Source[i] == '\t' {
				result.WriteString("\t")
			} else {
				result.WriteString(" ")
			}
		}
		result.WriteString("^\n")
	}

	return result.String()
}

// GenericExpr represents a parsed generic expression
type GenericExpr struct {
	BaseType string        // e.g., "Foo"
	TypeArgs []GenericExpr // e.g., [GenericExpr{BaseType: "Integer"}]
	IsSimple bool          // true if this is just a simple type like "Integer"
}

// GenericClassDef represents a generic class definition
type GenericClassDef struct {
	ClassName  string   // e.g., "Queue"
	TypeParams []string // e.g., ["T"]
	Body       string   // The class body with generic type parameters
	StartPos   int      // Start position in source
	EndPos     int      // End position in source
}

// Parser handles the parsing of Peak source code
type Parser struct {
	input    string
	pos      int
	fileName string // Optional file name for better error messages
}

// NewParser creates a new parser for the given input string.
func NewParser(input string) *Parser {
	return &Parser{
		input: input,
	}
}

// SetFileName sets the file name for better error messages.
func (p *Parser) SetFileName(fileName string) {
	p.fileName = fileName
}

// getLineAndColumn calculates the line and column number for the current position
func (p *Parser) getLineAndColumn(pos int) (line int, column int) {
	line = 1
	column = 1

	for i := 0; i < pos && i < len(p.input); i++ {
		if p.input[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}

	return line, column
}

// getSourceLine extracts the source line at the given position
func (p *Parser) getSourceLine(pos int) string {
	// Find start of line
	start := pos
	for start > 0 && p.input[start-1] != '\n' {
		start--
	}

	// Find end of line
	end := pos
	for end < len(p.input) && p.input[end] != '\n' {
		end++
	}

	return p.input[start:end]
}

// createError creates a ParseError at the current position
func (p *Parser) createError(pos int, message string) *ParseError {
	line, column := p.getLineAndColumn(pos)
	source := p.getSourceLine(pos)

	return &ParseError{
		Message: message,
		Line:    line,
		Column:  column,
		File:    p.fileName,
		Source:  source,
	}
}

// current returns the current character without advancing
func (p *Parser) current() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

// peek returns the character at offset from current position
func (p *Parser) peek(offset int) byte {
	pos := p.pos + offset
	if pos >= len(p.input) {
		return 0
	}
	return p.input[pos]
}

// advance moves the position forward by n characters
func (p *Parser) advance(n int) {
	p.pos += n
	if p.pos > len(p.input) {
		p.pos = len(p.input)
	}
}

// skipWhitespace skips whitespace characters
func (p *Parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.current())) {
		p.advance(1)
	}
}

// parseIdentifier parses an identifier (alphanumeric + underscore)
func (p *Parser) parseIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) {
		c := rune(p.current())
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' {
			break
		}
		p.advance(1)
	}
	return p.input[start:p.pos]
}

// ParseGeneric parses a generic expression like "Foo<Integer>" or "Map<String, List<Integer>>".
// This function is called when we encounter a '<' after an identifier.
//
// It recursively handles nested generics and validates syntax.
func (p *Parser) ParseGeneric(baseType string) (*GenericExpr, error) {
	expr := &GenericExpr{
		BaseType: baseType,
		TypeArgs: []GenericExpr{},
		IsSimple: false,
	}

	// We expect to be at '<'
	if p.current() != '<' {
		return nil, p.createError(p.pos, "expected '<'")
	}
	p.advance(1) // skip '<'

	// Parse type arguments
	for {
		p.skipWhitespace()

		// Parse the type argument (could be another generic)
		typeArg, err := p.parseTypeArgument()
		if err != nil {
			return nil, err
		}
		expr.TypeArgs = append(expr.TypeArgs, *typeArg)

		p.skipWhitespace()

		// Check what comes next
		if p.current() == '>' {
			p.advance(1) // skip '>'
			break
		} else if p.current() == ',' {
			p.advance(1) // skip ','
			continue
		} else {
			return nil, p.createError(p.pos, fmt.Sprintf("expected '>' or ',', got '%c'", p.current()))
		}
	}

	return expr, nil
}

// parseTypeArgument parses a single type argument, which could be:
//   - A simple type like "Integer"
//   - A nested generic like "List<String>"
//
// This method enables recursive parsing of nested generic structures.
func (p *Parser) parseTypeArgument() (*GenericExpr, error) {
	p.skipWhitespace()

	// Parse the base type name
	typeName := p.parseIdentifier()
	if typeName == "" {
		return nil, p.createError(p.pos, "expected type name")
	}

	p.skipWhitespace()

	// Check if this is a generic type (followed by '<')
	if p.current() == '<' {
		return p.ParseGeneric(typeName)
	}

	// Simple type
	return &GenericExpr{
		BaseType: typeName,
		TypeArgs: []GenericExpr{},
		IsSimple: true,
	}, nil
}

// FindGenerics scans through the input and finds all generic expressions.
// It returns a map from original expression text to parsed GenericExpr.
// Built-in Apex generic types (List, Set, Map) are excluded.
func (p *Parser) FindGenerics() (map[string]*GenericExpr, error) {
	generics := make(map[string]*GenericExpr)

	for p.pos < len(p.input) {
		// Skip until we find an identifier
		if !unicode.IsLetter(rune(p.current())) && p.current() != '_' {
			p.advance(1)
			continue
		}

		// Parse identifier
		start := p.pos
		identifier := p.parseIdentifier()

		// Check if followed by '<'
		p.skipWhitespace()
		if p.current() == '<' {
			// This might be a generic expression
			// We need to check it's not a comparison operator
			if p.peek(1) != '=' && !unicode.IsSpace(rune(p.peek(1))) {
				// Try to parse as generic
				savedPos := p.pos
				expr, err := p.ParseGeneric(identifier)
				if err != nil {
					// Not a valid generic, restore position and continue
					p.pos = savedPos + 1
					continue
				}

				// Skip built-in Apex generic types (List, Set, Map)
				if !isBuiltInGeneric(expr.BaseType) {
					// Successfully parsed a generic
					originalText := p.input[start:p.pos]
					generics[originalText] = expr

					// Also collect all nested generics (excluding built-ins)
					collectNestedGenerics(expr, generics)
				}
			}
		}
	}

	return generics, nil
}

// isBuiltInGeneric reports whether typeName is a built-in Apex generic type.
func isBuiltInGeneric(typeName string) bool {
	switch typeName {
	case "List", "Set", "Map":
		return true
	default:
		return false
	}
}

// collectNestedGenerics recursively collects all nested generic expressions
func collectNestedGenerics(expr *GenericExpr, generics map[string]*GenericExpr) {
	for _, typeArg := range expr.TypeArgs {
		if !typeArg.IsSimple && !isBuiltInGeneric(typeArg.BaseType) {
			// This is a nested generic and not a built-in type
			generics[typeArg.String()] = &typeArg
			// Recursively collect from this one too
			collectNestedGenerics(&typeArg, generics)
		}
	}
}

// GenerateConcreteClassName generates a concrete class name from a generic expression.
// Examples:
//   - Queue<Integer> → QueueInteger
//   - Dict<String, Integer> → DictStringInteger
//   - Queue<List<Integer>> → QueueListInteger
func GenerateConcreteClassName(expr *GenericExpr) string {
	parts := make([]string, 0, 1+len(expr.TypeArgs))
	parts = append(parts, expr.BaseType)

	for _, typeArg := range expr.TypeArgs {
		if typeArg.IsSimple {
			parts = append(parts, typeArg.BaseType)
		} else {
			parts = append(parts, GenerateConcreteClassName(&typeArg))
		}
	}

	return strings.Join(parts, "")
}

// String returns a string representation of the generic expression
func (g *GenericExpr) String() string {
	if g.IsSimple {
		return g.BaseType
	}

	args := make([]string, len(g.TypeArgs))
	for i, arg := range g.TypeArgs {
		args[i] = arg.String()
	}

	return fmt.Sprintf("%s<%s>", g.BaseType, strings.Join(args, ", "))
}

// FindGenericClassDefinitions scans for generic class definitions.
// It finds patterns like "class Queue<T>" or "class Dict<K, V>".
// Returns a map from class name to GenericClassDef.
func (p *Parser) FindGenericClassDefinitions() (map[string]*GenericClassDef, error) {
	definitions := make(map[string]*GenericClassDef)

	// Reset parser position
	originalPos := p.pos
	p.pos = 0

	for p.pos < len(p.input) {
		// Look for "class" keyword
		if !p.matchKeyword("class") {
			p.advance(1)
			continue
		}

		// Skip "class" keyword
		p.pos += 5
		p.skipWhitespace()

		// Parse class name
		className := p.parseIdentifier()
		if className == "" {
			continue
		}

		p.skipWhitespace()

		// Check if this is a generic class (has <T> after class name)
		if p.current() != '<' {
			continue
		}

		// Parse type parameters
		startPos := p.pos - len(className) - 5 // Include "class "
		typeParams, err := p.parseTypeParameters()
		if err != nil {
			p.pos = originalPos
			return nil, err
		}

		// Find the class body
		body, endPos := p.extractClassBody()

		definitions[className] = &GenericClassDef{
			ClassName:  className,
			TypeParams: typeParams,
			Body:       body,
			StartPos:   startPos,
			EndPos:     endPos,
		}
	}

	p.pos = originalPos
	return definitions, nil
}

// matchKeyword checks if the current position matches a keyword
func (p *Parser) matchKeyword(keyword string) bool {
	if p.pos+len(keyword) > len(p.input) {
		return false
	}

	// Check if previous character is not alphanumeric (word boundary)
	if p.pos > 0 {
		prev := rune(p.input[p.pos-1])
		if unicode.IsLetter(prev) || unicode.IsDigit(prev) {
			return false
		}
	}

	// Check if keyword matches
	if p.input[p.pos:p.pos+len(keyword)] != keyword {
		return false
	}

	// Check if next character is not alphanumeric (word boundary)
	if p.pos+len(keyword) < len(p.input) {
		next := rune(p.input[p.pos+len(keyword)])
		if unicode.IsLetter(next) || unicode.IsDigit(next) {
			return false
		}
	}

	return true
}

// parseTypeParameters parses type parameters like <T> or <T, U>
func (p *Parser) parseTypeParameters() ([]string, error) {
	if p.current() != '<' {
		return nil, p.createError(p.pos, "expected '<'")
	}

	// Check for << syntax error
	if p.peek(1) == '<' {
		return nil, p.createError(p.pos, "'<<' is not allowed in type parameters")
	}

	p.advance(1)

	var params []string
	for {
		p.skipWhitespace()

		// Check for >> syntax error
		if p.current() == '>' && p.peek(1) == '>' {
			return nil, p.createError(p.pos, "'>>' is not allowed in type parameters")
		}

		paramStart := p.pos
		param := p.parseIdentifier()
		if param == "" {
			return nil, p.createError(p.pos, "expected type parameter")
		}

		// Validate single-letter type parameter
		if len(param) != 1 {
			return nil, p.createError(paramStart, fmt.Sprintf("type parameter '%s' must be a single letter (e.g., T, U, V)", param))
		}

		// Validate it's a letter
		if !unicode.IsLetter(rune(param[0])) {
			return nil, p.createError(paramStart, fmt.Sprintf("type parameter '%s' must be a letter", param))
		}

		// Check for duplicate parameters
		for _, existingParam := range params {
			if existingParam == param {
				return nil, p.createError(paramStart, fmt.Sprintf("duplicate type parameter '%s'", param))
			}
		}

		params = append(params, param)

		p.skipWhitespace()

		// Check for >> syntax error before normal >
		if p.current() == '>' {
			if p.peek(1) == '>' {
				return nil, p.createError(p.pos, "'>>' is not allowed in type parameters")
			}
			p.advance(1)
			break
		} else if p.current() == ',' {
			p.advance(1)
			continue
		} else {
			return nil, p.createError(p.pos, "expected '>' or ','")
		}
	}

	return params, nil
}

// extractClassBody extracts the class body from current position
func (p *Parser) extractClassBody() (string, int) {
	p.skipWhitespace()

	// Find the opening brace
	for p.pos < len(p.input) && p.current() != '{' {
		p.advance(1)
	}

	if p.pos >= len(p.input) {
		return "", p.pos
	}

	startBody := p.pos
	p.advance(1) // skip '{'

	// Find matching closing brace
	braceCount := 1
	for p.pos < len(p.input) && braceCount > 0 {
		if p.current() == '{' {
			braceCount++
		} else if p.current() == '}' {
			braceCount--
		}
		p.advance(1)
	}

	endBody := p.pos
	return p.input[startBody:endBody], endBody
}
