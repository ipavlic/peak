package transpiler

import (
	"strings"
	"testing"

	"github.com/ipavlic/peak/pkg/config"
	"github.com/ipavlic/peak/pkg/parser"
)

func TestNewTranspiler(t *testing.T) {
	tr := NewTranspiler(nil)
	if tr == nil {
		t.Fatal("NewTranspiler returned nil")
	}
	if tr.templates == nil {
		t.Error("templates map not initialized")
	}
	if tr.templatePaths == nil {
		t.Error("templatePaths map not initialized")
	}
	if tr.usages == nil {
		t.Error("usages map not initialized")
	}
}

func TestTranspileFiles_SimpleTemplate(t *testing.T) {
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Queue.peak": `public class Queue<T> {
    private List<T> items;
    public Queue() { items = new List<T>(); }
    public void enqueue(T item) { items.add(item); }
}`,
		"Example.peak": `public class Example {
    private Queue<Integer> q;
    public Example() { q = new Queue<Integer>(); }
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Check that we got results for: Example.cls, QueueInteger.cls
	// Template file (Queue.peak) should be marked as template
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Find the template result
	var templateResult *FileResult
	for i := range results {
		if results[i].IsTemplate {
			templateResult = &results[i]
			break
		}
	}
	if templateResult == nil {
		t.Fatal("no template result found")
	}
	if templateResult.OriginalPath != "Queue.peak" {
		t.Errorf("expected template path Queue.peak, got %s", templateResult.OriginalPath)
	}

	// Find the Example.cls result
	var exampleResult *FileResult
	for i := range results {
		if results[i].OutputPath == "Example.cls" {
			exampleResult = &results[i]
			break
		}
	}
	if exampleResult == nil {
		t.Fatal("no Example.cls result found")
	}
	if !strings.Contains(exampleResult.Content, "QueueInteger") {
		t.Error("Example.cls should contain QueueInteger")
	}
	if strings.Contains(exampleResult.Content, "Queue<Integer>") {
		t.Error("Example.cls should not contain Queue<Integer>")
	}

	// Find the QueueInteger.cls result
	var concreteResult *FileResult
	for i := range results {
		if strings.Contains(results[i].OutputPath, "QueueInteger.cls") {
			concreteResult = &results[i]
			break
		}
	}
	if concreteResult == nil {
		t.Fatal("no QueueInteger.cls result found")
	}
	if !strings.Contains(concreteResult.Content, "public class QueueInteger") {
		t.Error("concrete class should start with 'public class QueueInteger'")
	}
	if !strings.Contains(concreteResult.Content, "List<Integer>") {
		t.Error("concrete class should contain List<Integer>")
	}
	if !strings.Contains(concreteResult.Content, "public QueueInteger()") {
		t.Error("concrete class should have QueueInteger() constructor")
	}
}

func TestTranspileFiles_MultipleTypeParameters(t *testing.T) {
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Dict.peak": `public class Dict<K, V> {
    private Map<K, V> items;
    public Dict() { items = new Map<K, V>(); }
    public void put(K key, V value) { items.put(key, value); }
}`,
		"Example.peak": `public class Example {
    private Dict<String, Integer> dict;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Find the DictStringInteger.cls result
	var concreteResult *FileResult
	for i := range results {
		if strings.Contains(results[i].OutputPath, "DictStringInteger.cls") {
			concreteResult = &results[i]
			break
		}
	}
	if concreteResult == nil {
		t.Fatal("no DictStringInteger.cls result found")
	}
	if !strings.Contains(concreteResult.Content, "public class DictStringInteger") {
		t.Error("concrete class should start with 'public class DictStringInteger'")
	}
	if !strings.Contains(concreteResult.Content, "Map<String, Integer>") {
		t.Error("concrete class should contain Map<String, Integer>")
	}
	if !strings.Contains(concreteResult.Content, "public void put(String key, Integer value)") {
		t.Error("concrete class should have correctly substituted method signature")
	}
}

func TestTranspileFiles_TransitiveDependencies(t *testing.T) {
	// This test verifies that when a template uses another template,
	// both concrete classes are generated correctly
	// Example: Dict<String, Queue<Integer>> should generate both
	// DictStringQueueInteger and QueueInteger
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Queue.peak": `public class Queue<T> {
    private List<T> items;
}`,
		"Dict.peak": `public class Dict<K, V> {
    private List<K> keys;
    private List<V> values;
}`,
		"Example.peak": `public class Example {
    private Dict<String, Queue<Integer>> dict;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Should generate: Example.cls, DictStringQueueInteger.cls, QueueInteger.cls
	var foundQueueInteger, foundDictStringQueueInteger bool
	for i := range results {
		// Use filepath.Base or exact match to avoid "DictStringQueueInteger.cls" matching "QueueInteger.cls"
		if results[i].OutputPath == "QueueInteger.cls" || strings.HasSuffix(results[i].OutputPath, "/QueueInteger.cls") {
			foundQueueInteger = true
			// Check that it's properly instantiated
			if !strings.Contains(results[i].Content, "List<Integer>") {
				t.Errorf("QueueInteger should contain List<Integer>, got:\n%s", results[i].Content)
			}
		}
		if strings.Contains(results[i].OutputPath, "DictStringQueueInteger.cls") {
			foundDictStringQueueInteger = true
			// Check that nested Queue<Integer> type is replaced with QueueInteger
			if !strings.Contains(results[i].Content, "QueueInteger") {
				t.Error("DictStringQueueInteger should contain QueueInteger")
			}
		}
	}

	if !foundQueueInteger {
		t.Error("QueueInteger.cls not generated (transitive dependency)")
	}
	if !foundDictStringQueueInteger {
		t.Error("DictStringQueueInteger.cls not generated")
	}
}

