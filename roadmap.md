# LazySpeed v1.0 Roadmap

## Current State: v0.1.6

**~997 lines of Go** across 3 packages (`main`, `model`, `ui`). A functional TUI speed test app with download/upload measurement, ping/jitter, server selection, and history persistence. Key gaps: minimal tests (<10% coverage), no CI for PRs, no linting, incomplete error handling (including a division-by-zero bug), no CLI flags, and no GoDoc.

---

## Phase 1: Bug Fixes & Critical Stability
**Goal:** Fix known bugs and merge pending work. Zero regressions.

### 1.1 Fix division-by-zero in ping calculation
- `model/model.go:312` — `sumPing / float64(len(m.PingResults))` panics if all 10 pings fail
- Guard against empty `PingResults`: return an error or use a sensible default (0.0)

### 1.2 Handle discarded errors
- `model/model.go:325` — `_ = m.SaveHistory()` silently discards write failures. Surface this as a non-fatal error (e.g., via the progress channel or a warning field on Model)
- `model/model.go:158-161` — `FetchUserInfo` error silently swallowed. Log or surface as a warning

### 1.3 Merge pending feature branches
- `feat/async-server-fetch-and-cancel` (1 commit ahead of main) — review and merge
- `feature/real-progress-bar` — evaluate, merge or close
- `feature/show-server-sponsor` — evaluate, merge or close
- Clean up stale remote branches (28+ branches, many with auto-generated names)

### 1.4 Extract magic numbers into named constants
- `1000000` (bytes-to-MB conversion) → `const bytesToMB = 1_000_000`
- `10` (ping iterations) → `const pingIterations = 10`
- `200ms` (progress ticker) → `const progressInterval = 200 * time.Millisecond`
- `100ms` (ping delay) → `const pingDelay = 100 * time.Millisecond`
- `15000` (assumed test duration for progress) → `const estimatedTestDurationMs = 15_000`
- `50` (max history) — already a const, but should be configurable (see Phase 3)

---

## Phase 2: Testing & CI/CD
**Goal:** Establish confidence in the codebase. Target 60%+ code coverage.

### 2.1 Introduce interfaces for testability
- Define a `SpeedTestBackend` interface wrapping `speedtest-go` operations (`FetchServers`, `FetchUserInfo`, `PingTest`, `DownloadTest`, `UploadTest`)
- Inject backend into `Model` (constructor parameter or option pattern)
- This decouples business logic from the network layer and enables mocking

### 2.2 Write unit tests for core logic
- **`model` package:**
  - `PerformSpeedTest` — test all phases (ping, download, upload), cancellation, error paths, progress updates
  - `FetchServerList` — test sorting, error handling
  - `LoadHistory` / `SaveHistory` — test file I/O (temp dir), corrupt data, max-size trimming, missing file
  - Ping failure edge cases (all pings fail, partial failure)
- **`ui` package:**
  - `RenderSpinner`, `RenderResults`, `RenderError`, `RenderHelp`, `RenderServerSelection` — verify output contains expected content and handles edge cases (empty data, nil pointers, extreme widths)
- **`main` package:**
  - `Update` message handling — verify state transitions for each `tea.Msg` type
  - `View` rendering — verify correct view shown for each app state
- **`version.go`:**
  - `GetVersionInfo` — verify output format

### 2.3 Set up CI workflow for pull requests
- `.github/workflows/ci.yml` triggered on push to `main` and all PRs:
  - `go vet ./...`
  - `golangci-lint run`
  - `go test -race -coverprofile=coverage.out ./...`
  - Coverage report upload (e.g., Codecov or just artifact)
  - `go build` to verify compilation

### 2.4 Configure linting
- Add `.golangci.yml` with linters: `govet`, `errcheck`, `staticcheck`, `unused`, `gosimple`, `ineffassign`, `goconst`, `gofmt`
- Add `Makefile` with targets: `test`, `lint`, `build`, `cover`, `clean`

### 2.5 Add dependency security scanning
- Enable Dependabot or Renovate for automated dependency updates
- Add `govulncheck` to CI pipeline

---

## Phase 3: CLI Flags & Configuration
**Goal:** Make LazySpeed configurable for power users and scripting. **Breaking change:** introduce proper CLI parsing.

### 3.1 Adopt a CLI framework
- Replace manual `os.Args` parsing with `cobra` or `kong`
- Subcommands: `lazyspeed` (default TUI), `lazyspeed version`, `lazyspeed run` (non-interactive), `lazyspeed history`

