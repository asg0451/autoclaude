# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test Commands

```bash
# Build
go build -o autoclaude .

# Run all tests
go test ./...

# Run a specific test
go test ./internal/config -run TestMergeSettings

# Install locally
go install .
```

## Architecture Overview

autoclaude orchestrates Claude Code in an autonomous coder-critic loop. The system runs through phases (planner → coder → critic → fixer → evaluator), each in a fresh Claude session.

### Package Structure

- **cmd/** - Cobra CLI commands (`init`, `run`, `resume`, `status`)
- **internal/claude/** - Claude CLI wrapper (runs `claude` subprocess, manages PID for hooks)
- **internal/config/** - Claude settings management (`.claude/settings.local.json`), permissions merging, hook setup
- **internal/prompt/** - Prompt templates for each phase (coder, critic, fixer, evaluator, planner)
- **internal/state/** - Loop state persistence (`.autoclaude/state.json`), TODO tracking, language detection
- **internal/tmux/** - Tmux integration for running Claude in background panes

### Key Files

- `internal/config/baseline.json` - Embedded baseline permissions merged with user settings
- `.autoclaude/` directory (created per-project):
  - `state.json` - Loop state (step, iteration, stats)
  - `TODO.md` - Task list with `- [ ]` checkboxes
  - `critic_verdict.md` - Critic writes APPROVED/NEEDS_FIXES/MINOR_ISSUES here
  - `current_todo.txt` - Currently active TODO (for fixer context)

### Loop Flow

The main loop in `cmd/run.go`:
1. For each incomplete TODO: run coder → critic
2. If NEEDS_FIXES: run fixer (up to 3 retries), then critic again
3. After all TODOs: run evaluator (may add more TODOs)

Communication between phases happens via files in `.autoclaude/`. The critic writes its verdict to `critic_verdict.md`, which is parsed by `state.GetCriticVerdict()`.

### Hook System

autoclaude uses Claude Code's stop hooks to regain control after each phase:
- `config.SetupStopHook()` adds a hook that runs `autoclaude _continue` when Claude stops
- `_continue` is a hidden command that signals the loop to proceed
- The planner uses a separate `_planner-done` hook

## Code Patterns

- Uses `github.com/spf13/cobra` for CLI
- Uses `github.com/chzyer/readline` for interactive prompts in init
- Embeds `baseline.json` with `//go:embed`
- Tests use `t.TempDir()` and `os.Chdir()` for isolation
