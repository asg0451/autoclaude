# autoclaude

A CLI tool that orchestrates Claude Code in an autonomous coder-critic loop with TODO-based task tracking.

## Overview

autoclaude runs Claude Code autonomously through a structured workflow:

1. **Planner**: Collaboratively designs the implementation with you, creating a plan and TODO list
2. **Coder**: Works on one TODO at a time, commits when done
3. **Critic**: Reviews changes, runs tests and linters, approves or requests fixes
4. **Fixer**: Addresses critic feedback (up to 3 attempts per TODO)
5. **Evaluator**: Final check that the goal is fully achieved

Each phase runs in a fresh Claude session. The loop continues until all TODOs are complete.

## Installation

```bash
go install go.coldcutz.net/autoclaude@latest
```

Or build from source:
```bash
git clone https://github.com/coldcutz/autoclaude
cd autoclaude
go build -o autoclaude .
```

Requires:
- Go 1.21+
- [Claude Code CLI](https://github.com/anthropics/claude-code) installed and authenticated

## Usage

### Initialize a project

```bash
# Interactive mode - prompts for goal, test command, etc.
autoclaude init -i

# Or provide goal directly
autoclaude init "implement user authentication" -t "go test ./..."
```

This will:
- Create `.autoclaude/` directory with tracking files
- Detect project language(s) and generate coding guidelines
- Run the planner to collaboratively design the implementation
- Create initial TODOs with completion criteria
- Commit the plan

### Run the loop

```bash
autoclaude run
```

The coder-critic loop will run until all TODOs are complete. You can interact with Claude for permission prompts.

### Resume after interruption

```bash
autoclaude resume
```

Cleans up any uncommitted changes and continues from where it left off.

### Check status

```bash
autoclaude status
```

Shows current step, progress, and recent activity.

## Directory Structure

```
.autoclaude/
├── state.json           # Loop state (step, iteration, stats)
├── TODO.md              # Task list with completion criteria
├── plan.md              # Architecture and design decisions
├── NOTES.md             # Tech debt and observations from critic
├── STATUS.md            # Current progress summary
├── coding-guidelines.md # Language-specific coding standards
├── critic_verdict.md    # Latest critic decision
└── current_todo.txt     # TODO currently being worked on
```

## How It Works

### The Loop

```
┌─────────────────────────────────────────────────────┐
│                    FOR EACH TODO                     │
├─────────────────────────────────────────────────────┤
│  ┌─────────┐                                        │
│  │  CODER  │ ─── works on TODO, commits ───────┐   │
│  └─────────┘                                   │   │
│       │                                        │   │
│       ▼                                        │   │
│  ┌─────────┐    APPROVED/MINOR ──► next TODO   │   │
│  │ CRITIC  │ ─────────────────────────────────►│   │
│  └─────────┘    NEEDS_FIXES                    │   │
│       │              │                         │   │
│       │              ▼                         │   │
│       │        ┌─────────┐                     │   │
│       │        │  FIXER  │ (up to 3 attempts)  │   │
│       │        └─────────┘                     │   │
│       │              │                         │   │
│       └──────────────┘                         │   │
│                                                │   │
└────────────────────────────────────────────────┘   │
                       │                             │
                       ▼                             │
                 ┌───────────┐                       │
                 │ EVALUATOR │ ── adds TODOs? ──────┘
                 └───────────┘
                       │
                       ▼
                    COMPLETE
```

### Critic Verdicts

- **APPROVED**: Code is correct, tests pass
- **MINOR_ISSUES**: Non-blocking issues added as new TODOs
- **NEEDS_FIXES**: Blocking issues, fixer will address them

### Language Support

autoclaude detects your project language and generates appropriate coding guidelines:

- **Go**: gofmt, golangci-lint, panic-safe mutex handling, error wrapping
- **Rust**: Error handling with Result, unsafe guidelines
- **Python**: Type hints, context managers, exception handling
- **Node/TypeScript**: Async/await patterns, promise handling

## Configuration

### Permissions

autoclaude merges baseline permissions with your existing `.claude/settings.local.json`. The baseline includes common safe commands like `git`, `go test`, `make`, etc.

### Hooks

autoclaude uses Claude Code's stop hooks to orchestrate the loop. These are automatically configured during `init` and `run`.

## Commands

| Command | Description |
|---------|-------------|
| `autoclaude init` | Initialize project with planner |
| `autoclaude run` | Start the coder-critic loop |
| `autoclaude resume` | Resume after interruption |
| `autoclaude status` | Show current progress |

### Init Flags

| Flag | Description |
|------|-------------|
| `-i, --interactive` | Interactive mode |
| `-t, --test-cmd` | Test command (e.g., `go test ./...`) |
| `-c, --constraints` | Additional rules/constraints |
| `--skip-planner` | Skip initial planning phase |

## Stats

At the end of a run, autoclaude displays statistics:

```
─── Run Statistics ───
  Claude invocations:  12
  TODOs attempted:     5
  TODOs completed:     5

  Critic verdicts:
    Approved:          3
    Minor issues:      1
    Needs fixes:       2

  Fix attempts:        2
  Fix successes:       2

  First-pass accept rate: 80.0%
  Fix success rate:       100.0%
──────────────────────
```

## License

MIT