func TestTranspileFiles_NestedGenerics(t *testing.T) {
	// Tests that nested built-in generics are properly preserved.
	// When Queue<List<Integer>> is instantiated, T should be replaced with "List<Integer>",
	// so List<T> becomes List<List<Integer>>, NOT List<ListInteger>.
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Queue.peak": `public class Queue<T> {
    private List<T> items;
}`,
		"Example.peak": `public class Example {
    private Queue<List<Integer>> q;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Find the QueueListInteger.cls result
	var concreteResult *FileResult
	for i := range results {
		if strings.Contains(results[i].OutputPath, "QueueListInteger.cls") {
			concreteResult = &results[i]
			break
		}
	}
	if concreteResult == nil {
		t.Fatal("no QueueListInteger.cls result found")
	}
	// Correct behavior: List<T> where T=List<Integer> should become List<List<Integer>>
	if !strings.Contains(concreteResult.Content, "List<List<Integer>>") {
		t.Errorf("concrete class should contain List<List<Integer>>, got:\n%s", concreteResult.Content)
	}
	// Make sure it doesn't have the old buggy behavior
	if strings.Contains(concreteResult.Content, "List<ListInteger>") {
		t.Error("concrete class should NOT contain List<ListInteger> (old buggy behavior)")
	}
}

func TestTranspileFiles_ParseError(t *testing.T) {
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Bad.peak": `public class Bad<<T>> {
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles should not return error, got: %v", err)
	}

	// Should have a result with an error
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Error == nil {
		t.Error("expected error in result")
	}
}

func TestTranspileFiles_NoTemplates(t *testing.T) {
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Example.peak": `public class Example {
    private Integer x;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].IsTemplate {
		t.Error("Example.peak should not be marked as template")
	}
	if results[0].OutputPath != "Example.cls" {
		t.Errorf("expected output path Example.cls, got %s", results[0].OutputPath)
	}
}

func TestTranspileFiles_BuiltInGenerics(t *testing.T) {
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Example.peak": `public class Example {
    private List<String> list;
    private Set<Integer> set;
    private Map<String, Integer> map;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Should only generate Example.cls, no concrete classes for built-in generics
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].OutputPath != "Example.cls" {
		t.Errorf("expected Example.cls, got %s", results[0].OutputPath)
	}
	// Built-in generics should remain unchanged
	if !strings.Contains(results[0].Content, "List<String>") {
		t.Error("List<String> should remain unchanged")
	}
	if !strings.Contains(results[0].Content, "Set<Integer>") {
		t.Error("Set<Integer> should remain unchanged")
	}
	if !strings.Contains(results[0].Content, "Map<String, Integer>") {
		t.Error("Map<String, Integer> should remain unchanged")
	}
}

func TestReplaceTypeParameter(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		param        string
		concreteType string
		expected     string
	}{
		{
			name:         "simple replacement",
			input:        "private T item;",
			param:        "T",
			concreteType: "Integer",
			expected:     "private Integer item;",
		},
		{
			name:         "multiple occurrences",
			input:        "public T get() { return (T)item; }",
			param:        "T",
			concreteType: "String",
			expected:     "public String get() { return (String)item; }",
		},
		{
			name:         "no partial match in String",
			input:        "private String s; private T item;",
			param:        "T",
			concreteType: "Integer",
			expected:     "private String s; private Integer item;",
		},
		{
			name:         "word boundary respected",
			input:        "private T item; private Tuple tuple;",
			param:        "T",
			concreteType: "Boolean",
			expected:     "private Boolean item; private Tuple tuple;",
		},
		{
			name:         "class name replacement",
			input:        "public class Queue { public Queue() {} }",
			param:        "Queue",
			concreteType: "QueueInteger",
			expected:     "public class QueueInteger { public QueueInteger() {} }",
		},
		{
			name:         "no replacement when part of identifier",
			input:        "private Testing test;",
			param:        "T",
			concreteType: "String",
			expected:     "private Testing test;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceTypeParameter(tt.input, tt.param, tt.concreteType)
			if result != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestIsIdentifierChar(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'_', true},
		{' ', false},
		{'\t', false},
		{'\n', false},
		{'.', false},
		{'<', false},
		{'>', false},
		{',', false},
		{';', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := isIdentifierChar(tt.char)
			if result != tt.expected {
				t.Errorf("isIdentifierChar(%q) = %v, expected %v", tt.char, result, tt.expected)
			}
		})
	}
}

