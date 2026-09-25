# Go Domain-Driven Architecture Guide (Vertical Slice / Bounded Context)

This architecture organizes code by **Domain Boundary** (Feature Slices) rather than purely technical layers. It keeps business logic framework-agnostic while working *with* Go’s strict package visibility and compile-time cycle-prevention rules.

---

## 1. Directory & Package Layout

```text
my-app/
├── cmd/                          # UI Assembly & Binary Entry Points
│   ├── cli/
│   │   └── main.go              # Cobra CLI binary entry point
│   └── server/
│       └── main.go              # Web API binary entry point (optional)
│
├── internal/                     # Private Application Packages
│   ├── resources/               # 📦 Shared Infrastructure Clients & SDK Drivers
│   │   ├── db/                  # Postgres/MySQL connection pools & migrations
│   │   ├── llm/                 # OpenAI/Anthropic SDK client initializers
│   │   └── redis/               # Cache client initializers
│   │
│   ├── features/               # 📦 Application Domains/Features
|   │   ├── user/                    # 👤 "User" Bounded Context / Domain Slice
|   │   │   ├── domain/              # 🔒 Pure Domain Entities, Rules & Repo Interfaces
|   │   │   │   ├── user.go
|   │   │   │   └── user_test.go     # Unit tests (pure business logic, zero external deps)
|   │   │   ├── command/             # 📝 CQRS Write Use Cases (Create, Update, Delete)
|   │   │   │   ├── create_user.go
|   │   │   │   └── create_user_test.go # Use case unit tests (mocked repos)
|   │   │   ├── query/               # 🔍 CQRS Read Use Cases (Get, List, Search)
|   │   │   │   ├── get_user.go
|   │   │   │   └── get_user_test.go
|   │   │   ├── cli/                 # 💻 Cobra CLI Adapter (package usercli)
|   │   │   │   ├── user.go
|   │   │   │   └── user_test.go     # CLI adapter & flag parsing tests
|   │   │   ├── api/                 # 🌐 HTTP/REST Adapter (package userapi)
|   │   │   │   ├── handler.go
|   │   │   │   └── handler_test.go  # HTTP handler tests (httptest)
|   │   │   ├── infra/               # 🔌 Database Repositories (uses resources/db)
|   │   │   │   ├── postgres.go
|   │   │   │   └── postgres_test.go # Slice integration tests (real DB / testcontainers)
|   │   │   └── testdata/            # 📁 Domain-specific test fixtures (JSON, SQL dumps)
|   │   │       ├── unit_mock.json
|   │   │       └── db_dump.sql
|   │   │
|   │   └── billing/                 # 💳 "Billing" Bounded Context
|   │       ├── domain/              # 🔒 Pure Domain Entities
|   │       ├── command/             # 📝 CQRS Write Use Cases
|   │       ├── query/               # 🔍 CQRS Read Use Cases
|   │       ├── cli/                 # 💻 Cobra CLI Adapter
|   │       └── infra/               # 🔌 DB / Payment Gateway Implementations
|   |
│   ├── utils/               # 📦 Generic utility packages that could easily be copied and pasted into a new project and moved into a publically published go package. These should prioritize being dependency free to dependency-lite.
│   │   ├── file/                  # File operations
│   │   ├── str/                 # String utility methods
│   │   └── yamlx/               # YAML utility methods
|   |
│   └── shared/               # 📦 Global Shared Packages (these should be small and dep free)
|       ├── errors/                    # Global Application Error Handler
|       ├── constants/                    # Global Application Constants
|       └── schemas/                    # Global Application Schemas

├── test/                         # 🧪 End-to-End & Cross-Domain Integration Tests
│   ├── api_test.go               # Full HTTP API test suite against running server
│   ├── health_test.go            # Health and readiness probe tests
│   ├── signup_test.go            # Cross-boundary workflow tests (e.g., User + Billing)
│   └── testdata/                 # 📁 Central testdata directory (one per package)
│       ├── auth/                 # Grouped by domain/feature
│       │   ├── login_req.json
│       │   └── login_success.golden
│       ├── signup/
│       │   ├── invalid_email.json
│       │   └── test_signup_success.golden
│       ├── shared/               # Reusable fixtures across suites (JWTs, common payloads)
│       └── global_seed.sql       # Shared global database seeds
│
├── go.mod
└── go.sum

```

