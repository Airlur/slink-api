# slink-api Service Context

## Project Overview
**slink-api** is a lightweight, high-performance URL shortening service written in Go. It provides long URL to short code conversion (Base62), redirection, comprehensive access statistics, and user management. It is designed for personal or small team use, featuring security measures like rate limiting, CAPTCHA, and role-based access control.

## Interaction Standards
*   **Precision:** Do not guess. Do not use words like "maybe", "perhaps", or "possibly" when referring to code logic. Verify first.
*   **Analysis First:** Before refactoring, thoroughly analyze the existing codebase, understand the data flow, and identify specific bottlenecks or risks.
*   **Discussion:** Present findings and a proposed solution to the user for discussion before writing code, especially for complex architectural changes.
*   **Documentation:** Keep this `GEMINI.md` updated with project conventions and architectural decisions.

## Technology Stack
*   **Language:** Go (1.24+)
*   **Web Framework:** [Gin](https://github.com/gin-gonic/gin)
*   **Database:** MySQL (via [GORM](https://gorm.io/))
*   **Cache & Rate Limiting:** Redis (via [go-redis](https://github.com/redis/go-redis))
*   **Configuration:** [Viper](https://github.com/spf13/viper)
*   **Logging:** [Zap](https://github.com/uber-go/zap)
*   **Authentication:** JWT (Access & Refresh Tokens)
*   **Job Scheduling:** [Robfig Cron](https://github.com/robfig/cron)

## Project Structure
The project follows a standard Go project layout (Clean Architecture-ish):

```text
slink-api/
├── cmd/                # Entry points
│   └── main.go         # Main server entry point
├── configs/            # Configuration files (config.yaml)
├── internal/           # Private application code
│   ├── api/            # HTTP Handlers (Controllers), Middleware, Routes
│   │   └── v1/         # V1 API implementation
│   ├── bootstrap/      # App initialization (DB, Redis, Logger, Router)
│   ├── dto/            # Data Transfer Objects (Request/Response structs)
│   ├── model/          # Database Models (GORM structs)
│   ├── repository/     # Data Access Layer (DB operations)
│   └── service/        # Business Logic Layer
├── pkg/                # Public library code (Utilities)
│   ├── config/         # Config loader
│   ├── logger/         # Logger setup
│   ├── response/       # Standard API response helpers
│   └── ...             # Other utils (jwt, geoip, base62, etc.)
├── scripts/            # Build scripts and SQL files
│   ├── sql/            # Database schema SQLs
│   └── gen_module.bat  # Code generation script
├── logs/               # Application logs
├── templates/          # Templates for code generation
├── Makefile            # Build and utility commands
├── go.mod              # Go module definition
└── README.md           # Project documentation
```

## Key Features
*   **User Management:** Registration, Login, Password Reset (Email/Captcha), Role Management (User/Admin).
*   **Short Links:** Create (Auto/Custom), Redirect (302), Expiry Management, Tagging.
*   **Statistics:** Access logs, Click counts, Region/Device/Referer analysis.
*   **Security:** Rate limiting (Global/IP/User), Captcha, Account Locking, HTTPS enforcement (prod).

## Development Workflow

### Prerequisites
1.  **Go:** 1.24 or later.
2.  **MySQL:** Database server running.
3.  **Redis:** Cache server running.

### Configuration
1.  Copy/Edit `configs/config.yaml`.
2.  Set database credentials (`database.host`, `database.username`, `database.password`).
3.  Set Redis connection (`redis.host`, `redis.password`).
4.  Configure Email SMTP for notifications.

### Build & Run
Use the `Makefile` for common tasks:

*   **Run Locally:**
    ```bash
    make run
    ```
*   **Build Binary:**
    ```bash
    make build
    ```
    Output: `bin/server`
*   **Clean Build:**
    ```bash
    make clean
    ```

### Code Generation
To quickly scaffold a new module (Model, DTO, Repository, Service, Handler) from a SQL file:

1.  Ensure a corresponding SQL file exists in `scripts/sql/` (e.g., `new_feature.sql`).
2.  Run the generation command:
    ```bash
    make gen MODULE=new_feature
    ```
    *Shortcut:* `make gen-shortlink` (for specific modules defined in Makefile).

### Testing
*   **Run Tests:**
    ```bash
    make test
    ```
    *(Note: Ensure `go test` works in your environment)*

## Coding Conventions
*   **Layered Architecture:** strict separation of concerns: `API -> Service -> Repository -> Model`.
*   **Error Handling:** Use `internal/pkg/errors` and `response` package for consistent HTTP error responses.
*   **Configuration:** All configurable values should be in `config.yaml` and loaded via `viper`.
*   **Logging:** Use `logger.Info`, `logger.Error` etc., from `internal/pkg/logger`.
*   **Naming:**
    *   Files: `snake_case.go`
    *   Structs/Interfaces: `PascalCase`
    *   Variables: `camelCase`