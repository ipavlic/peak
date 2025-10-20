package parser

import (
	"strings"
	"testing"
)

func TestParseGeneric(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		baseType    string
		expected    string
		shouldError bool
	}{
		{
			name:     "simple generic",
			input:    "<Integer>",
			baseType: "Foo",
			expected: "Foo<Integer>",
		},
		{
			name:     "two type parameters",
			input:    "<String, Integer>",
			baseType: "Map",
			expected: "Map<String, Integer>",
		},
		{
			name:     "nested generic",
			input:    "<List<String>>",
			baseType: "Foo",
			expected: "Foo<List<String>>",
		},
		{
			name:     "complex nested generic",
			input:    "<String, List<Integer>>",
			baseType: "Map",
			expected: "Map<String, List<Integer>>",
		},
		{
			name:     "deeply nested",
			input:    "<Map<String, List<Integer>>>",
			baseType: "Wrapper",
			expected: "Wrapper<Map<String, List<Integer>>>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			expr, err := p.ParseGeneric(tt.baseType)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if expr.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, expr.String())
			}
		})
	}
}

func TestFindGenerics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string // original -> concrete class name
	}{
		{
			name:  "single generic",
			input: "public class Test { Foo<Integer> foo; }",
			expected: map[string]string{
				"Foo<Integer>": "FooInteger",
			},
		},
		{
			name:  "multiple generics",
			input: "public class Test { Foo<Integer> foo; Bar<String> bar; }",
			expected: map[string]string{
				"Foo<Integer>": "FooInteger",
				"Bar<String>":  "BarString",
			},
		},
		{
			name:  "nested generic with built-in types ignored",
			input: "Wrapper<List<Integer>> wrapper;",
			expected: map[string]string{
				"Wrapper<List<Integer>>": "WrapperListInteger",
			},
		},
		{
			name:     "ignore built-in List, Set, Map",
			input:    "List<String> list; Set<Integer> set; Map<String, Integer> map;",
			expected: map[string]string{},
		},
		{
			name:     "ignore comparison operators",
			input:    "if (x < 5) { return true; }",
			expected: map[string]string{},
		},
		{
			name:  "method with generic",
			input: "public Foo<Integer> getFoo() { return new Foo<Integer>(); }",
			expected: map[string]string{
				"Foo<Integer>": "FooInteger",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			generics, err := p.FindGenerics()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(generics) != len(tt.expected) {
				t.Errorf("expected %d generics, got %d", len(tt.expected), len(generics))
			}

			for original, expectedConcrete := range tt.expected {
				expr, ok := generics[original]
				if !ok {
					t.Errorf("expected to find generic %s", original)
					continue
				}

				concrete := GenerateConcreteClassName(expr)
				if concrete != expectedConcrete {
					t.Errorf("for %s, expected concrete name %s, got %s", original, expectedConcrete, concrete)
				}
			}
		})
	}
}

func TestGenerateConcreteClassName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		baseType string
		expected string
	}{
		{
			name:     "simple generic",
			input:    "<Integer>",
			baseType: "Foo",
			expected: "FooInteger",
		},
		{
			name:     "two parameters",
			input:    "<String, Integer>",
			baseType: "Map",
			expected: "MapStringInteger",
		},
		{
			name:     "nested generic",
			input:    "<List<String>>",
			baseType: "Foo",
			expected: "FooListString",
		},
		{
			name:     "complex nested",
			input:    "<String, List<Integer>>",
			baseType: "Map",
			expected: "MapStringListInteger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			expr, err := p.ParseGeneric(tt.baseType)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			concrete := GenerateConcreteClassName(expr)
			if concrete != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, concrete)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		fileName     string
		pos          int
		message      string
		expectLine   int
		expectColumn int
	}{
		{
			name:         "error at start",
			input:        "test",
			fileName:     "test.peak",
			pos:          0,
			message:      "unexpected token",
			expectLine:   1,
			expectColumn: 1,
		},
		{
			name:         "error after newline",
			input:        "line1\nline2",
			fileName:     "test.peak",
			pos:          6,
			message:      "syntax error",
			expectLine:   2,
			expectColumn: 1,
		},
		{
			name:         "error mid-line",
			input:        "hello world",
			fileName:     "",
			pos:          6,
			message:      "error here",
			expectLine:   1,
			expectColumn: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			if tt.fileName != "" {
				p.SetFileName(tt.fileName)
			}
			p.pos = tt.pos

			err := p.createError(tt.pos, tt.message)

			if err.Message != tt.message {
				t.Errorf("expected message %q, got %q", tt.message, err.Message)
			}

			if err.Line != tt.expectLine {
				t.Errorf("expected line %d, got %d", tt.expectLine, err.Line)
			}

			if err.Column != tt.expectColumn {
				t.Errorf("expected column %d, got %d", tt.expectColumn, err.Column)
			}

			if tt.fileName != "" && err.File != tt.fileName {
				t.Errorf("expected file %q, got %q", tt.fileName, err.File)
			}

			// Test Error() method
			errStr := err.Error()
			if errStr == "" {
				t.Error("Error() returned empty string")
			}

			// Test FormatError() method
			formatted := err.FormatError()
			if formatted == "" {
				t.Error("FormatError() returned empty string")
			}
			if !strings.Contains(formatted, tt.message) {
				t.Errorf("FormatError() should contain message %q", tt.message)
			}
		})
	}
}

