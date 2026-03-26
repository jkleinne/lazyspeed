# Contributing to LazySpeed

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

## Code Conventions

### Error Handling

- **Wrap with context:** `fmt.Errorf("failed to <action>: %v", err)`: use `%v`, not `%w`
- **Non-critical failures:** set `model.Warning` string field instead of returning an error
- **Context cancellation:** check `ctx.Err()` at phase boundaries; return the raw context error
- **Missing files:** `os.IsNotExist(err)`: return nil gracefully
- **No logging library.** Output through TUI model fields (`.Error`, `.Warning`) or `fmt.Fprintf(os.Stderr, ...)` for CLI errors

### Testing

- **stdlib `testing` only:** no testify, no third-party assertion libraries
- **White-box testing:** test files use the same package name as the code under test
- **Test naming:** `Test<Function>` or `Test<Function><Scenario>`
- **Assertions:** `t.Errorf` for non-fatal, `t.Fatalf` for fatal preconditions. Message format: `"Expected <expectation>, got <actual>"`
- **Table-driven:** slice named `tests`, iterator named `tt`, subtests via `t.Run(tt.name, ...)`
- **Mock pattern:** function-field struct; nil field returns sensible default. Configure only the fields relevant to each test
- **Filesystem tests:** `t.TempDir()` + `t.Setenv("HOME", tmpDir)`

### Key Patterns

- **State machine:** typed enums for state (`ModelState`, `ViewState`). `Update()`/`View()` dispatch via nested switches to focused handler functions. The [`exhaustive`](https://github.com/nishanths/exhaustive) linter enforces complete switch coverage, so add a case for every variant.
- **Goroutine lifecycle:** `done`/`doneAck` channel pairs. Close `done` to signal, block on `<-doneAck` to confirm exit.
- **Nil-safe guards:** optional channels and cancel functions are nil-checked before use.
- **Model/UI separation:** `model/` owns business state with zero UI dependency. `ui/` has stateless render functions that take data and return styled strings. `main.go` bridges them.

### Other Conventions

- Named constants for all domain/configuration values: no magic numbers
- File permissions: `0600` for history (contains PII), `0644` for exports
- Interfaces defined in the consumer package, not the provider package
- Pointer receivers for all stateful structs (`*Model`, `*speedTest`)
- `strings.Builder` for all view and string composition

## Branching

- Branch off `main`: `type/short-description` (e.g., `feat/add-auth`, `fix/nav-crash`)
- Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`, `build`

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description
```

Keep the subject under 72 characters, imperative mood ("add feature" not "added feature").

## Pull Requests

1. Branch off latest `main`
2. All tests pass (`make test`)
3. Lint clean (`make lint`)
4. One coherent change per PR
