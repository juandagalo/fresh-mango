# Contributing to freshMango

Thank you for your interest in contributing! This guide covers the development workflow, branch conventions, and quality standards for the project.

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork:
   ```bash
   git clone https://github.com/<your-user>/fresh-mango.git
   cd fresh-mango
   ```
3. **Install dependencies**:
   - Go 1.24+ ([download](https://go.dev/dl/))
   - [golangci-lint](https://golangci-lint.run/welcome/install-locally/) (for linting)
4. **Verify your setup**:
   ```bash
   make check   # runs vet + lint + tests
   ```

## Branch Naming Convention

Use the following prefixes for all branches:

| Prefix                          | Purpose                      | Example                               |
|---------------------------------|------------------------------|---------------------------------------|
| `feat/{phase}-{feature}`        | New features                 | `feat/p2-fan-speed-override`          |
| `fix/{scope}-{description}`     | Bug fixes                    | `fix/nbfc-empty-config-dir`           |
| `refactor/{scope}-{description}`| Code restructuring           | `refactor/tui-extract-chart-render`   |
| `docs/{description}`            | Documentation changes        | `docs/contributing-guide`             |
| `test/{scope}`                  | Test additions/improvements  | `test/nbfc-status-parsing`            |
| `release/v{semver}`             | Release preparation          | `release/v0.2.0`                      |

## Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification. Every commit message should follow this format:

```
<type>(<scope>): <short description>

[optional body]

[optional footer(s)]
```

### Types

| Type       | When to use                                  |
|------------|----------------------------------------------|
| `feat`     | A new feature                                |
| `fix`      | A bug fix                                    |
| `refactor` | Code change that neither fixes nor adds      |
| `docs`     | Documentation only                           |
| `test`     | Adding or updating tests                     |
| `chore`    | Tooling, CI, dependencies, repo maintenance  |
| `style`    | Formatting, whitespace (no logic change)     |
| `perf`     | Performance improvement                      |

### Scopes

Use the package or area being changed: `tui`, `nbfc`, `system`, `ci`, `build`.

### Examples

```
feat(tui): add fan speed override slider
fix(nbfc): handle empty config directory
refactor(tui): extract chart rendering into separate model
docs: add contributing guide
test(system): add root detection unit tests
chore(ci): add coverage upload to CI pipeline
```

## Pull Request Process

1. Create a branch following the [naming convention](#branch-naming-convention)
2. Make your changes in small, focused commits
3. Ensure all quality checks pass:
   ```bash
   make check   # vet + lint + tests
   ```
4. Push your branch and open a pull request against `main`
5. Fill in the [PR template](.github/PULL_REQUEST_TEMPLATE.md):
   - Describe **what** and **why**
   - Select the PR type (`feat`, `fix`, etc.)
   - Indicate which **phase** the change belongs to
   - Complete the checklist
6. Wait for CI to pass and address any review feedback

## Development Workflow

### Common Make Targets

```bash
make build          # Compile the binary
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run golangci-lint
make vet            # Run go vet
make check          # Full quality gate (vet + lint + tests)
make clean          # Remove build artifacts
```

### Before Pushing

Always run the full quality gate before pushing:

```bash
make check
```

This runs `go vet`, `golangci-lint`, and `go test` in sequence. CI will reject PRs that fail any of these.

### Code Formatting

Go code must be formatted with `gofmt`:

```bash
gofmt -w .
```

The CI lint step enforces `gofmt` compliance via `golangci-lint`.

## Code Style

- Follow standard **Go idioms** and conventions ([Effective Go](https://go.dev/doc/effective_go))
- Use the **Bubble Tea** model pattern: `Init()`, `Update()`, `View()` for all TUI components
- Pass models **by value** (Bubble Tea convention — see `.golangci.yml` for disabled `hugeParam` check)
- Keep exported types and functions to a minimum — prefer `internal/` packages
- Use the project's [Warm Ember theme](internal/tui/styles.go) for all new UI colors and styles

## Release Strategy

Releases follow [Semantic Versioning](https://semver.org/) aligned to the project phases:

| Version | Phase                    |
|---------|--------------------------|
| v0.1.0  | Phase 1 — Foundation     |
| v0.2.0  | Phase 2 — Core Controls  |
| v0.3.0  | Phase 3 — Profiles & Sensors |
| v0.4.0  | Phase 4 — IPC & Polish   |
| v1.0.0  | Stable release           |

Release branches use the `release/v{semver}` naming convention (e.g., `release/v0.2.0`).

## Project Structure

```
freshMango/
├── main.go              # Entry point
├── internal/
│   ├── tui/             # Terminal UI (Bubble Tea models)
│   ├── nbfc/            # nbfc-linux CLI wrapper
│   └── system/          # System utilities
├── Makefile             # Build & quality targets
└── .github/
    ├── workflows/ci.yml # CI pipeline
    └── PULL_REQUEST_TEMPLATE.md
```

## Questions?

If you're unsure about anything, open an issue to discuss before starting work. We're happy to help!