func TestFindGenericClassDefinitions(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedCount  int
		expectedClass  string
		expectedParams []string
	}{
		{
			name: "simple single type parameter",
			input: `public class Queue<T> {
    private List<T> items;
}`,
			expectedCount:  1,
			expectedClass:  "Queue",
			expectedParams: []string{"T"},
		},
		{
			name: "multiple type parameters",
			input: `public class Dict<K, V> {
    private Map<K, V> items;
}`,
			expectedCount:  1,
			expectedClass:  "Dict",
			expectedParams: []string{"K", "V"},
		},
		{
			name: "multiple classes",
			input: `public class Foo<T> {
}
public class Bar<U> {
}`,
			expectedCount: 2,
		},
		{
			name: "non-generic class ignored",
			input: `public class Regular {
    private Integer x;
}`,
			expectedCount: 0,
		},
		{
			name: "class with body",
			input: `public class Wrapper<T> {
    private T value;
    public T get() { return value; }
}`,
			expectedCount:  1,
			expectedClass:  "Wrapper",
			expectedParams: []string{"T"},
		},
		{
			name: "with sharing keyword",
			input: `public with sharing class Queue<T> {
    private List<T> items;
}`,
			expectedCount:  1,
			expectedClass:  "Queue",
			expectedParams: []string{"T"},
		},
		{
			name: "without sharing keyword",
			input: `public without sharing class Dict<K, V> {
    private Map<K, V> data;
}`,
			expectedCount:  1,
			expectedClass:  "Dict",
			expectedParams: []string{"K", "V"},
		},
		{
			name: "inherited sharing keyword",
			input: `public inherited sharing class Stack<T> {
    private List<T> items;
}`,
			expectedCount:  1,
			expectedClass:  "Stack",
			expectedParams: []string{"T"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			defs, err := p.FindGenericClassDefinitions()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(defs) != tt.expectedCount {
				t.Errorf("expected %d definitions, got %d", tt.expectedCount, len(defs))
			}

			if tt.expectedClass != "" {
				def, ok := defs[tt.expectedClass]
				if !ok {
					t.Errorf("expected to find class %s", tt.expectedClass)
					return
				}

				if def.ClassName != tt.expectedClass {
					t.Errorf("expected class name %s, got %s", tt.expectedClass, def.ClassName)
				}

				if len(def.TypeParams) != len(tt.expectedParams) {
					t.Errorf("expected %d type params, got %d", len(tt.expectedParams), len(def.TypeParams))
				}

				for i, param := range tt.expectedParams {
					if i < len(def.TypeParams) && def.TypeParams[i] != param {
						t.Errorf("expected param[%d]=%s, got %s", i, param, def.TypeParams[i])
					}
				}
			}
		})
	}
}

