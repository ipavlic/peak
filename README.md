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

- **Write once, use everywhere** - Create a generic like `Queue<T>` and use it with any type
- **Type safety** - Generated classes are strongly typed; no casting, no runtime errors
- **Zero runtime cost** - All generics resolve at compile time to concrete classes
- **Future-proof** - Minimal syntax transformation ensures compatibility with Apex updates
- **Nested generics** - Support for complex types like `Queue<List<Integer>>`

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

### Usage

```bash
peak examples/                  # Transpile directory
peak --watch examples/          # Auto-recompile on changes
peak --out-dir build/ src/      # Custom output directory
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

### Step 3: Transpile & Output

Run Peak to generate concrete Apex classes:

```bash
peak examples/
```

Peak generates:

1. **Transpiled usage files** - `QueueExample.cls` with generic references replaced:
   ```apex
   public class QueueExample {
       private QueueInteger intQueue;    // Queue<Integer> → QueueInteger
       private QueueString stringQueue;  // Queue<String> → QueueString
       // ...
   }
   ```

2. **Concrete class files** - Type-specific classes from templates:
   - `QueueInteger.cls` - all `T` replaced with `Integer`
   - `QueueString.cls` - all `T` replaced with `String`

3. **Templates skipped** - `Queue.peak` is not compiled (it's a template)

All `.cls` files are ready to deploy to Salesforce!

## Configuration

**Output Location**

By default, `.cls` files are placed alongside source `.peak` files. Override with `--out-dir`:

```bash
peak --out-dir build/classes src/
```

**Config File** (optional)

Create `peakconfig.json` in your source directory:

```json
{
  "compilerOptions": {
    "outDir": "build/classes",
    "instantiate": {
      "classes": {
        "Queue": ["Integer", "String", "Boolean"],
        "Optional": ["Double", "Decimal"]
      },
      "methods": {
        "Repository.get": ["Account", "Contact", "String"],
        "Repository.put": ["Account", "Contact"]
      }
    }
  }
}
```

**Options:**
- `outDir` - Output directory (can be overridden by `--out-dir` flag)
- `instantiate.classes` - Force generation of specific class instantiations
- `instantiate.methods` - Force generation of specific method instantiations (format: `"ClassName.methodName": ["Type1", "Type2"]`)

## Features

### Type Parameter Rules

Type parameters must be single uppercase letters (`T`, `K`, `V`, etc.):

```apex
✓ class Queue<T>              // Good - single letter
✓ class Dict<K, V>            // Good - multiple single letters
✗ class Queue<Type>           // Error - multi-letter not allowed
✗ class Dict<T, T>            // Error - duplicate parameters
```

### Built-in Generics

Apex's native `List<T>`, `Set<T>`, and `Map<K,V>` remain unchanged. Only custom generic classes are transformed.

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

Define generic methods that work with any type:

```apex
public class Repository {
    public <T> T get(String key) { ... }
    public <T> void put(String key, T value) { ... }
}
```

Configure concrete method generation in `peakconfig.json`:

```json
{
  "compilerOptions": {
    "instantiate": {
      "methods": {
        "Repository.get": ["Account", "Contact", "String"],
        "Repository.put": ["Account", "Contact"]
      }
    }
  }
}
```

Generates concrete methods:
```apex
public Account getAccount(String key) { ... }
public Contact getContact(String key) { ... }
public void putAccount(String key, Account value) { ... }
```

Naming: `methodName` + type (e.g., `getString`, `putAccount`)

### Error Handling

Peak provides clear error messages with line/column info. Files with errors are reported but don't block other files from compiling.

```
Queue.peak:5:14: error: type parameter must be a single letter, got: Type
```

## Examples

See `examples/` directory:

**Templates:**
- `Queue.peak` - Generic queue `<T>`
- `Dict.peak` - Generic dictionary `<K, V>`
- `Repository.peak` - Generic methods

**Usage:**
- `QueueExample.peak` - Basic usage
- `NestedGenericsExample.peak` - Nested types (`Queue<List<Integer>>`)
- `ComplexExample.peak` - Advanced patterns

Run `peak examples/` to try it out!

## Development

```bash
go build -o peak ./cmd/peak    # Build
go test ./...                   # Test
```

**Structure:**
- `cmd/peak/` - CLI application
- `pkg/parser/` - Generic syntax parser
- `pkg/transpiler/` - Template instantiation
- `examples/` - Example files

## Limitations

**Not yet supported:**
- Type constraints: `class Queue<T extends SObject>`
- Variance annotations: `class Queue<out T>`

**Note:** Generated class names use simple concatenation (`Queue<List<Integer>>` → `QueueListInteger`), which can create long names for deeply nested generics.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - see LICENSE file for details.

---

**Questions or Issues?** Open an issue or check the `examples/` directory for working code.
