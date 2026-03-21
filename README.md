```
     _)
  __               _      __  __
 / _|_ _ ___  ___ | |__  |  \/  |__ _ _ _  __ _ ___
|  _| '_/ -_)(_-< | '_ \ | |\/| / _` | ' \/ _` / _ \
|_| |_| \___)/___||_||_| |_|  |_\__,_|_||_\__, \___/
                                           |___/
```

[![CI](https://github.com/juandagalo/fresh-mango/actions/workflows/ci.yml/badge.svg)](https://github.com/juandagalo/fresh-mango/actions/workflows/ci.yml)

A terminal UI for Linux fan control via [nbfc-linux](https://github.com/nbfc-linux/nbfc-linux), built with the [Charmbracelet](https://charm.sh/) stack.

## Features

- **Dashboard** — Live CPU temperature and fan speed monitoring with real-time graphs
- **Curve Editor** — Visual temperature-to-fan-speed threshold editor
- **Installer Wizard** — Guided setup for nbfc-linux detection, model configuration, and service activation
- **Hub Navigation** — Central hub-and-spoke menu for quick access to all views
- **Warm Ember Theme** — Amber/teal color palette designed for extended terminal sessions

## Screenshots

<!-- Screenshots coming soon -->

## Requirements

- **Go** 1.24+
- **Linux** (nbfc-linux is Linux-only)
- [**nbfc-linux**](https://github.com/nbfc-linux/nbfc-linux) installed and available on `$PATH`

## Installation

### From source (recommended)

```bash
git clone https://github.com/juandagalo/fresh-mango.git
cd fresh-mango
make build
```

The binary is written to `./freshMango`.

### With `go install`

```bash
go install github.com/mango/freshMango@latest
```

## Usage

```bash
# Run freshMango (some nbfc operations require root)
freshMango

# Or with sudo for full fan control
sudo freshMango
```

On first run, if nbfc-linux is not detected or not configured, freshMango launches the **Installer Wizard** to walk you through setup. Once configured, the app opens to the **Hub** where you can navigate to any view.

### Key Bindings

| Key          | Action                         |
|--------------|--------------------------------|
| `↑` / `k`   | Move up                        |
| `↓` / `j`   | Move down                      |
| `1`–`6`     | Jump to menu item              |
| `Enter`      | Select / confirm               |
| `Esc`        | Back to hub                    |
| `q`          | Quit                           |

## Architecture

```
freshMango/
├── main.go                 # Entry point — launches Bubble Tea program
├── internal/
│   ├── tui/                # Terminal UI layer (Bubble Tea models & views)
│   │   ├── app.go          # Root model, startup state machine, view routing
│   │   ├── hub.go          # Hub menu (central navigation)
│   │   ├── dashboard.go    # Live monitoring view
│   │   ├── curve.go        # Curve editor view
│   │   ├── installer.go    # Installer wizard (setup flow)
│   │   └── styles.go       # Warm Ember theme (colors & Lipgloss styles)
│   ├── nbfc/               # nbfc-linux CLI wrapper
│   │   └── nbfc.go         # Commands: status, config set/list, start/stop
│   └── system/             # System-level utilities
│       └── system.go       # Root detection, command execution helpers
├── Makefile                # Build, test, lint, coverage targets
└── .github/
    └── workflows/ci.yml    # CI pipeline (lint, test, build)
```

**Stack**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (TUI framework) · [Lipgloss](https://github.com/charmbracelet/lipgloss) (styling) · [nbfc-linux](https://github.com/nbfc-linux/nbfc-linux) (fan control backend)

## Roadmap

| Phase | Focus                   | Status |
|-------|-------------------------|--------|
| 1     | Foundation              | ✅ Complete — theme, hub, installer wizard, smart entry routing |
| 2     | Core Controls           | 🔜 Next — fan speed override, profile switching |
| 3     | Profiles & Sensors      | 🔜 Planned — sensor configuration UI, config rating system |
| 4     | IPC & Polish            | 🔜 Planned — real-time IPC, dashboard enhancements, settings |

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on branching, commit messages, and the pull request process.

This project uses [Conventional Commits](https://www.conventionalcommits.org/) and enforces quality gates via `make check`.

## License

This project is released under the [MIT License](LICENSE).
