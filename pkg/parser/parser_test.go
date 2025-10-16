package parser

import (
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
			name:  "ignore built-in List, Set, Map",
			input: "List<String> list; Set<Integer> set; Map<String, Integer> map;",
			expected: map[string]string{},
		},
		{
			name:  "ignore comparison operators",
			input: "if (x < 5) { return true; }",
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
