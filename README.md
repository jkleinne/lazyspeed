# LazySpeed

A terminal-based internet speed test with an interactive TUI and a headless CLI for scripting and CI.

[![CI](https://github.com/jkleinne/lazyspeed/actions/workflows/ci.yml/badge.svg)](https://github.com/jkleinne/lazyspeed/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/jkleinne/lazyspeed)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/jkleinne/lazyspeed)](https://github.com/jkleinne/lazyspeed/releases)

<img width="480" alt="image" src="https://github.com/user-attachments/assets/2988d63e-3fcf-42de-83f2-9bce5e00106f" />
<img width="346" alt="image" src="https://github.com/user-attachments/assets/d191a282-760e-4b96-b93b-595c1c409104" />

## Features

- **Download & upload** speed measurement with live EWMA progress
- **Ping & jitter:** configurable ping count, jitter as Mean Absolute Deviation
- **Network diagnostics:** traceroute, DNS timing, quality scoring (`lazyspeed diag`)
- **Interactive TUI:** server selection, results, history, export, help overlay
- **Headless CLI:** JSON, CSV, and one-liner output for scripting and CI
- **Persistent history:** XDG-compliant storage with configurable retention
- **Multi-test runs:** `--count N` for sequential tests
- **Shell completions:** bash, zsh, fish, powershell
- **Single binary:** zero runtime dependencies, cross-platform (macOS, Linux, Windows)

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap jkleinne/tools
brew install lazyspeed
```

### GitHub Releases

Download the latest binary from [Releases](https://github.com/jkleinne/lazyspeed/releases).

### Build from Source

Requires Go 1.24+.

```bash
git clone https://github.com/jkleinne/lazyspeed.git
cd lazyspeed
make build
```

## Usage

### TUI (Interactive)

```bash
lazyspeed
```

| Key | Action |
|-----|--------|
| `n` | Start a new speed test |
| `d` | Run network diagnostics |
| `e` | Export latest result (JSON or CSV) |
| `h` | Toggle help overlay |
| `↑`/`↓` or `k`/`j` | Navigate lists |
| `Enter` | Select |
| `Esc` | Back |
| `q` / `Ctrl+C` | Quit |

### Speed Test

```bash
# Default (human-readable progress on stderr)
lazyspeed run

# Structured output
lazyspeed run --json
lazyspeed run --csv
lazyspeed run --simple          # DL: X Mbps | UL: X Mbps | Ping: X ms

# Options
lazyspeed run --server <id>     # Use a specific server
lazyspeed run --no-upload       # Skip upload phase
lazyspeed run --no-download     # Skip download phase
lazyspeed run --count 5 --json  # Run 5 sequential tests
```

### Network Diagnostics

```bash
# Interactive diagnostics in TUI
lazyspeed diag

# Headless with structured output
lazyspeed diag --json
lazyspeed diag --csv
lazyspeed diag --simple

# Specify target
lazyspeed diag --server <host>

# View diagnostics history
lazyspeed diag --history
lazyspeed diag --history --last 5
```

### History

```bash
lazyspeed history                    # Table view
lazyspeed history --format json      # JSON output
lazyspeed history --format csv       # CSV output
lazyspeed history --last 10          # Last N entries
lazyspeed history --clear            # Delete all history
```

### Server List

```bash
lazyspeed servers                    # Table of servers sorted by latency
lazyspeed servers --format json
lazyspeed servers --format csv
```

### Other Commands

```bash
lazyspeed version                    # Version, commit, build date
lazyspeed completion bash            # Shell completions (bash/zsh/fish/powershell)
```

## Configuration

Config file: `~/.config/lazyspeed/config.yaml` (respects `$XDG_CONFIG_HOME`)

```yaml
history:
  max_entries: 50          # Maximum history entries (default: 50)
  path: ""                 # Override history path (default: ~/.local/share/lazyspeed/history.json)

test:
  ping_count: 10           # Ping iterations per test (default: 10)
  fetch_timeout: 30        # Server list fetch timeout in seconds (default: 30)
  test_timeout: 120        # Speed test timeout in seconds (default: 120)

export:
  directory: ""            # Default export directory (default: current directory)
```

All settings are optional: sensible defaults are used when omitted.

## Comparison

| Feature | LazySpeed | Ookla CLI | fast-cli | speedtest-go CLI |
|---------|-----------|-----------|----------|------------------|
| Interactive TUI | **Yes** | No | No | No |
| Headless CLI | **Yes** | Yes | Yes | Yes |
| Persistent history | **Yes** | No | No | No |
| JSON/CSV export | **Yes** | JSON only | No | JSON only |
| Network diagnostics | **Yes** | No | No | No |
| Configurable | **Yes** (YAML) | Limited | No | No |
| Single binary | **Yes** | Yes | No (Node.js) | Yes |
| Cross-platform | **macOS, Linux, Windows** | All | All | All |
| Open source | **MIT** | Proprietary | MIT | MIT |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, coding conventions, and PR guidelines.

## License

[MIT](LICENSE)