---

## 2. Layer Responsibilities & Compiler Guarantees

| Package | Purpose | Allowed Imports | Forbidden Imports |
| --- | --- | --- | --- |
| **`resources`** 📦 | Raw infrastructure client constructors (`*pgxpool.Pool`, OpenAI SDK client, Redis connection). | DB/SDK drivers, Standard Library | `domain`, `command`, `query`, `cli`, `api` |
| **`domain`** 🔒 | Business entities, value objects, domain errors, repository interfaces. | *Standard Library Only* | `command`, `query`, `infra`, `resources`, `cli`, `api`, or external frameworks |
| **`command`** 📝 | Write orchestration (mutates state, enforces application rules). | `domain` | `cli`, `api`, `infra`, `resources`, Cobra, HTTP drivers |
| **`query`** 🔍 | Read orchestration (fetches presentation data without side-effects). | `domain` | `cli`, `api`, `infra`, `resources`, Cobra, HTTP drivers |
| **`infra`** 🔌 | Domain-specific persistence (Postgres queries) and external clients. Implements `domain` interfaces. | `domain`, `resources`, DB drivers | `cli`, `api` |
| **`cli`** 💻 | Cobra command trees, terminal flags, argument parsing, stdout/stderr formatting. | `command`, `query`, `[github.com/spf13/cobra](https://github.com/spf13/cobra)` | `infra` directly (use `command`/`query`) |
| **`api`** 🌐 | HTTP routes, request/response DTOs, JSON encoders. | `command`, `query`, Router (Chi, Fiber, etc.) | `cli`, `infra` directly |
| **`cmd/`** | Global root assembly, flag inheritance, Dependency Injection (DI) wiring. | `cli`, `api`, `infra`, `resources` | N/A (Root level) |
| **`test/`** 🧪 | Full application E2E tests, cross-slice workflows, and system acceptance suites. | `cmd/`, `internal/` packages, DB/HTTP drivers, Standard Library | Direct access to unexported package internals (uses `_test` packages) |

---

## 3. Dependency Flow Rules

```text
[ cmd/cli/main.go ] ────(initializes client)────► [ internal/resources/db ]
       │                                                      │
       ▼                                                      ▼
[ internal/feature/user/cli ] ──► [ internal/feature/user/command ]    [ internal/feature/user/infra ]
                                  │                           │
                                  └──────────► [ domain ] ◄───┘ (impl interface)

```

### Compiler Invariants

1. **Raw Drivers Isolation:** `internal/resources/` initializes low-level clients (e.g., `*pgx.ConnPool` or OpenAI client). These clients are passed into `internal/feature/user/infra`, keeping raw connection details separated from business repositories.
2. **Hard Isolation:** Because Go packages are folder-scoped, `domain`, `command`, and `query` **cannot access Cobra or DB drivers** unless explicitly imported. The compiler prevents leakage.
3. **Tree Shaking across Binaries:** Building `cmd/cli/main.go` only compiles imported resources and packages. Unused drivers or adapters are omitted from the final binary.
4. **Testing Isolation & Single `testdata` Rule:** Go ignores directories named `testdata` during build time, reserving them exclusively for test fixtures. Because each Go package shares a single working directory during `go test`, maintain **one `testdata` folder per package**: domain-level unit tests use their slice's colocated `internal/<domain>/testdata/`, while all end-to-end suites share a single, centrally organized `test/testdata/` with subfolders for domains (`auth/`, `signup/`) and common mocks (`shared/`).

---

## 4. Code Implementation Patterns

### A. Resource Client Initializer (`internal/resources/db/postgres.go`)

Provides initialized connections to databases or external services.

```go
package db

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
    pool, err := pgxpool.New(ctx, connString)
    if err != nil {
        return nil, fmt.Errorf("unable to connect to database: %w", err)
    }

    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("database ping failed: %w", err)
    }

    return pool, nil
}

```

---

### B. Domain Repository Implementation (`internal/feature/user/infra/postgres.go`)

Uses the shared resource client to satisfy the pure domain repository interface.

```go
package infra

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "my-app/internal/feature/user/domain"
)

type PostgresUserRepository struct {
    pool *pgxpool.Pool // Uses the raw client from internal/resources/db
}

func NewPostgresUserRepository(pool *pgxpool.Pool) domain.UserRepository {
    return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) Save(ctx context.Context, user *domain.User) error {
    query := `INSERT INTO users (id, name, email) VALUES ($1, $2, $3)`
    _, err := r.pool.Exec(ctx, query, user.ID(), user.Name(), user.Email())
    return err
}

```