func TestReplaceGenericUsages(t *testing.T) {
	// Create a transpiler with some templates
	tr := NewTranspiler(nil)
	tr.templates["Queue"] = &parser.GenericClassDef{
		ClassName:  "Queue",
		TypeParams: []string{"T"},
		Body:       "{}",
	}
	tr.templates["Dict"] = &parser.GenericClassDef{
		ClassName:  "Dict",
		TypeParams: []string{"K", "V"},
		Body:       "{}",
	}

	tests := []struct {
		name     string
		input    string
		generics map[string]*parser.GenericExpr
		expected string
	}{
		{
			name:  "single generic replacement",
			input: "private Queue<Integer> q;",
			generics: map[string]*parser.GenericExpr{
				"Queue<Integer>": {
					BaseType: "Queue",
					TypeArgs: []parser.GenericExpr{{BaseType: "Integer", IsSimple: true}},
				},
			},
			expected: "private QueueInteger q;",
		},
		{
			name:  "multiple generics",
			input: "Queue<String> q1; Queue<Integer> q2;",
			generics: map[string]*parser.GenericExpr{
				"Queue<String>": {
					BaseType: "Queue",
					TypeArgs: []parser.GenericExpr{{BaseType: "String", IsSimple: true}},
				},
				"Queue<Integer>": {
					BaseType: "Queue",
					TypeArgs: []parser.GenericExpr{{BaseType: "Integer", IsSimple: true}},
				},
			},
			expected: "QueueString q1; QueueInteger q2;",
		},
		{
			name:  "nested generics - longest first",
			input: "private Queue<List<Integer>> q;",
			generics: map[string]*parser.GenericExpr{
				"Queue<List<Integer>>": {
					BaseType: "Queue",
					TypeArgs: []parser.GenericExpr{
						{
							BaseType: "List",
							TypeArgs: []parser.GenericExpr{{BaseType: "Integer", IsSimple: true}},
						},
					},
				},
			},
			expected: "private QueueListInteger q;",
		},
		{
			name:     "no generics",
			input:    "private Integer x;",
			generics: map[string]*parser.GenericExpr{},
			expected: "private Integer x;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.replaceGenericUsages(tt.input, tt.generics)
			if result != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestInstantiateTemplate(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name          string
		template      *parser.GenericClassDef
		instantiation *parser.GenericExpr
		checks        []string // strings that should appear in output
		notChecks     []string // strings that should NOT appear in output
	}{
		{
			name: "simple single type parameter",
			template: &parser.GenericClassDef{
				ClassName:  "Queue",
				TypeParams: []string{"T"},
				Body: `{
    private List<T> items;
    public Queue() { items = new List<T>(); }
    public void enqueue(T item) { items.add(item); }
}`,
			},
			instantiation: &parser.GenericExpr{
				BaseType: "Queue",
				TypeArgs: []parser.GenericExpr{{BaseType: "Integer", IsSimple: true}},
			},
			checks: []string{
				"public class QueueInteger",
				"List<Integer>",
				"public QueueInteger()",
				"public void enqueue(Integer item)",
			},
			notChecks: []string{
				"<T>",
				"Queue<",
				"List<T>",
			},
		},
		{
			name: "multiple type parameters",
			template: &parser.GenericClassDef{
				ClassName:  "Dict",
				TypeParams: []string{"K", "V"},
				Body: `{
    private Map<K, V> items;
    public Dict() {}
    public void put(K key, V value) {}
}`,
			},
			instantiation: &parser.GenericExpr{
				BaseType: "Dict",
				TypeArgs: []parser.GenericExpr{
					{BaseType: "String", IsSimple: true},
					{BaseType: "Integer", IsSimple: true},
				},
			},
			checks: []string{
				"public class DictStringInteger",
				"Map<String, Integer>",
				"public void put(String key, Integer value)",
			},
			notChecks: []string{
				"<K, V>",
				"<K>",
				"<V>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.instantiateTemplate(tt.template, tt.instantiation)

			for _, check := range tt.checks {
				if !strings.Contains(result, check) {
					t.Errorf("expected output to contain %q\nGot:\n%s", check, result)
				}
			}

			for _, notCheck := range tt.notChecks {
				if strings.Contains(result, notCheck) {
					t.Errorf("expected output NOT to contain %q\nGot:\n%s", notCheck, result)
				}
			}
		})
	}
}

func TestInstantiateTemplate_TypeParameterMismatch(t *testing.T) {
	tr := NewTranspiler(nil)
	template := &parser.GenericClassDef{
		ClassName:  "Queue",
		TypeParams: []string{"T"},
		Body:       "{}",
	}
	instantiation := &parser.GenericExpr{
		BaseType: "Queue",
		TypeArgs: []parser.GenericExpr{
			{BaseType: "String", IsSimple: true},
			{BaseType: "Integer", IsSimple: true},
		},
	}

	result := tr.instantiateTemplate(template, instantiation)
	if !strings.Contains(result, "ERROR") {
		t.Error("expected error comment for type parameter mismatch")
	}
	if !strings.Contains(result, "expected 1, got 2") {
		t.Error("expected error message to mention parameter count")
	}
}

func TestTranspileFiles_ComplexNestedGenerics(t *testing.T) {
	// Tests that built-in generics (List, Set, Map) are preserved while
	// custom templates (Queue, Wrapper) are correctly converted, even in complex nesting.
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Wrapper.peak": `public class Wrapper<T> {
    private T value;
    public T getValue() { return value; }
    public void setValue(T v) { value = v; }
}`,
		"Queue.peak": `public class Queue<T> {
    private List<T> items;
    public T get() { return items[0]; }
}`,
		"Example.peak": `public class Example {
    // Built-in generic with custom template type arg
    private Wrapper<Map<String, Integer>> w1;

    // Custom template with built-in generic type arg
    private Queue<List<String>> q1;

    // Custom template with custom template type arg
    private Wrapper<Queue<Integer>> w2;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	tests := []struct {
		fileName         string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			fileName: "WrapperMapStringInteger.cls",
			shouldContain: []string{
				"public class WrapperMapStringInteger",
				"Map<String, Integer> value",
				"Map<String, Integer> getValue()",
				"void setValue(Map<String, Integer> v)",
			},
			shouldNotContain: []string{
				"MapStringInteger value", // Should NOT flatten built-in generic
			},
		},
		{
			fileName: "QueueListString.cls",
			shouldContain: []string{
				"public class QueueListString",
				"List<List<String>> items",
				"List<String> get()",
			},
			shouldNotContain: []string{
				"List<ListString>", // Should NOT flatten built-in generic
				"ListString get()", // Should NOT flatten built-in generic
			},
		},
		{
			fileName: "WrapperQueueInteger.cls",
			shouldContain: []string{
				"public class WrapperQueueInteger",
				"QueueInteger value", // Custom template should be converted
				"QueueInteger getValue()",
				"void setValue(QueueInteger v)",
			},
			shouldNotContain: []string{
				"Queue<Integer>", // Custom template should be converted
			},
		},
	}

	for _, tt := range tests {
		var result *FileResult
		for i := range results {
			if strings.Contains(results[i].OutputPath, tt.fileName) {
				result = &results[i]
				break
			}
		}

		if result == nil {
			t.Errorf("%s not found", tt.fileName)
			continue
		}

		for _, expected := range tt.shouldContain {
			if !strings.Contains(result.Content, expected) {
				t.Errorf("%s should contain %q\nGot:\n%s", tt.fileName, expected, result.Content)
			}
		}

		for _, unexpected := range tt.shouldNotContain {
			if strings.Contains(result.Content, unexpected) {
				t.Errorf("%s should NOT contain %q\nGot:\n%s", tt.fileName, unexpected, result.Content)
			}
		}
	}
}

func TestTranspileFile(t *testing.T) {
	tr := NewTranspiler(nil)
	tr.templates["Queue"] = &parser.GenericClassDef{
		ClassName:  "Queue",
		TypeParams: []string{"T"},
		Body:       "{}",
	}

	tests := []struct {
		name           string
		path           string
		content        string
		expectTemplate bool
		expectError    bool
		outputPath     string
		checkContent   string
	}{
		{
			name: "non-template file with generics",
			path: "Example.peak",
			content: `public class Example {
    private Queue<Integer> q;
}`,
			expectTemplate: false,
			expectError:    false,
			outputPath:     "Example.cls",
			checkContent:   "QueueInteger",
		},
		{
			name: "template file",
			path: "Queue.peak",
			content: `public class Queue<T> {
    private List<T> items;
}`,
			expectTemplate: true,
			expectError:    false,
		},
		{
			name:           "file without generics",
			path:           "Simple.peak",
			content:        "public class Simple { private Integer x; }",
			expectTemplate: false,
			expectError:    false,
			outputPath:     "Simple.cls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tr.transpileFile(tt.path, tt.content)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result.IsTemplate != tt.expectTemplate {
				t.Errorf("expected IsTemplate=%v, got %v", tt.expectTemplate, result.IsTemplate)
			}

			if !tt.expectTemplate && result.OutputPath != tt.outputPath {
				t.Errorf("expected output path %s, got %s", tt.outputPath, result.OutputPath)
			}

			if tt.checkContent != "" && !strings.Contains(result.Content, tt.checkContent) {
				t.Errorf("expected content to contain %q", tt.checkContent)
			}
		})
	}
}

