# Peak to Apex Transpiler - Development Documentation

## Project Context

This project implements a transpiler that converts "Peak" language (Apex with generic templates) to standard Salesforce Apex by generating concrete classes. The key insight is to provide generic programming capabilities without reimplementing the entire Apex language.

### Design Philosophy

**Minimal Intervention Approach**: The transpiler only acts when it detects generic syntax (`<` after an identifier). All other Apex code passes through unchanged. This ensures:
- No need to maintain a full Apex language specification
- Future-proof against Salesforce Apex updates
- Simple extension without reimplementing the language

## Architecture

### Core Components

1. **Parser** (`pkg/parser/parser.go`)
   - Recursive descent parser for generic expressions
   - Distinguishes between generic syntax and comparison operators
   - Validates type parameters (single-letter requirement)
   - Detects syntax errors (`<<`, `>>`, duplicates)

2. **Transpiler** (`pkg/transpiler/transpiler.go`)
   - Manages directory-based compilation (no single-file mode)
   - Tracks template definitions vs usages
   - Supports transitive template dependencies
   - Generates concrete classes from templates
   - Handles type parameter substitution with three-pass approach

3. **CLI** (`cmd/peak/main.go`, `cmd/peak/watch.go`)
   - Directory-based processing (compile or watch modes)
   - Watch mode with file system monitoring and debouncing
   - Recursive .peak file discovery

### Data Structures

```go
// Generic expression representation
type GenericExpr struct {
    BaseType string        // e.g., "Queue"
    TypeArgs []GenericExpr // e.g., [Integer]
    IsSimple bool          // true for simple types
}

// Template definition
type GenericClassDef struct {
    ClassName  string   // e.g., "Queue"
    TypeParams []string // e.g., ["T"]
    Body       string   // Class body with generics
    StartPos   int
    EndPos     int
}

// Transpilation result
type FileResult struct {
    OriginalPath string
    OutputPath   string
    Content      string
    IsTemplate   bool
    Error        error
}
```

## Key Learnings

### 1. Parser Design Decisions

**Recursive Parsing for Generics**
- When `<` is encountered after an identifier, recursively parse the type arguments
- This naturally handles nested generics like `Queue<List<Integer>>`
- Type arguments can themselves be generic expressions