---

### C. The CLI Adapter (`internal/feature/user/cli/user.go`)

Converts Cobra terminal inputs into application DTOs. Cobra parameters (`*cobra.Command`) **never** pass beyond this layer.

```go
package usercli

import (
    "fmt"
    "github.com/spf13/cobra"
    usercmd "my-app/internal/feature/user/command"
)

func NewUserCmd(createHandler usercmd.CreateUserHandler) *cobra.Command {
    var email string

    cmd := &cobra.Command{
        Use:   "create [name]",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // 1. Map CLI input to App DTO
            input := usercmd.CreateUserCommand{
                Name:  args[0],
                Email: email,
            }

            // 2. Execute Application Write Use-Case
            if err := createHandler.Handle(cmd.Context(), input); err != nil {
                return fmt.Errorf("create user failed: %w", err)
            }

            cmd.Println("User created successfully!")
            return nil
        },
    }

    cmd.Flags().StringVarP(&email, "email", "e", "", "User email address")
    _ = cmd.MarkFlagRequired("email")

    return cmd
}

```

---

### D. Application Use-Case (`internal/feature/user/command/create_user.go`)

Orchestrates domain entities and infrastructure ports without knowing *how* it was invoked (CLI vs API).

```go
package command

import (
    "context"
    "my-app/internal/feature/user/domain"
)

type CreateUserCommand struct {
    Name  string
    Email string
}

type CreateUserHandler struct {
    repo domain.UserRepository
}

func NewCreateUserHandler(repo domain.UserRepository) CreateUserHandler {
    return CreateUserHandler{repo: repo}
}

func (h CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) error {
    user, err := domain.NewUser(cmd.Name, cmd.Email)
    if err != nil {
        return err // Return pure domain error
    }

    return h.repo.Save(ctx, user)
}

```

---

### E. Binary Entry Point (`cmd/cli/main.go`)

Initializes resources, wires infrastructure dependencies, constructs Cobra command sub-trees, and boots the CLI application.

```go
package main

import (
    "context"
    "os"
    "github.com/spf13/cobra"

    "my-app/internal/resources/db"
    usercmd "my-app/internal/feature/user/command"
    usercli "my-app/internal/feature/user/cli"
    userinfra "my-app/internal/feature/user/infra"
)

func main() {
    ctx := context.Background()

    // 1. Initialize Shared Resources (DB Pool, LLM Clients, etc.)
    dbPool, err := db.NewPostgresPool(ctx, os.Getenv("DATABASE_URL"))
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to init db: %v\n", err)
        os.Exit(1)
    }
    defer dbPool.Close()

    // 2. Initialize Infrastructure Implementations
    userRepo := userinfra.NewPostgresUserRepository(dbPool)

    // 3. Initialize Application Handlers
    createUserHandler := usercmd.NewCreateUserHandler(userRepo)

    // 4. Assemble Cobra Tree
    rootCmd := &cobra.Command{Use: "app"}
    rootCmd.AddCommand(usercli.NewUserCmd(createUserHandler))

    // 5. Execute
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

```

---

## 5. Summary Checklist for Code Reviews

* [ ] Are raw client initializers (DB pools, OpenAI SDKs, Redis connections) isolated in `internal/resources/`?
* [ ] Is `*cobra.Command` restricted **exclusively** to `cmd/` and `internal/*/cli` packages?
* [ ] Do `domain` packages contain **zero** third-party framework dependencies?
* [ ] Do Cobra commands use `RunE` (returning errors) instead of exiting abruptly via `os.Exit()` inside command handlers?
* [ ] Are global Cobra flags (e.g., `--verbose`, `--config`) passed into adapters via constructors or context rather than importing `cmd/root.go` (avoiding import cycles)?
* [ ] Is each Bounded Context (`internal/feature/user/`) self-contained, high-cohesion, and ready to be sliced into a standalone service if needed?
* [ ] Are end-to-end and cross-boundary tests placed in root `test/` using a single centralized `test/testdata/` directory (organized by domain/shared subfolders), while unit and slice integration tests are colocated within their domain packages (`*_test.go` and `testdata/`)?