func TestRecordError(t *testing.T) {
	tr := NewTranspiler(nil)
	results := []FileResult{}

	// Test adding new error
	err1 := &parser.ParseError{Message: "first error", Line: 1, Column: 1}
	tr.recordError("file1.peak", err1, &results)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Error("expected error to be set")
	}
	if results[0].OriginalPath != "file1.peak" {
		t.Errorf("expected path file1.peak, got %s", results[0].OriginalPath)
	}

	// Test updating existing error
	err2 := &parser.ParseError{Message: "second error", Line: 2, Column: 2}
	tr.recordError("file1.peak", err2, &results)

	if len(results) != 1 {
		t.Errorf("expected 1 result (updated), got %d", len(results))
	}
	if results[0].Error != err2 {
		t.Error("expected error to be updated")
	}

	// Test adding error for different file
	err3 := &parser.ParseError{Message: "third error", Line: 3, Column: 3}
	tr.recordError("file2.peak", err3, &results)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestCollectUsages_WithValidUsages(t *testing.T) {
	tr := NewTranspiler(nil)
	results := []FileResult{}

	// First collect templates
	files := map[string]string{
		"Queue.peak": `public class Queue<T> {
    private List<T> items;
}`,
		"Usage.peak": `public class Usage {
    private Queue<Integer> q;
}`,
	}

	tr.collectTemplates(files, &results)

	// Reset results to test collectUsages independently
	results = []FileResult{}

	// Test collectUsages
	hasErrors := tr.collectUsages(files, &results)

	if hasErrors {
		t.Error("expected no errors in collectUsages")
	}

	// Check that Queue<Integer> was collected as a usage
	if len(tr.usages) == 0 {
		t.Error("expected usages to be collected")
	}

	found := false
	for key := range tr.usages {
		if strings.Contains(key, "Queue<Integer>") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find Queue<Integer> in usages")
	}
}

