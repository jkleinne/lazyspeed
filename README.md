# LazySpeed - Terminal Speed Test

A simple terminal-based internet speed test application built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

<img width="480" alt="image" src="https://github.com/user-attachments/assets/2988d63e-3fcf-42de-83f2-9bce5e00106f" />
<img width="346" alt="image" src="https://github.com/user-attachments/assets/af8856ea-45c1-4d5a-8361-378ca76bc7e9" />

## Features

- 📥 Download speed measurement
- 📤 Upload speed measurement
- 🔄 Ping/Latency testing
- 📊 Jitter calculation (Mean Absolute Deviation)

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

- Go 1.21 or higher

1. Clone the repository:
```bash
git clone https://github.com/jkleinne/lazyspeed.git
cd lazyspeed
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o lazyspeed
```

## Usage

Run the application:
```bash
lazyspeed
```

Display version information:
```bash
lazyspeed version
```

### Controls

- Press `n` to start a new speed test
- Press `h` to toggle help menu
- Press `q` to quit the application

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