func TestFindGenericClassDefinitions_InvalidSharing(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int // Should be 0 for invalid patterns
	}{
		{
			name: "invalid: with without sharing",
			input: `public with class Queue<T> {
    private List<T> items;
}`,
			expectedCount: 0, // Should not be detected
		},
		{
			name: "invalid: without without sharing",
			input: `public without class Queue<T> {
    private List<T> items;
}`,
			expectedCount: 0, // Should not be detected
		},
		{
			name: "invalid: inherited without sharing",
			input: `public inherited class Queue<T> {
    private List<T> items;
}`,
			expectedCount: 0, // Should not be detected
		},
		{
			name: "invalid: sharing without prefix",
			input: `public sharing class Queue<T> {
    private List<T> items;
}`,
			expectedCount: 0, // Should not be detected
		},
		{
			name: "invalid: random word before sharing",
			input: `public foo sharing class Queue<T> {
    private List<T> items;
}`,
			expectedCount: 0, // Should not be detected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			defs, err := p.FindGenericClassDefinitions()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(defs) != tt.expectedCount {
				t.Errorf("expected %d definitions, got %d", tt.expectedCount, len(defs))
				for key := range defs {
					t.Logf("Found: %s", key)
				}
			}
		})
	}
}

func TestFindGenericClassDefinitions_Errors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "duplicate type parameter",
			input:       "public class Foo<T, T> {}",
			expectError: true,
		},
		{
			name:        "invalid type parameter (too long)",
			input:       "public class Foo<Type> {}",
			expectError: true,
		},
		{
			name:        "invalid type parameter (number)",
			input:       "public class Foo<1> {}",
			expectError: true,
		},
		{
			name:        "double angle bracket",
			input:       "public class Foo<<T>> {}",
			expectError: true,
		},
		{
			name:        "closing double angle bracket",
			input:       "public class Foo<T>> {}",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.FindGenericClassDefinitions()

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestCollectNestedGenerics(t *testing.T) {
	// Test deeply nested generics collection
	expr := &GenericExpr{
		BaseType: "Outer",
		TypeArgs: []GenericExpr{
			{
				BaseType: "Middle",
				TypeArgs: []GenericExpr{
					{
						BaseType: "Inner",
						TypeArgs: []GenericExpr{
							{BaseType: "Integer", IsSimple: true},
						},
					},
				},
			},
		},
	}

	generics := make(map[string]*GenericExpr)
	collectNestedGenerics(expr, generics)

	// Should collect Middle<Inner<Integer>> and Inner<Integer>
	if len(generics) < 2 {
		t.Errorf("expected at least 2 nested generics, got %d", len(generics))
	}

	// Check that we collected the nested ones
	foundMiddle := false
	foundInner := false
	for key := range generics {
		if strings.Contains(key, "Middle") {
			foundMiddle = true
		}
		if strings.Contains(key, "Inner") && !strings.Contains(key, "Middle") {
			foundInner = true
		}
	}

	if !foundMiddle {
		t.Error("expected to find Middle generic")
	}
	if !foundInner {
		t.Error("expected to find Inner generic")
	}
}

func TestCollectNestedGenerics_BuiltInIgnored(t *testing.T) {
	// Test that built-in generics are not collected
	expr := &GenericExpr{
		BaseType: "Wrapper",
		TypeArgs: []GenericExpr{
			{
				BaseType: "List",
				TypeArgs: []GenericExpr{
					{BaseType: "Integer", IsSimple: true},
				},
			},
		},
	}

	generics := make(map[string]*GenericExpr)
	collectNestedGenerics(expr, generics)

	// Should not collect List<Integer> because List is built-in
	for key := range generics {
		if strings.Contains(key, "List") {
			t.Errorf("should not collect built-in generic List, but found: %s", key)
		}
	}
}

func TestParserPrimitives(t *testing.T) {
	t.Run("current and peek", func(t *testing.T) {
		p := NewParser("abc")
		if p.current() != 'a' {
			t.Errorf("expected 'a', got '%c'", p.current())
		}
		if p.peek(1) != 'b' {
			t.Errorf("expected 'b', got '%c'", p.peek(1))
		}
		if p.peek(2) != 'c' {
			t.Errorf("expected 'c', got '%c'", p.peek(2))
		}
		if p.peek(10) != 0 {
			t.Errorf("expected 0 for out of bounds, got '%c'", p.peek(10))
		}
	})

	t.Run("advance", func(t *testing.T) {
		p := NewParser("test")
		p.advance(2)
		if p.current() != 's' {
			t.Errorf("expected 's' after advance(2), got '%c'", p.current())
		}
		p.advance(100) // Should not panic
		if p.current() != 0 {
			t.Error("expected 0 at end of input")
		}
	})

	t.Run("skipWhitespace", func(t *testing.T) {
		p := NewParser("   \t\n  test")
		p.skipWhitespace()
		if p.current() != 't' {
			t.Errorf("expected 't' after skipWhitespace, got '%c'", p.current())
		}
	})

	t.Run("parseIdentifier", func(t *testing.T) {
		p := NewParser("myVar123_test")
		id := p.parseIdentifier()
		if id != "myVar123_test" {
			t.Errorf("expected 'myVar123_test', got %q", id)
		}
	})
}

func TestIsBuiltInGeneric(t *testing.T) {
	tests := []struct {
		typeName string
		expected bool
	}{
		{"List", true},
		{"Set", true},
		{"Map", true},
		{"Queue", false},
		{"String", false},
		{"Integer", false},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			result := isBuiltInGeneric(tt.typeName)
			if result != tt.expected {
				t.Errorf("isBuiltInGeneric(%q) = %v, expected %v", tt.typeName, result, tt.expected)
			}
		})
	}
}