func TestTranspileFile_ParseErrors(t *testing.T) {
	tr := NewTranspiler(nil)

	t.Run("error finding generic class definitions", func(t *testing.T) {
		result, err := tr.transpileFile("Bad.peak", "public class Bad<<T>> {}")

		if err == nil {
			t.Error("expected error but got none")
		}
		if result.Error == nil {
			t.Error("expected result.Error to be set")
		}
		if result.OriginalPath != "Bad.peak" {
			t.Errorf("expected path Bad.peak, got %s", result.OriginalPath)
		}
	})
}

func TestGetContentToScan(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name        string
		content     string
		shouldScan  []string // strings that should be in scanned content
		shouldSkip  []string // strings that should NOT be in scanned content
	}{
		{
			name: "template file - scan only body",
			content: `public class Queue<T> {
    private List<T> items;
    private Queue<Boolean> nested;
}`,
			shouldScan:  []string{"private List<T> items", "private Queue<Boolean> nested"},
			shouldSkip:  []string{}, // In this case, the declaration is part of the body
		},
		{
			name: "non-template file - scan all",
			content: `public class Example {
    private Queue<Integer> q;
}`,
			shouldScan: []string{"public class Example", "private Queue<Integer> q"},
			shouldSkip: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanned := tr.getContentToScan(tt.content)

			for _, expected := range tt.shouldScan {
				if !strings.Contains(scanned, expected) {
					t.Errorf("scanned content should contain %q\nScanned:\n%s", expected, scanned)
				}
			}

			for _, unexpected := range tt.shouldSkip {
				if strings.Contains(scanned, unexpected) {
					t.Errorf("scanned content should NOT contain %q\nScanned:\n%s", unexpected, scanned)
				}
			}
		})
	}
}

func TestGenerateConcreteClasses_NoTemplate(t *testing.T) {
	tr := NewTranspiler(nil)
	// Add a usage for a template that doesn't exist
	tr.usages["Queue<Integer>"] = &parser.GenericExpr{
		BaseType: "Queue",
		TypeArgs: []parser.GenericExpr{{BaseType: "Integer", IsSimple: true}},
	}

	results := tr.generateConcreteClasses()

	// Should handle gracefully (no crash, no output for missing template)
	if len(results) != 0 {
		t.Errorf("expected 0 results when template doesn't exist, got %d", len(results))
	}
}

func TestTranspileFiles_MultipleParseErrors(t *testing.T) {
	tr := NewTranspiler(nil)
	files := map[string]string{
		"Bad1.peak": `public class Bad1<<T>> {}`,
		"Bad2.peak": `public class Bad2<T, T> {}`,
		"Good.peak": `public class Good { private Integer x; }`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles should not return error even with parse errors, got: %v", err)
	}

	// Check that errors were recorded for bad files
	errorCount := 0
	for _, result := range results {
		if result.Error != nil {
			errorCount++
		}
	}

	if errorCount < 2 {
		t.Errorf("expected at least 2 errors, got %d", errorCount)
	}

	// Verify that we received results (errors block Phase 3, but Phase 1 & 2 errors are recorded)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for error files, got %d", len(results))
	}
}