### 3.2 Add CLI flags
- `--json` — output results as JSON to stdout (for piping/scripting)
- `--csv` — output results as CSV
- `--server <id>` — skip server selection, use a specific server
- `--no-upload` / `--no-download` — skip a phase
- `--count <n>` — run multiple tests sequentially
- `--simple` — minimal output (one line: DL/UL/Ping)

### 3.3 Configuration file support
- Config file at `~/.config/lazyspeed/config.yaml` (XDG-compliant)
- Configurable options:
  - `history.max_entries` (default 50)
  - `history.path` (default `~/.local/share/lazyspeed/history.json` — **breaking change** from `~/.lazyspeed_history.json`)
  - `test.ping_count` (default 10)
  - `ui.theme` (for Phase 5)
- Auto-migrate old history file location on first run

### 3.4 Improve history file
- **Breaking change:** Move from `~/.lazyspeed_history.json` to XDG-compliant `~/.local/share/lazyspeed/history.json`
- Tighten file permissions to `0600` (contains IP address — PII)
- Add optional `--clear-history` flag
- Validate/sanitize history data on load (guard against corrupt JSON)

---

## Phase 4: Result Export
**Goal:** Allow users to extract and share their speed test data.

### 4.1 JSON export
- `lazyspeed history --format json` — dump full history as JSON to stdout
- `lazyspeed history --format json --last 5` — export last N results
- Human-readable JSON with proper field names (snake_case)

### 4.2 CSV export
- `lazyspeed history --format csv` — dump history as CSV
- Headers: `timestamp,server,country,download_mbps,upload_mbps,ping_ms,jitter_ms,ip,isp`

### 4.3 Single-test export
- After a test completes in TUI mode, press `e` to export the latest result
- Prompt for format (JSON/CSV) and write to file or clipboard

---

## Phase 5: Polish & Ecosystem Maturity
**Goal:** Developer experience, documentation, and distribution readiness.

### 5.1 GoDoc and code documentation
- Add package-level doc comments for `model` and `ui`
- Add GoDoc comments on all exported types, functions, methods, and constants
- Add inline comments explaining non-obvious logic (EWMA, jitter MAD calculation, progress estimation)

### 5.2 Centralize UI theming
- Extract all hardcoded colors into a `Theme` struct in `ui/theme.go`
- Single source of truth for the color palette (`#7D56F4` currently repeated 7 times)
- Make spinner box width and progress bar width responsive to terminal size (currently hardcoded to 70 and 50)

### 5.3 API surface cleanup
- Unexport internal `Model` fields that don't belong in the public API (`PingResults`, `FetchingServers`, `PendingServerSelection`, `Testing`, `Cursor`, `SelectingServer`)
- Either use accessor methods or restructure so `main` doesn't reach into model internals
- Decouple `ui` from `model` — render functions should accept data structs or interfaces, not `*model.Model` directly

### 5.4 Contributor experience
- Add `CONTRIBUTING.md` with setup instructions, coding conventions, PR process
- Add `CHANGELOG.md` (or auto-generate from conventional commits)
- Add `.editorconfig` for consistent formatting across editors
- Add pre-commit hook configuration (optional, via `pre-commit` or `lefthook`)

### 5.5 Distribution expansion
- Add to more package managers: `scoop` (Windows, for future), `nix`, `snap`, AUR
- Evaluate `pkgx` or `webi` for easy install scripts
- Add `goreleaser` checksum file generation for verification
- Update Go minimum version to 1.22+ (enables range-over-int and other modern features)

### 5.6 README overhaul
- Update feature list to reflect v1.0 capabilities
- Add CLI usage examples with flags
- Add configuration documentation
- Add "Comparison with alternatives" section (speedtest-cli, fast.com CLI, etc.)
- Add badges (CI status, Go version, latest release, license)

---

## Phase 6: Final Release Preparation
**Goal:** Ship v1.0 with confidence.

### 6.1 Release candidate testing
- Tag `v1.0.0-rc.1`, distribute for testing
- Verify on macOS (amd64 + arm64) and Linux (amd64 + arm64)
- Test Homebrew tap installation end-to-end
- Verify config migration from old history file
- Run full test suite with `-race` flag

### 6.2 Version bump and tagging
- Update any hardcoded references to the version
- Tag `v1.0.0`
- GoReleaser produces final artifacts

### 6.3 Announcement
- GitHub Release with detailed changelog
- Update Homebrew tap
