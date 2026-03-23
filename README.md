# LazySpeed - Terminal Speed Test

A simple terminal-based internet speed test application built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

<img width="480" alt="image" src="https://github.com/user-attachments/assets/2988d63e-3fcf-42de-83f2-9bce5e00106f" />
<img width="346" alt="image"  alt="image" src="https://github.com/user-attachments/assets/d191a282-760e-4b96-b93b-595c1c409104" />

## Features

- 📥 Download speed measurement
- 📤 Upload speed measurement
- 🔄 Ping/Latency testing
- 📊 Jitter calculation (Mean Absolute Deviation)
- 🖥️ Headless CLI mode (`lazyspeed run`) for scripting and CI
- 📜 Test history tracking with persistent storage
- 📁 JSON and CSV result export (CLI and TUI)
- ⚙️ XDG-compliant YAML configuration
- 🔁 Multi-test runs (`--count N`)

## Installation

### Using Homebrew

1. Add the tap:
```bash
brew tap jkleinne/tools
```

2. Install LazySpeed:
```bash
brew install lazyspeed
```

### Building from Source

#### Prerequisites

- Go 1.24.2 or higher

1. Clone the repository:
```bash
git clone https://github.com/jkleinne/lazyspeed.git
cd lazyspeed
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application (setting version and build date via `ldflags`):
```bash
go build
```

## Usage

### TUI Mode

Launch the interactive terminal UI:
```bash
lazyspeed
```

### `run` — Headless Speed Test

Run a speed test non-interactively (useful for scripting and CI):
```bash
# Default output
lazyspeed run

# JSON output
lazyspeed run --json

# CSV output
lazyspeed run --csv

# Minimal one-line output (DL/UL/Ping)
lazyspeed run --simple

# Use a specific server
lazyspeed run --server <server-id>

# Skip upload or download
lazyspeed run --no-upload
lazyspeed run --no-download

# Run multiple tests
lazyspeed run --count 5 --json
```

### `history` — View Test History

```bash
# Display history as a table
lazyspeed history

# Export as JSON or CSV
lazyspeed history --format json
lazyspeed history --format csv

# Limit to last N results
lazyspeed history --last 10 --format json

# Clear all history
lazyspeed history --clear
```

### `version`

```bash
lazyspeed version
```

### TUI Controls

| Key | Action |
|-----|--------|
| `n` | Start a new speed test |
| `e` | Export latest result (after test completes) |
| `h` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

**Server selection:**

| Key | Action |
|-----|--------|
| `↑`/`↓` or `k`/`j` | Navigate server list |
| `Enter` | Select server |
| `Esc` | Back to home |
| `q` / `Ctrl+C` | Quit |

**Export prompt (after pressing `e`):**

| Key | Action |
|-----|--------|
| `j` | Export as JSON |
| `c` | Export as CSV |
| `Esc` / `q` / `Ctrl+C` | Cancel |

## Configuration

LazySpeed uses XDG-compliant paths for configuration and data storage.

**Config file:** `~/.config/lazyspeed/config.yaml`

```yaml
history:
  max_entries: 50    # Maximum history entries to keep (default: 50)
  path: ""           # Override history file path (default: ~/.local/share/lazyspeed/history.json)

test:
  ping_count: 10     # Number of ping measurements per test (default: 10)
```

**History file:** `~/.local/share/lazyspeed/history.json`

The config file is optional — all settings have sensible defaults. If migrating from an older version, history is automatically moved from the legacy path (`~/.lazyspeed_history.json`).

## How it Works

LazySpeed uses the [speedtest-go](https://github.com/showwin/speedtest-go) library to perform internet speed tests. The application:

1. Finds the closest speed test server
2. Measures ping and calculates jitter
3. Performs download speed test
4. Performs upload speed test
5. Displays the results

UI is built using:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions

## License

MIT License
