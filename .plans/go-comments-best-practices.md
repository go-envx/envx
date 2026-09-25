# Go Commenting & Documentation Standards

Guidelines for writing idiomatic Go doc comments, code documentation, and function contracts compatible with Go 1.19+ and standard Go doc tooling.

## Core Rules

### 1. Function Scope & Caller Decoupling
- **Focus on individual contract**: Document what a function does, its parameters, return values, side effects, and error conditions.
- **Never document specific callers**: Do not describe which external functions call this logic or how caller workflows operate. Referencing callers introduces tight coupling and breaks encapsulation.
- **Document calling rules, not callers**: Specify strict interaction rules callers must obey (concurrency safety, required locks, initialization order).

### 2. Symbol & Package Doc Comments
- **Start with the symbol name**: Every exported identifier's doc comment must begin with its exact name (e.g., `// CalculateTotal ...`).
- **Use complete sentences**: Write grammatically complete sentences ending with a period.
- **Package doc comments**: Place directly above `package pkgname` with no blank lines, or inside a dedicated `doc.go` file for complex packages.
- **Exported struct fields**: Document exported fields directly above their declaration.

### 3. Formatting (Go 1.19+ Standard Doc Format)
- **Section Headings**: Use `#` at the start of a line inside doc comments to add headers.
- **Symbol Linking**: Reference Go symbols using square brackets (e.g., `[bytes.Buffer]` or `[User]`).
- **Code Examples**: Indent code examples with 4 spaces or a tab within the comment block.

### 4. Special Conventions & Directives
- **Deprecation notices**: Must begin with `// Deprecated:` on its own line followed by the alternative (e.g., `// Deprecated: Use [NewFunc] instead.`).
- **Compiler directives**: Ensure **no space** after `//` for directives like `//go:build` or `//go:generate`.
- **TODO comments**: Follow the standard format including owner or issue link: `// TODO(username): description (#issue)`.

### 5. Tone & Quality
- **Explain why, not what**: Focus on intent, invariants, business logic, and memory/performance considerations rather than repeating the implementation line-by-line.
- **Document all exported items**: Every public function, struct, interface, constant, and variable must have a doc comment.

---

## Code Examples

### Standard Function Doc Comment
```go
// FetchUser retrieves user profile data for the given identifier.
//
// Returns [ErrNotFound] if the user ID does not exist in the store.
// It is safe for concurrent use by multiple goroutines.
func (s *Service) FetchUser(id string) (*User, error) { ... }

```

### Struct and Field Documentation

```go
// User represents an authenticated application account.
type User struct {
    // ID is the unique database primary key.
    ID string

    // Email is the primary verified contact address.
    Email string
}

```

### Compiler Directives & Deprecations

```go
//go:build linux || darwin

// ParseLegacy converts v1 payload byte streams into a Data struct.
//
// Deprecated: Use [Parse] instead.
func ParseLegacy(b []byte) (*Data, error) { ... }

```

### Calling Rules & Lock Requirements

```go
// flushLocked flushes internal buffers directly to disk.
// The caller MUST hold the struct lock (c.mu) before invoking flushLocked.
func (c *Client) flushLocked() error { ... }

```

```

```