func TestFormatError_WithTab(t *testing.T) {
	// Test FormatError with tab character in source line
	input := "hello\tworld"
	p := NewParser(input)
	p.SetFileName("test.peak")

	err := p.createError(6, "error at tab position")
	formatted := err.FormatError()

	if !strings.Contains(formatted, "test.peak") {
		t.Error("formatted error should contain filename")
	}
	if !strings.Contains(formatted, "error at tab position") {
		t.Error("formatted error should contain error message")
	}
	if !strings.Contains(formatted, "^") {
		t.Error("formatted error should contain error pointer")
	}
}

func TestParseGeneric_Errors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		baseType    string
		expectError string
	}{
		{
			name:        "missing closing bracket",
			input:       "<Integer",
			baseType:    "Foo",
			expectError: "expected '>' or ','",
		},
		{
			name:        "invalid character after type arg",
			input:       "<Integer!>",
			baseType:    "Foo",
			expectError: "expected '>' or ','",
		},
		{
			name:        "empty type argument",
			input:       "<>",
			baseType:    "Foo",
			expectError: "expected type name",
		},
		{
			name:        "missing type after comma",
			input:       "<Integer,>",
			baseType:    "Foo",
			expectError: "expected type name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.ParseGeneric(tt.baseType)
			if err == nil {
				t.Error("expected error but got none")
				return
			}
			if tt.expectError != "" && !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("expected error containing %q, got %q", tt.expectError, err.Error())
			}
		})
	}
}

func TestFindGenerics_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "less than with equals",
			input:    "if (x <= 5) { }",
			expected: 0,
		},
		{
			name:     "less than with space",
			input:    "if (x < 5) { }",
			expected: 0,
		},
		{
			name:     "invalid generic that fails to parse",
			input:    "Foo<",
			expected: 0,
		},
		{
			name:     "underscore in identifier",
			input:    "My_Class<Integer> x;",
			expected: 1,
		},
		{
			name:     "multiple occurrences of same generic",
			input:    "Foo<Integer> a; Foo<Integer> b;",
			expected: 1, // Should only collect once
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			generics, err := p.FindGenerics()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(generics) != tt.expected {
				t.Errorf("expected %d generics, got %d", tt.expected, len(generics))
			}
		})
	}
}

func TestMatchKeyword_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		keyword  string
		pos      int
		expected bool
	}{
		{
			name:     "at start of input",
			input:    "class Foo",
			keyword:  "class",
			pos:      0,
			expected: true,
		},
		{
			name:     "at end of input",
			input:    "public class",
			keyword:  "class",
			pos:      7,
			expected: true,
		},
		{
			name:     "not at word boundary (prefix)",
			input:    "myclass Foo",
			keyword:  "class",
			pos:      2,
			expected: false,
		},
		{
			name:     "not at word boundary (suffix)",
			input:    "class2",
			keyword:  "class",
			pos:      0,
			expected: false,
		},
		{
			name:     "keyword too long for remaining input",
			input:    "cl",
			keyword:  "class",
			pos:      0,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			p.pos = tt.pos
			result := p.matchKeyword(tt.keyword)
			if result != tt.expected {
				t.Errorf("matchKeyword(%q) at pos %d = %v, expected %v", tt.keyword, tt.pos, result, tt.expected)
			}
		})
	}
}

