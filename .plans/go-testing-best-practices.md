Writing clean, reliable, and maintainable tests in Go relies on idiomatic patterns that align with the language's design philosophy.

### 1. Structure Tests using Table-Driven Design

Table-driven testing is the standard pattern in Go. It allows you to run multiple test cases against the same logic cleanly without code duplication.

```go
func TestCalculateDiscount(t *testing.T) {
    tests := []struct {
        name     string
        price    float64
        discount float64
        want     float64
        wantErr  bool
    }{
        {name: "valid 10% discount", price: 100, discount: 0.10, want: 90, wantErr: false},
        {name: "negative price error", price: -10, discount: 0.10, want: 0, wantErr: true},
        {name: "invalid discount > 1", price: 100, discount: 1.5, want: 0, wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := CalculateDiscount(tt.price, tt.discount)
            if (err != nil) != tt.wantErr {
                t.Fatalf("CalculateDiscount() error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("CalculateDiscount() = %v, want %v", got, tt.want)
            }
        })
    }
}

```

### 2. Isolate Packages with External Test Packages (`_test`)

To enforce black-box testing and prevent testing private implementation details, place test files in a package named `<packagename>_test`.

```go
// File: user/user_test.go
package user_test // Enforces testing only exported functions/types

import (
    "testing"
    "myproject/user"
)

func TestNewUser(t *testing.T) { ... }

```

*Use internal `package user` tests only when you specifically need to test unexported/private helper functions.*

### 3. Use `t.Helper()` for Test Utilities

When writing helper functions to reduce setup boilerplate, call `t.Helper()`. This instructs the Go test runner to report failures at the caller's line number rather than inside the helper function.

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper() // Corrects file/line logging on failure
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("failed to open db: %v", err)
    }
    return db
}

```

### 4. Clean Up Resources with `t.Cleanup`

Instead of relying on `defer` inside setup helpers (which executes as soon as the helper finishes, not the test), register teardown callbacks with `t.Cleanup`.

```go
func createTempFile(t *testing.T) string {
    t.Helper()
    f, err := os.CreateTemp("", "test")
    if err != nil {
        t.Fatalf("failed to create temp file: %v", err)
    }

    t.Cleanup(func() {
        os.Remove(f.Name())
    })

    return f.Name()
}

```

### 5. Favor Interfaces and Mocks over Heavy Frameworks

Go emphasizes composition through small, focused interfaces. To test code with external dependencies (like databases or API clients), pass interfaces and mock them manually or using a lightweight code generator (like `mockery` or `gomock`).

```go
type DataFetcher interface {
    FetchData(id string) (string, error)
}

type MockFetcher struct {
    Response string
    Err      error
}

func (m *MockFetcher) FetchData(id string) (string, error) {
    return m.Response, m.Err
}

```

### 6. Leverage Built-In Tooling

Go comes out of the box with tools for race detection, coverage, and performance benchmarking:

* **Detect Race Conditions:** Run tests with the race detector enabled during CI.
```bash
go test -race ./...

```


* **Check Test Coverage:** Generate and view coverage reports visually.
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

```


* **Benchmark Performance:** Write benchmarks using `testing.B`.
```go
func BenchmarkCalculateDiscount(b *testing.B) {
    for i := 0; i < b.N; i++ {
        CalculateDiscount(100, 0.10)
    }
}

```



### 7. Differentiate `t.Error` vs. `t.Fatal`

* Use **`t.Errorf`** when a check fails but subsequent assertions in the test can still execute safely.
* Use **`t.Fatalf`** when the failure prevents remaining code from working (e.g., failed setup or a `nil` pointer).

### 8. Centralize `testdata` with Logical Subdirectories

Go automatically ignores folders named `testdata` when compiling packages. Because all test files in the same directory belong to the same package and share the identical working directory when `go test` runs, maintain **one `testdata/` directory per package** rather than scattering separate folders:

```text
my-app/
└── test/                      # Root integration/E2E test package
    ├── api_test.go
    ├── health_test.go
    ├── signup_test.go
    └── testdata/              # Single central testdata directory
        ├── auth/              # Grouped by domain/feature
        │   ├── login_req.json
        │   └── login_success.golden
        ├── signup/
        │   ├── invalid_email.json
        │   └── test_signup_success.golden
        ├── shared/            # Shared reusable mocks (e.g., JWTs, common payloads)
        └── global_seed.sql    # Shared global database seeds
```

#### Best Practices for Centralized `testdata`

* **Namespace Golden Files by Test Function:** When asserting outputs against saved reference files, name the file after the test function to prevent collisions:
  ```go
  // In test/signup_test.go: Go sets the working directory to "test/", so relative paths work directly
  goldenData, err := os.ReadFile("testdata/signup/test_signup_success.golden")
  if err != nil {
      t.Fatalf("failed reading golden fixture: %v", err)
  }
  ```
* **Use a Shared Folder for Reusable Fixtures:** Put cross-cutting payloads (e.g., standard mock JWT tokens, common headers, generic addresses) in `testdata/shared/` or `testdata/common/` to avoid duplication.
* **Map Table-Driven Tests to Fixtures:** Instead of creating multiple `_test.go` files or separate folders for variations, drive tests through multi-case tables pointing to fixture files:
  ```go
  func TestAPI_Users(t *testing.T) {
      tests := []struct {
          name        string
          fixtureFile string
          wantStatus  int
      }{
          {name: "Valid User", fixtureFile: "testdata/users/valid.json", wantStatus: http.StatusOK},
          {name: "Missing Email", fixtureFile: "testdata/users/missing_email.json", wantStatus: http.StatusBadRequest},
      }

      for _, tt := range tests {
          t.Run(tt.name, func(t *testing.T) {
              payload, err := os.ReadFile(tt.fixtureFile)
              if err != nil {
                  t.Fatalf("failed reading fixture %s: %v", tt.fixtureFile, err)
              }
              // Execute request with payload and assert status...
          })
      }
  }
  ```
* **Separate E2E from Unit Test Runs via Build Tags:** Use build tags (e.g., `//go:build e2e`) on root `test/` suites so CI and local development can run fast unit tests without spinning up heavy dependencies:
  ```bash
  # Run unit & domain integration tests:
  go test ./...

  # Run E2E suites:
  go test -tags=e2e ./test/...
  ```