**Word Boundary Detection**
```go
func isIdentifierChar(r rune) bool {
    return (r >= 'a' && r <= 'z') ||
           (r >= 'A' && r <= 'Z') ||
           (r >= '0' && r <= '9') ||
           r == '_'
}
```
This prevents false matches when replacing type parameters (e.g., don't replace "T" in "String").

**Comparison Operator Disambiguation**
```go
// Check it's not a comparison operator
if p.peek(1) != '=' && !unicode.IsSpace(rune(p.peek(1)))
```
This distinguishes `Queue<T>` from `x < 5`.

### 2. Type Parameter Substitution

**Template Instantiation Process** (Three-Pass Approach):

The `instantiateTemplate` function performs three distinct substitution passes to correctly generate concrete classes:

1. **Pass 1: Type Parameter Substitution**
   - Parse template to extract type parameters (e.g., `Queue<T>` → `["T"]`)
   - Parse usage to extract concrete types (e.g., `Queue<Integer>` → `["Integer"]`)
   - Build substitution map: `{"T": "Integer"}`
   - Replace all occurrences of type parameters in template body using word boundary detection

2. **Pass 2: Nested Generic Replacement**
   - After type parameter substitution, scan for remaining generic usages
   - Replace nested template usages with concrete class names
   - Example: `Queue<Boolean>` → `QueueBoolean` (critical for transitive dependencies)
   - This enables templates to use other templates internally

3. **Pass 3: Class Name and Constructor Replacement**
   - Remove type parameters from class declaration
   - Replace template class name with concrete class name
   - Ensures `Queue()` constructors become `QueueInteger()`

**Why Three Passes?**
The multi-pass approach handles complex scenarios like `Dict<K, V>` using `Queue<K>` internally. When instantiating `Dict<String, Integer>`, Pass 1 creates `Queue<String>`, then Pass 2 converts it to `QueueString`.

### 3. Compilation Process

**Four-Phase Compilation**:

1. **Phase 1**: Collect all generic class definitions (templates)
   - Parse each file for `class Name<T>` patterns
   - Store templates in a map by class name
   - Track file paths for each template

2. **Phase 2**: Collect all generic instantiations (with transitive support)
   - Find all uses of generics (e.g., `Queue<Integer>`)
   - **Critical**: For template files, scan only class bodies (not declarations)
   - This prevents `class Queue<T>` from being treated as a usage
   - Enables transitive dependencies: templates can use other templates
   - Only track usages of defined templates
   - Ignore built-in types (List, Set, Map)

3. **Phase 3**: Generate output for each file
   - Template files are skipped (no .cls generated)
   - Non-template files have generic references replaced with concrete names
   - Uses `replaceGenericUsages` helper to eliminate code duplication

4. **Phase 4**: Generate concrete class files
   - For each unique instantiation, substitute type parameters
   - Uses three-pass substitution (see above)
   - Generate .cls file with concrete types in same directory as template

### 4. Error Handling Strategy

**Validation Points**:
- Type parameter parsing (single-letter check)
- Syntax error detection (`<<`, `>>`)
- Duplicate parameter check
- Template/usage mismatch

**Error Propagation**:
```go
type FileResult struct {
    // ... other fields
    Error error  // Captured at parse time
}
```
Errors are captured per-file and reported during output generation, allowing partial compilation.

### 5. File Watching Implementation

**Debouncing Strategy**:
```go
debounceDuration := 500 * time.Millisecond
debounceTimer = time.AfterFunc(debounceDuration, func() {
    compileDirectory(dir)
})
```
Prevents multiple recompiles when rapid changes occur (e.g., editor auto-save).

**Hidden Directory Filtering**:
```go
if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != root {
    return filepath.SkipDir
}
```
Avoids watching `.git`, `.vscode`, etc.

### 6. Built-in Type Handling

**Preserving Apex Native Generics**:
```go
func isBuiltInGeneric(typeName string) bool {
    switch typeName {
    case "List", "Set", "Map":
        return true
    default:
        return false
    }
}
```
Apex's native generics remain unchanged; only custom templates trigger concrete class generation.

### 7. Code Reusability: Helper Methods

**replaceGenericUsages Helper**:
To eliminate code duplication between Phase 3 (file transpilation) and Pass 2 (template instantiation), a shared helper method was extracted:

```go
func (t *Transpiler) replaceGenericUsages(content string, generics map[string]*parser.GenericExpr) string {
    // Sort by length (longest first) to handle nested generics
    sortedKeys := make([]string, 0, len(generics))
    for key := range generics {
        sortedKeys = append(sortedKeys, key)
    }
    sort.Slice(sortedKeys, func(i, j int) bool {
        return len(sortedKeys[i]) > len(sortedKeys[j])
    })

    // Replace in order
    output := content
    for _, original := range sortedKeys {
        expr := generics[original]
        if _, isTemplate := t.templates[expr.BaseType]; isTemplate {
            concrete := parser.GenerateConcreteClassName(expr)
            output = strings.ReplaceAll(output, original, concrete)
        }
    }
    return output
}
```

This method is used both in `transpileFile` (replacing generics in non-template files) and in `instantiateTemplate` Pass 2 (replacing nested generics after type parameter substitution).

### 8. Name Generation

**Concatenation Strategy**:
- `Queue<Integer>` → `QueueInteger`
- `Dict<String, Integer>` → `DictStringInteger`
- `Queue<List<Integer>>` → `QueueListInteger`

This is simple and predictable, though it can create long names for deeply nested generics.

## Challenges & Solutions

### Challenge 1: Nested Generic Parsing
**Problem**: How to handle `Map<String, List<Integer>>`?

**Solution**: Recursive type argument parsing. When parsing type arguments, each argument can itself be a generic expression, triggering another recursive parse.

### Challenge 2: Template vs Usage Distinction (with Transitive Dependencies)
**Problem**: How to distinguish template definitions from usages, while allowing templates to use other templates?

**Initial Incorrect Approach**: Skip collecting usages from template files entirely
- This broke transitive dependencies
- Example: `Dict<K,V>` internally uses `Queue<K>`, so when instantiating `Dict<String,Integer>`, we need `QueueString.cls` generated

**Correct Solution**:
- **Phase 1**: Scan for `class Name<T>` patterns → templates
- **Phase 2**: For template files, scan only class bodies (not declarations)
  ```go
  if len(defs) > 0 {
      var bodies []string
      for _, def := range defs {
          bodies = append(bodies, def.Body)
      }
      contentToScan = strings.Join(bodies, "\n")
  }
  ```
- This prevents `class Queue<T>` from being treated as a usage
- But allows `Queue<K>` inside `Dict<K,V>` body to be detected
- Template files don't generate .cls output for themselves
- Usages (including in template bodies) trigger concrete class generation

### Challenge 3: Multiple Type Parameters
**Problem**: Supporting `Dict<K, V>` with proper substitution

**Solution**:
- Parse comma-separated type parameters
- Build substitution map with index-based mapping
- Validate parameter count matches during instantiation

### Challenge 4: Constructor Renaming
**Problem**: `Queue()` constructor in template needs to become `QueueInteger()` in concrete class

**Solution**: After type parameter substitution, perform an additional replacement of the template class name with the concrete name using word boundary detection.

### Challenge 5: Error Reporting Without Breaking Compilation
**Problem**: One file's error shouldn't prevent other files from compiling

**Solution**:
- Capture errors per file in `FileResult.Error`
- Continue compilation for files without errors
- Report all errors at the end
- Return error code if any files failed

## Performance Considerations

### Parser Efficiency
- Single-pass scanning for both templates and usages
- No AST construction (minimal memory overhead)
- Early exit on non-generic code

### File System Watching
- Uses `fsnotify` for efficient OS-level file watching
- Debouncing prevents redundant compilations
- Only monitors `.peak` files

### Compilation Speed
Typical compile times (examples directory with 5 files):
- Initial: ~3-5ms
- Incremental: ~2-4ms

## Testing Strategy

### Unit Tests
- Parser: Generic expression parsing, type parameters, nested generics
- Transpiler: Template substitution, name generation
- Error handling: Syntax errors, validation

### Integration Tests
Example files demonstrate:
- Single type parameter (`Queue<T>`)
- Multiple type parameters (`Dict<K, V>`)
- Nested generics (`Queue<List<Integer>>`)
- Built-in type preservation (`List<String>` unchanged)

## Future Enhancements

### Potential Improvements

1. **Template Constraints**
   ```
   class Queue<T extends SObject>
   ```
   Would require constraint parsing and validation.

2. **Generic Methods**
   ```
   public static <T> T max(T a, T b)
   ```
   Currently only class-level generics are supported.

3. **Variance Annotations**
   ```
   class Queue<in T>  // contravariant
   class Queue<out T> // covariant
   ```
   Would enable more sophisticated type checking.

4. **Template Specialization**
   ```
   class Queue<T> { /* generic implementation */ }
   class Queue<Integer> { /* optimized for Integer */ }
   ```
   Allow hand-written optimizations for specific types.

5. **Better Error Messages**
   - Line/column information in errors
   - Suggestions for common mistakes
   - Context snippets in error output

6. **Source Maps**
   - Map generated code back to templates
   - Enable debugging of template code

## Code Organization

```
peak/
├── cmd/
│   └── peak/              # CLI entry point
│       ├── main.go        # Main program, mode detection
│       ├── main_multifile.go  # Directory processing logic
│       └── watch.go       # File watching mode
├── pkg/
│   ├── parser/            # Generic parsing logic
│   │   ├── parser.go      # Parser implementation
│   │   └── parser_test.go # Parser tests
│   └── transpiler/        # Transpilation logic
│       └── transpiler.go  # Transpiler implementation
├── examples/              # Example .peak files
│   ├── Queue.peak         # Single type param template
│   ├── Dict.peak          # Multiple type param template
│   ├── QueueExample.peak  # Simple template usage
│   ├── NestedGenericsExample.peak     # Nested generics (Queue<List<T>>)
│   ├── MultiParametersExample.peak   # Multiple type parameters (Dict<K,V>)
│   └── ComplexExample.peak            # Complex patterns (Dict<K, Queue<V>>)
├── go.mod
├── README.md              # User documentation
└── CLAUDE.md             # This file - development docs
```

## Design Patterns Used

### 1. Parser Combinator Pattern
Small parsing functions compose to handle complex expressions:
- `parseIdentifier()`
- `parseTypeArgument()`
- `parseGeneric(baseType)`

### 2. Visitor Pattern (Implicit)
Template substitution visits each type parameter occurrence and replaces it.

### 3. Builder Pattern
`FileResult` construction accumulates information through compilation phases.

### 4. Observer Pattern
File watcher observes file system changes and triggers compilation.

## Key Insights

1. **Don't Parse Everything**: Only parse what you need to transform. Everything else is pass-through.

2. **Recursive Data Structures**: Generics are naturally recursive (`List<List<T>>`), so use recursive parsing.

3. **Word Boundaries Matter**: When doing text replacement, always check word boundaries to avoid partial matches.

4. **Fail Gracefully**: When compiling a directory, one file's error shouldn't block others.

5. **Debouncing is Essential**: File watchers need debouncing or they'll trigger multiple times per save.

6. **Type Safety Through Names**: Since Apex doesn't have runtime generics, compile-time name generation ensures type safety.

## Transpiler Workflow Example

### Input Files

**Queue.peak** (template):
```apex
public class Queue<T> {
    private List<T> items;
    public Queue() { items = new List<T>(); }
    public void enqueue(T item) { items.add(item); }
}
```

**QueueExample.peak** (usage):
```apex
public class QueueExample {
    private Queue<Integer> q;
    public QueueExample() { q = new Queue<Integer>(); }
}
```

### Processing Steps

1. **Template Collection**: Find `Queue<T>`, store body and parameters
2. **Usage Detection**: Find `Queue<Integer>` in QueueExample.peak
3. **File Generation**:
   - Skip Queue.peak (it's a template)
   - Generate QueueExample.cls with `Queue<Integer>` → `QueueInteger`
   - Generate QueueInteger.cls from Queue template with T → Integer

### Output Files

**QueueExample.cls**:
```apex
public class QueueExample {
    private QueueInteger q;
    public QueueExample() { q = new QueueInteger(); }
}
```

**QueueInteger.cls**:
```apex
public class QueueInteger {
    private List<Integer> items;
    public QueueInteger() { items = new List<Integer>(); }
    public void enqueue(Integer item) { items.add(item); }
}
```

## Conclusion

This transpiler demonstrates that you don't need to fully parse a language to extend it. By focusing on a specific pattern (generic syntax) and using minimal intervention, we achieved:

- Generic programming in Apex
- Type-safe code generation
- Zero runtime overhead (everything is compile-time)
- Future-proof design (works with any Apex version)
- Fast compilation (no heavy AST processing)

The key is identifying the smallest set of syntax that needs transformation and leaving everything else alone.