func TestParseTypeParameters_AdditionalErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError string
	}{
		{
			name:        "missing closing bracket after param",
			input:       "class Foo<T",
			expectError: "expected '>' or ','",
		},
		{
			name:        "empty after comma",
			input:       "class Foo<T,",
			expectError: "expected type parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			// Position parser at the '<'
			p.pos = strings.Index(tt.input, "<")
			_, err := p.parseTypeParameters()
			if err == nil {
				t.Error("expected error but got none")
				return
			}
			if !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("expected error containing %q, got %q", tt.expectError, err.Error())
			}
		})
	}
}

func TestExtractClassBody_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedBody string
	}{
		{
			name:         "nested braces",
			input:        "{ public void method() { if (true) { } } }",
			expectedBody: "{ public void method() { if (true) { } } }",
		},
		{
			name:         "no opening brace found",
			input:        "no braces here",
			expectedBody: "",
		},
		{
			name:         "empty body",
			input:        "{}",
			expectedBody: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			body, _ := p.extractClassBody()
			if body != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

func TestParseTypeArgument_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "whitespace before type",
			input:       "   Integer>",
			expectError: false,
		},
		{
			name:        "empty input",
			input:       "",
			expectError: true,
		},
		{
			name:        "only whitespace",
			input:       "   ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			_, err := p.parseTypeArgument()
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSkipComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected byte // expected character after skipping
	}{
		{
			name:     "single line comment",
			input:    "// this is a comment\ncode",
			expected: 'c',
		},
		{
			name:     "multi-line comment",
			input:    "/* this is\n a multi-line\n comment */code",
			expected: 'c',
		},
		{
			name:     "no comment",
			input:    "code",
			expected: 'c',
		},
		{
			name:     "multiple single-line comments",
			input:    "// comment 1\n// comment 2\ncode",
			expected: 'c',
		},
		{
			name:     "comment at end",
			input:    "// comment at end",
			expected: 0, // EOF
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			p.skipComments()
			if p.current() != tt.expected {
				t.Errorf("expected '%c', got '%c'", tt.expected, p.current())
			}
		})
	}
}

func TestFindGenerics_WithComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // expected number of generics found
	}{
		{
			name: "single-line comment with generic",
			input: `public class Test {
    // This is a Queue<Integer> in a comment
    private Queue<String> realQueue;
}`,
			expected: 1, // Should only find Queue<String>, not the one in comment
		},
		{
			name: "multi-line comment with generic",
			input: `public class Test {
    /* Here's an example:
       Queue<Integer> myQueue;
    */
    private Queue<String> realQueue;
}`,
			expected: 1, // Should only find Queue<String>
		},
		{
			name: "mixed comments",
			input: `public class Test {
    // Queue<Integer> commented
    private Queue<String> field1; // Queue<Boolean> also commented
    /* Queue<Double> in
       multi-line comment */
    private Queue<Long> field2;
}`,
			expected: 2, // Should find Queue<String> and Queue<Long>
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			generics, err := p.FindGenerics()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(generics) != tt.expected {
				t.Errorf("expected %d generics, got %d", tt.expected, len(generics))
				for key := range generics {
					t.Logf("Found: %s", key)
				}
			}
		})
	}
}

func TestFindGenericClassDefinitions_WithComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // expected number of class definitions found
	}{
		{
			name: "commented class definition",
			input: `
// class Queue<T> { }
public class RealQueue<T> {
    private List<T> items;
}`,
			expected: 1, // Should only find RealQueue<T>
		},
		{
			name: "multi-line comment with class definition",
			input: `
/*
public class Queue<T> {
    private List<T> items;
}
*/
public class RealQueue<T> {
    private List<T> items;
}`,
			expected: 1, // Should only find RealQueue<T>
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input)
			defs, err := p.FindGenericClassDefinitions()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(defs) != tt.expected {
				t.Errorf("expected %d class definitions, got %d", tt.expected, len(defs))
				for key := range defs {
					t.Logf("Found: %s", key)
				}
			}
		})
	}
}
