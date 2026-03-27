# Contributing to LazySpeed

Thanks for your interest in contributing. Here's what you need to know to get started.

## Prerequisites

- Go 1.24+
- [golangci-lint](https://golangci-lint.run/) v2

## Getting Started

```bash
git clone https://github.com/jkleinne/lazyspeed.git
cd lazyspeed
go mod download
make build
```

## Development Commands

| Command | Description |
|---------|-------------|
| `make test` | Run tests with race detector |
| `make lint` | Run golangci-lint |
| `make build` | Build binary |
| `make cover` | Generate HTML coverage report |

## Code Style

The codebase follows a few conventions worth knowing before you open a PR.

### Error handling

Wrap errors with context: `fmt.Errorf("failed to <action>: %v", err)` — always `%v`, not `%w`. For non-critical failures, set the `model.Warning` field instead of returning an error. There's no logging library; output goes through TUI model fields (`.Error`, `.Warning`) or `fmt.Fprintf(os.Stderr, ...)` for CLI mode.

### Testing

Tests use stdlib `testing` only — no testify or third-party assertion libraries. Test files live in the same package as the code they test (white-box).

Naming follows `Test<Function>` or `Test<Function><Scenario>`. Use `t.Errorf` for non-fatal checks and `t.Fatalf` when a precondition is broken. For anything with multiple cases, use table-driven tests with a `tests` slice, `tt` iterator, and `t.Run(tt.name, ...)`.

Mocks are function-field structs where a nil field returns a sensible default — only set the fields relevant to each test case. For filesystem tests, use `t.TempDir()` and `t.Setenv("HOME", tmpDir)`.

### Architecture

The project keeps a strict separation between model and UI. `model/` owns business state with no UI imports. `ui/` has stateless render functions that take data params and return styled strings. `main.go` bridges them.

State transitions use typed enums (`ModelState`, `ViewState`) dispatched through nested switches. The [`exhaustive`](https://github.com/nishanths/exhaustive) linter enforces complete coverage, so adding a new state variant means adding a case everywhere it's switched on.

A few other things to keep in mind:

- Named constants for domain/config values, no magic numbers
- File permissions: `0600` for history (contains PII), `0644` for exports
- Interfaces live in the consumer package, not the provider
- Pointer receivers on stateful structs (`*Model`, `*speedTest`)
- `strings.Builder` for all string composition in views

## Branching

Branch off `main` with the format `type/short-description` (e.g., `feat/add-auth`, `fix/nav-crash`). Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`, `build`.

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description
```

## Pull Requests

1. Branch off latest `main`
2. All tests pass (`make test`)
3. Lint clean (`make lint`)
4. One coherent change per PR
