# Peak - Generics for Salesforce Apex

[![Test and Coverage](https://github.com/ipavlic/peak/actions/workflows/test.yml/badge.svg)](https://github.com/ipavlic/peak/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/ipavlic/peak/graph/badge.svg?token=8PMNGNTH9E)](https://codecov.io/gh/ipavlic/peak)
[![Go Report Card](https://goreportcard.com/badge/github.com/ipavlic/peak)](https://goreportcard.com/report/github.com/ipavlic/peak)

**Peak** is a transpiler that brings generic programming back to Salesforce Apex. Write reusable generic classes once, and Peak generates type-safe concrete Apex classes ready for deployment.

```apex
// Write once
public class Queue<T> {
    private List<T> items;
    public void enqueue(T item) { items.add(item); }
    public T dequeue() { return items.remove(0); }
}

// Use everywhere
Queue<Integer> numbers = new Queue<Integer>();
Queue<Account> accounts = new Queue<Account>();
```

## Why Peak?

Peak brings compile-time generics to Apex without runtime overhead:

- **Write once, use everywhere**: Create a generic like `Queue<T>` and use it with any type
- **Type safety**: Generated classes are strongly typed — no casting, no runtime errors
- **Zero runtime cost**: All generics resolve at compile time to concrete classes
- **Future-proof**: Minimal syntax transformation means compatibility with upcoming Apex versions
- **Nested generics**: Support for complex types like `Queue<List<Integer>>`

## Quick Start

### Installation

```bash
# Install from source (requires Go 1.20+)
git clone https://github.com/ipavlic/peak.git
cd peak
go build -o peak ./cmd/peak

# Or install directly
go install github.com/ipavlic/peak/cmd/peak@latest
```

### Basic Usage

```bash
# Show help
peak --help

# Transpile all .peak files in a directory
peak examples/

# Watch mode - automatically recompile on changes
peak --watch examples/

# Specify output directory
peak --out-dir build/ src/
```

## How It Works

### Step 1: Define a Generic Template

Create a `.peak` file with generic type parameters:

```apex
// Queue.peak - A generic queue that works with any type
public class Queue<T> {
    private List<T> items;

    public Queue() {
        this.items = new List<T>();
    }

    public void enqueue(T item) {
        items.add(item);
    }

    public T dequeue() {
        return items.remove(0);
    }
}
```

### Step 2: Use the Template

Reference your generic class with concrete types:

```apex
// QueueExample.peak - Uses Queue with specific types
public class QueueExample {
    private Queue<Integer> intQueue;
    private Queue<String> stringQueue;

    public QueueExample() {
        this.intQueue = new Queue<Integer>();
        this.stringQueue = new Queue<String>();
    }
}
```

### Step 3: Transpile

Run Peak to generate concrete Apex classes:

```bash
peak examples/
```

### Step 4: What You Get

Peak generates three types of output:

**1. Skips Templates**
`Queue.peak` is recognized as a template (it defines `Queue<T>`) and no `Queue.cls` is generated.

**2. Transpiled Usage Files**
`QueueExample.cls` with generic references replaced by concrete class names:
```apex
public class QueueExample {
    private QueueInteger intQueue;    // Queue<Integer> → QueueInteger
    private QueueString stringQueue;  // Queue<String> → QueueString

    public QueueExample() {
        this.intQueue = new QueueInteger();
        this.stringQueue = new QueueString();
    }
}
```

**3. Concrete Class Files**
Type-specific classes generated from templates:
- `QueueInteger.cls` - all `T` replaced with `Integer`
- `QueueString.cls` - all `T` replaced with `String`

These `.cls` files are ready to deploy to Salesforce!

## CLI Reference

### Commands

```bash
# Show help
peak --help
peak -h

# Transpile directory
peak [directory]              # Transpile all .peak files (default: current directory)
peak examples/                # Transpile examples/ directory

# Watch mode
peak --watch [directory]      # Auto-recompile on file changes
peak -w                       # Short form

# Custom output directory
peak --out-dir <dir>          # Override output location
peak -o build/                # Short form
```

### Configuration

**Output Location**

By default, generated `.cls` files are placed alongside their source `.peak` files:

```
examples/
├── Queue.peak              # Template (not compiled)
├── QueueExample.peak       # Usage file
├── QueueExample.cls        # ✓ Generated
├── QueueInteger.cls        # ✓ Generated
└── QueueString.cls         # ✓ Generated
```

**Config File** (optional)

Create `peakconfig.json` in your source directory to customize behavior:

```json
{
  "compilerOptions": {
    "outDir": "build/classes",
    "instantiate": {
      "classes": {
        "Queue": ["Integer", "String"],
        "Optional": ["Double", "Decimal"]
      },
      "methods": {
        "Repository.get": ["Account", "Contact", "String"],
        "Repository.put": ["Account", "Contact"],
        "Repository.getOrDefault": ["String", "Integer"]
      }
    }
  }
}
```

**Configuration Options:**

- `outDir` - Output directory for generated files (can be overridden by `--out-dir` flag)
- `instantiate` - List of generic instantiations to always generate, even if not used in code
  - `classes` - Map of template names to type arguments (e.g., `"Queue": ["Integer", "String"]`)
  - `methods` - Map of method keys to type arguments (e.g., `"ClassName.methodName": ["String", "Decimal"]`)
    - For methods with multiple type parameters, use comma-separated types (e.g., `"String,String"` for `<K,V>` with both as String)

## Features

### Type Parameter Rules

Type parameters must be single uppercase letters (`T`, `K`, `V`, etc.):

```apex
✓ class Queue<T>              // Good - single letter
✓ class Dict<K, V>            // Good - multiple single letters
✗ class Queue<Type>           // Error - multi-letter not allowed
✗ class Dict<T, T>            // Error - duplicate parameters
```

### Built-in Generics Preserved

Apex's native `List<T>`, `Set<T>`, and `Map<K,V>` remain unchanged. Peak only transforms your custom generic classes.

### Multiple Type Parameters

Define classes with multiple type parameters:

```apex
public class Dict<K, V> {
    private List<K> keys;
    private List<V> values;

    public void put(K key, V value) {
        keys.add(key);
        values.add(value);
    }

    public V get(K key) {
        Integer index = keys.indexOf(key);
        return index >= 0 ? values.get(index) : null;
    }
}

// Use with any key-value combination
Dict<String, Integer> scores = new Dict<String, Integer>();
Dict<Integer, Account> accountMap = new Dict<Integer, Account>();
```

### Nested Generics

Generic types can be nested to any depth:

```apex
Queue<List<Integer>> batchQueue = new Queue<List<Integer>>();
Dict<String, Queue<Account>> accountQueues = new Dict<String, Queue<Account>>();
```

Generates concrete classes like `QueueListInteger.cls` and `DictStringQueueAccount.cls`.

### Generic Methods

Peak supports generic methods with type parameters, allowing you to create reusable methods that work with any type:

```apex
// Repository.peak - A generic cache/repository
public class Repository {
    private Map<String, Object> cache;

    public Repository() {
        this.cache = new Map<String, Object>();
    }

    // Generic method to get a cached value
    public <T> T get(String key) {
        return (T) cache.get(key);
    }

    // Generic method to store a value
    public <T> void put(String key, T value) {
        cache.put(key, value);
    }

    // Generic method to get with default value
    public <T> T getOrDefault(String key, T defaultValue) {
        if (cache.containsKey(key)) {
            return (T) cache.get(key);
        }
        return defaultValue;
    }
}
```

**Generated Concrete Methods**

Configure which concrete methods to generate using `instantiateSpec` in `peakconfig.json`:

```json
{
  "compilerOptions": {
    "instantiateSpec": {
      "methods": {
        "Repository.get": ["Account", "Contact", "String"],
        "Repository.put": ["Account", "Contact"],
        "Repository.getOrDefault": ["String", "Integer"]
      }
    }
  }
}
```

This generates concrete methods inserted into the same class:

```apex
// Generated concrete methods
public Account getAccount(String key) { ... }
public Contact getContact(String key) { ... }
public String getString(String key) { ... }

public void putAccount(String key, Account value) { ... }
public void putContact(String key, Contact value) { ... }

public String getOrDefaultString(String key, String defaultValue) { ... }
public Integer getOrDefaultInteger(String key, Integer defaultValue) { ... }
```

**Method Naming Convention:**
- Single type parameter: `methodName` + type (e.g., `getString`)
- Multiple type parameters: `methodName` + all types concatenated (e.g., `transformStringString`)

### Valid Generic Expressions

```apex
Queue<Integer>               // Simple type
Dict<String, Account>        // Multiple parameters
Queue<List<Integer>>         // Nested generics
Dict<Integer, Queue<String>> // Complex nesting
```

### Error Handling

Peak provides clear error messages with line and column information:

```
Queue.peak:5:14: error: type parameter must be a single letter, got: Type
public class Queue<Type> {
                   ^
```

Files with errors are reported but don't block compilation of other files.

## Examples

The `examples/` directory contains complete working demonstrations:

### Templates
- **`Queue.peak`** - Generic queue with single type parameter `<T>`
- **`Dict.peak`** - Generic dictionary with key-value parameters `<K, V>`

### Usage Examples
- **`QueueExample.peak`** - Basic usage of `Queue<Integer>` and `Queue<String>`
- **`NestedGenericsExample.peak`** - Nested types like `Queue<List<Integer>>`
- **`MultiParametersExample.peak`** - Multiple instantiations of `Dict<K, V>`
- **`ComplexExample.peak`** - Advanced patterns like `Dict<String, Queue<Integer>>`
- **`Repository.peak`** - Generic methods with `get<T>`, `put<T>`, and `getOrDefault<T>`

Run `peak examples/` to see the transpiler in action!

## Development

### Build

```bash
go build -o peak ./cmd/peak
```

### Test

```bash
go test ./...
```

### Project Structure

```
peak/
├── cmd/peak/          # CLI application
├── pkg/
│   ├── parser/        # Generic syntax parser
│   └── transpiler/    # Template instantiation logic
└── examples/          # Example .peak files
```

## Current Limitations

Peak focuses on class-level generics. These features are not yet supported:

| Feature | Status | Example |
|---------|--------|---------|
| Type constraints | Not supported | `class Queue<T extends SObject>` |
| Variance annotations | Not supported | `class Queue<out T>` |

**Name Generation**: Generated class names use simple concatenation (`Queue<List<Integer>>` → `QueueListInteger`). This can create long names for deeply nested generics but ensures predictability and avoids naming conflicts.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - see LICENSE file for details.

---

**Questions or Issues?** Open an issue or check the `examples/` directory for working code.