func TestCollectTemplates_Errors(t *testing.T) {
	tr := NewTranspiler(nil)
	results := []FileResult{}

	files := map[string]string{
		"Bad.peak":  `public class Bad<<T>> {}`,
		"Good.peak": `public class Good<T> {}`,
	}

	hasErrors := tr.collectTemplates(files, &results)

	if !hasErrors {
		t.Error("expected collectTemplates to detect errors")
	}

	// Check that Good.peak was still collected
	if _, exists := tr.templates["Good"]; !exists {
		t.Error("Good template should be collected despite error in other file")
	}

	// Check that error was recorded for Bad.peak
	foundError := false
	for _, result := range results {
		if result.OriginalPath == "Bad.peak" && result.Error != nil {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error to be recorded for Bad.peak")
	}
}

func TestReplaceGenericUsages_EmptyGenerics(t *testing.T) {
	tr := NewTranspiler(nil)
	content := "public class Example { private Integer x; }"
	generics := map[string]*parser.GenericExpr{}

	result := tr.replaceGenericUsages(content, generics)

	if result != content {
		t.Error("content should remain unchanged when no generics present")
	}
}

func TestReplaceGenericUsages_BuiltInIgnored(t *testing.T) {
	tr := NewTranspiler(nil)
	// Don't add List to templates - it's built-in
	content := "public class Example { private List<String> list; }"
	generics := map[string]*parser.GenericExpr{
		"List<String>": {
			BaseType: "List",
			TypeArgs: []parser.GenericExpr{{BaseType: "String", IsSimple: true}},
		},
	}

	result := tr.replaceGenericUsages(content, generics)

	// Built-in generics should not be replaced
	if !strings.Contains(result, "List<String>") {
		t.Error("built-in generic List<String> should remain unchanged")
	}
	if strings.Contains(result, "ListString") {
		t.Error("built-in generic should not be converted to concrete name")
	}
}

func TestTranspileFiles_WithErrorInPhase3(t *testing.T) {
	// Test a scenario where Phase 3 (transpileFile) encounters an error
	// This is difficult to trigger naturally, so we test the error recording path explicitly
	tr := NewTranspiler(nil)

	// Create a file that will pass template collection but fail in transpilation
	files := map[string]string{
		"Test.peak": "public class Test<<BadSyntax>> {}",
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles should not return error, got: %v", err)
	}

	// Check that error was recorded
	foundError := false
	for _, result := range results {
		if result.Error != nil {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error to be recorded in results")
	}
}

func TestReplaceGenericUsages_PreservesComments(t *testing.T) {
	tr := NewTranspiler(nil)
	tr.templates["Queue"] = &parser.GenericClassDef{
		ClassName:  "Queue",
		TypeParams: []string{"T"},
		Body:       "{}",
	}

	content := `public class Test {
    // Comment with Queue<Integer>
    private Queue<String> field1;
    /* Multi-line
       Queue<Boolean> here
    */
    private Queue<Long> field2;
}`

	generics := map[string]*parser.GenericExpr{
		"Queue<String>": {
			BaseType: "Queue",
			TypeArgs: []parser.GenericExpr{{BaseType: "String", IsSimple: true}},
		},
		"Queue<Long>": {
			BaseType: "Queue",
			TypeArgs: []parser.GenericExpr{{BaseType: "Long", IsSimple: true}},
		},
	}

	result := tr.replaceGenericUsages(content, generics)

	// Should replace actual usages
	if !strings.Contains(result, "QueueString field1") {
		t.Error("should replace Queue<String> with QueueString outside comments")
	}
	if !strings.Contains(result, "QueueLong field2") {
		t.Error("should replace Queue<Long> with QueueLong outside comments")
	}

	// Should NOT replace in comments
	if !strings.Contains(result, "// Comment with Queue<Integer>") {
		t.Error("should preserve Queue<Integer> in single-line comment")
	}
	if !strings.Contains(result, "Queue<Boolean> here") {
		t.Error("should preserve Queue<Boolean> in multi-line comment")
	}
}

func TestSetInstantiations(t *testing.T) {
	tr := NewTranspiler(nil)
	instantiations := []string{"Queue<Boolean>", "Optional<Double>"}

	tr.SetInstantiations(instantiations)

	if len(tr.instantiations) != 2 {
		t.Errorf("expected 2 instantiations, got %d", len(tr.instantiations))
	}
	if tr.instantiations[0] != "Queue<Boolean>" {
		t.Errorf("expected first instantiation to be Queue<Boolean>, got %s", tr.instantiations[0])
	}
}

func TestSetInstantiateSpec(t *testing.T) {
	tr := NewTranspiler(nil)
	spec := &config.InstantiateSpec{
		Classes: map[string][]string{
			"Queue": {"Integer", "String"},
		},
		Methods: map[string][]string{
			"Repository.get": {"Account", "Contact"},
		},
	}

	tr.SetInstantiateSpec(spec)

	if tr.instantiateSpec == nil {
		t.Fatal("instantiateSpec should be set")
	}
	if len(tr.instantiateSpec.Classes) != 1 {
		t.Errorf("expected 1 class in spec, got %d", len(tr.instantiateSpec.Classes))
	}
	if len(tr.instantiateSpec.Methods) != 1 {
		t.Errorf("expected 1 method in spec, got %d", len(tr.instantiateSpec.Methods))
	}
}

func TestParseInstantiation(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name        string
		input       string
		expectError bool
		checkType   string
	}{
		{
			name:        "simple instantiation",
			input:       "Queue<Integer>",
			expectError: false,
			checkType:   "Queue",
		},
		{
			name:        "multiple type params",
			input:       "Dict<String, Integer>",
			expectError: false,
			checkType:   "Dict",
		},
		{
			name:        "no generic expression",
			input:       "JustAClass",
			expectError: true,
		},
		{
			name:        "invalid syntax",
			input:       "Queue<<T>>",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := tr.parseInstantiation(tt.input)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if expr == nil {
					t.Fatal("expected expression but got nil")
				}
				if expr.BaseType != tt.checkType {
					t.Errorf("expected base type %s, got %s", tt.checkType, expr.BaseType)
				}
			}
		})
	}
}

func TestProcessInstantiations(t *testing.T) {
	tr := NewTranspiler(nil)

	// Add a template
	tr.templates["Queue"] = &parser.GenericClassDef{
		ClassName:  "Queue",
		TypeParams: []string{"T"},
		Body:       "{}",
	}

	tests := []struct {
		name            string
		instantiations  []string
		expectErrors    bool
		expectedUsages  int
	}{
		{
			name:            "valid instantiation",
			instantiations:  []string{"Queue<Integer>"},
			expectErrors:    false,
			expectedUsages:  1,
		},
		{
			name:            "template not found",
			instantiations:  []string{"NonExistent<String>"},
			expectErrors:    true,
			expectedUsages:  0,
		},
		{
			name:            "invalid syntax",
			instantiations:  []string{"Queue<<Bad>>"},
			expectErrors:    true,
			expectedUsages:  0,
		},
		{
			name:            "empty list",
			instantiations:  []string{},
			expectErrors:    false,
			expectedUsages:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.instantiations = tt.instantiations
			tr.usages = make(map[string]*parser.GenericExpr)
			results := []FileResult{}

			hasErrors := tr.processInstantiations(&results)

			if tt.expectErrors != hasErrors {
				t.Errorf("expected errors=%v, got %v", tt.expectErrors, hasErrors)
			}

			if len(tr.usages) != tt.expectedUsages {
				t.Errorf("expected %d usages, got %d", tt.expectedUsages, len(tr.usages))
			}
		})
	}
}

