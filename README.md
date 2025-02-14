# LazySpeed - Terminal Speed Test

A simple terminal-based internet speed test application built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

<img width="500" alt="image" src="https://github.com/user-attachments/assets/2988d63e-3fcf-42de-83f2-9bce5e00106f" />
<img width="350" alt="image" src="https://github.com/user-attachments/assets/91a25e2b-3ef0-4d12-b2f0-4a18fff67c84" />


## Features

- 📥 Download speed measurement
- 📤 Upload speed measurement
- 🔄 Ping/Latency testing
- 📊 Jitter calculation
- 🌍 Server information display
- 📈 Real-time progress visualization
- 🎨 Beautiful TUI with responsive design

## Installation

### Prerequisites

- Go 1.21 or higher

### Building from Source

1. Clone the repository:
```bash
git clone https://github.com/yourusername/lazyspeed.git
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
./lazyspeed
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

The beautiful terminal user interface is built using:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions

## License

MIT License