func TestProcessMethodInstantiations(t *testing.T) {
	tr := NewTranspiler(nil)

	// Add a method template
	tr.methodTemplates["Repository.get"] = &parser.GenericMethodDef{
		ClassName:  "Repository",
		MethodName: "get",
		TypeParams: []string{"T"},
		Signature:  "public <T> T get(String key)",
		Body:       "{ return (T) cache.get(key); }",
	}

	tests := []struct {
		name           string
		spec           *config.InstantiateSpec
		expectErrors   bool
		expectedUsages int
	}{
		{
			name: "valid method instantiation",
			spec: &config.InstantiateSpec{
				Methods: map[string][]string{
					"Repository.get": {"Account", "Contact"},
				},
			},
			expectErrors:   false,
			expectedUsages: 2,
		},
		{
			name: "method not found",
			spec: &config.InstantiateSpec{
				Methods: map[string][]string{
					"NonExistent.method": {"String"},
				},
			},
			expectErrors:   true,
			expectedUsages: 0,
		},
		{
			name:           "empty spec",
			spec:           nil,
			expectErrors:   false,
			expectedUsages: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.instantiateSpec = tt.spec
			tr.methodUsages = make(map[string][]string)
			results := []FileResult{}

			hasErrors := tr.processMethodInstantiations(&results)

			if tt.expectErrors != hasErrors {
				t.Errorf("expected errors=%v, got %v", tt.expectErrors, hasErrors)
			}

			if tt.spec != nil && tt.spec.Methods != nil {
				for methodKey := range tt.spec.Methods {
					if usages, exists := tr.methodUsages[methodKey]; exists {
						if len(usages) != tt.expectedUsages {
							t.Errorf("expected %d usages for %s, got %d", tt.expectedUsages, methodKey, len(usages))
						}
					} else if !tt.expectErrors {
						t.Errorf("expected usages for %s to exist", methodKey)
					}
				}
			}
		})
	}
}

func TestInstantiateMethod(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name         string
		methodDef    *parser.GenericMethodDef
		typeArgs     []string
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "single type parameter",
			methodDef: &parser.GenericMethodDef{
				ClassName:  "Repository",
				MethodName: "get",
				TypeParams: []string{"T"},
				Signature:  "public <T> T get(String key)",
				Body:       "{ return (T) cache.get(key); }",
			},
			typeArgs: []string{"Account"},
			shouldContain: []string{
				"public  Account getAccount(String key)",
				"return (Account) cache.get(key)",
			},
			shouldNotContain: []string{
				"<T>",
				"(T)",
			},
		},
		{
			name: "multiple type parameters",
			methodDef: &parser.GenericMethodDef{
				ClassName:  "Repository",
				MethodName: "transform",
				TypeParams: []string{"K", "V"},
				Signature:  "public <K, V> Map<K, V> transform(K key, V value)",
				Body:       "{ return new Map<K, V>(); }",
			},
			typeArgs: []string{"String", "Integer"},
			shouldContain: []string{
				"public  Map<String, Integer> transformStringInteger",
				"return new Map<String, Integer>",
			},
			shouldNotContain: []string{
				"<K, V>",
				"<K>",
				"<V>",
			},
		},
		{
			name: "parameter count mismatch",
			methodDef: &parser.GenericMethodDef{
				ClassName:  "Repository",
				MethodName: "get",
				TypeParams: []string{"T"},
				Signature:  "public <T> T get(String key)",
				Body:       "{}",
			},
			typeArgs: []string{"String", "Integer"},
			shouldContain: []string{
				"ERROR",
				"expected 1, got 2",
			},
			shouldNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.instantiateMethod(tt.methodDef, tt.typeArgs)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q\nGot:\n%s", expected, result)
				}
			}

			for _, unexpected := range tt.shouldNotContain {
				if strings.Contains(result, unexpected) {
					t.Errorf("expected result NOT to contain %q\nGot:\n%s", unexpected, result)
				}
			}
		})
	}
}

func TestInsertMethods(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name           string
		content        string
		methods        []string
		shouldContain  []string
	}{
		{
			name: "insert single method",
			content: `public class Repository {
    private Map<String, Object> cache;
}`,
			methods: []string{
				"public Account getAccount(String key) { return (Account) cache.get(key); }",
			},
			shouldContain: []string{
				"// Generated concrete methods",
				"public Account getAccount",
			},
		},
		{
			name: "insert multiple methods",
			content: `public class Repository {
    private Map<String, Object> cache;
}`,
			methods: []string{
				"public Account getAccount(String key) { return (Account) cache.get(key); }",
				"public Contact getContact(String key) { return (Contact) cache.get(key); }",
			},
			shouldContain: []string{
				"getAccount",
				"getContact",
			},
		},
		{
			name: "no closing brace",
			content: `public class Repository {
    private Map<String, Object> cache;`,
			methods: []string{
				"public Account getAccount(String key) {}",
			},
			shouldContain: []string{
				"private Map<String, Object> cache;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.insertMethods(tt.content, tt.methods)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(result, expected) {
					t.Errorf("expected result to contain %q\nGot:\n%s", expected, result)
				}
			}
		})
	}
}

func TestCollectMethodTemplates(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name           string
		files          map[string]string
		expectErrors   bool
		expectedMethods int
	}{
		{
			name: "single generic method",
			files: map[string]string{
				"Repository.peak": `public class Repository {
    public <T> T get(String key) { return (T) cache.get(key); }
}`,
			},
			expectErrors:    false,
			expectedMethods: 1,
		},
		{
			name: "multiple generic methods",
			files: map[string]string{
				"Repository.peak": `public class Repository {
    public <T> T get(String key) { return (T) cache.get(key); }
    public <T> void put(String key, T value) { cache.put(key, value); }
}`,
			},
			expectErrors:    false,
			expectedMethods: 2,
		},
		{
			name: "generic method in template class",
			files: map[string]string{
				"Queue.peak": `public class Queue<T> {
    public <K> Map<K, List<T>> groupBy(String field) { return new Map<K, List<T>>(); }
}`,
			},
			expectErrors:    false,
			expectedMethods: 1,
		},
		{
			name: "parse error in method",
			files: map[string]string{
				"Bad.peak": `public class Bad {
    public <T T> T badMethod() {}
}`,
			},
			expectErrors:    false, // Parser handles gracefully
			expectedMethods: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr.methodTemplates = make(map[string]*parser.GenericMethodDef)
			results := []FileResult{}

			hasErrors := tr.collectMethodTemplates(tt.files, &results)

			if tt.expectErrors != hasErrors {
				t.Errorf("expected errors=%v, got %v", tt.expectErrors, hasErrors)
			}

			if len(tr.methodTemplates) != tt.expectedMethods {
				t.Errorf("expected %d method templates, got %d", tt.expectedMethods, len(tr.methodTemplates))
			}
		})
	}
}

func TestTranspileFiles_WithForcedInstantiations(t *testing.T) {
	tr := NewTranspiler(nil)
	tr.SetInstantiations([]string{"Queue<Boolean>", "Queue<Decimal>"})

	files := map[string]string{
		"Queue.peak": `public class Queue<T> {
    private List<T> items;
}`,
		"Example.peak": `public class Example {
    private Integer x;
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Should generate QueueBoolean and QueueDecimal even though not used in code
	var foundBoolean, foundDecimal bool
	for _, result := range results {
		if strings.Contains(result.OutputPath, "QueueBoolean.cls") {
			foundBoolean = true
			if !strings.Contains(result.Content, "List<Boolean>") {
				t.Error("QueueBoolean should contain List<Boolean>")
			}
		}
		if strings.Contains(result.OutputPath, "QueueDecimal.cls") {
			foundDecimal = true
			if !strings.Contains(result.Content, "List<Decimal>") {
				t.Error("QueueDecimal should contain List<Decimal>")
			}
		}
	}

	if !foundBoolean {
		t.Error("QueueBoolean.cls should be generated from forced instantiation")
	}
	if !foundDecimal {
		t.Error("QueueDecimal.cls should be generated from forced instantiation")
	}
}

func TestTranspileFiles_WithGenericMethods(t *testing.T) {
	tr := NewTranspiler(nil)
	tr.SetInstantiateSpec(&config.InstantiateSpec{
		Methods: map[string][]string{
			"Repository.get": {"Account", "Contact"},
		},
	})

	files := map[string]string{
		"Repository.peak": `public class Repository {
    private Map<String, Object> cache;

    public <T> T get(String key) {
        return (T) cache.get(key);
    }
}`,
	}

	results, err := tr.TranspileFiles(files)
	if err != nil {
		t.Fatalf("TranspileFiles failed: %v", err)
	}

	// Find Repository.cls
	var repoResult *FileResult
	for i := range results {
		if results[i].OutputPath == "Repository.cls" {
			repoResult = &results[i]
			break
		}
	}

	if repoResult == nil {
		t.Fatal("Repository.cls not found")
	}

	// Check that concrete methods were inserted
	if !strings.Contains(repoResult.Content, "getAccount") {
		t.Error("Repository.cls should contain getAccount method")
	}
	if !strings.Contains(repoResult.Content, "getContact") {
		t.Error("Repository.cls should contain getContact method")
	}
	if !strings.Contains(repoResult.Content, "// Generated concrete methods") {
		t.Error("Repository.cls should contain generated methods comment")
	}
}

func TestExtractClassName(t *testing.T) {
	tr := NewTranspiler(nil)

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple class",
			content:  "public class MyClass { }",
			expected: "MyClass",
		},
		{
			name:     "class with generic",
			content:  "public class Queue<T> { }",
			expected: "Queue",
		},
		{
			name:     "private class",
			content:  "private class Helper { }",
			expected: "Helper",
		},
		{
			name:     "class without modifier",
			content:  "class Simple { }",
			expected: "Simple",
		},
		{
			name:     "multiline",
			content:  "  \n  public class Test { }",
			expected: "Test",
		},
		{
			name:     "multiple spaces",
			content:  "public    class     MyClass { }",
			expected: "MyClass",
		},
		{
			name:     "tabs and spaces",
			content:  "public\t\tclass\t MyClass<T> { }",
			expected: "MyClass",
		},
		{
			name:     "no class",
			content:  "interface ITest { }",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.extractClassName(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
