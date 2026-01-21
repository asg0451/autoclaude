package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const GuidelinesFile = "coding-guidelines.md"

// GuidelinesPath returns the path to the coding-guidelines.md file
func GuidelinesPath() string {
	return filepath.Join(AutoclaudeDir, GuidelinesFile)
}

// Language represents a detected programming language
type Language string

const (
	LangGo     Language = "go"
	LangRust   Language = "rust"
	LangPython Language = "python"
	LangNode   Language = "node"
)

// DetectLanguages scans the current directory for language indicators
func DetectLanguages() []Language {
	var langs []Language

	// Go: go.mod or go.work
	if fileExists("go.mod") || fileExists("go.work") {
		langs = append(langs, LangGo)
	}

	// Rust: Cargo.toml
	if fileExists("Cargo.toml") {
		langs = append(langs, LangRust)
	}

	// Python: pyproject.toml, setup.py, requirements.txt
	if fileExists("pyproject.toml") || fileExists("setup.py") || fileExists("requirements.txt") {
		langs = append(langs, LangPython)
	}

	// Node: package.json
	if fileExists("package.json") {
		langs = append(langs, LangNode)
	}

	return langs
}

// GenerateGuidelines creates coding guidelines based on detected languages
func GenerateGuidelines(langs []Language) string {
	var sections []string

	sections = append(sections, "# Coding Guidelines\n")
	sections = append(sections, "Follow these guidelines when writing code.\n")

	for _, lang := range langs {
		switch lang {
		case LangGo:
			sections = append(sections, goGuidelines())
		case LangRust:
			sections = append(sections, rustGuidelines())
		case LangPython:
			sections = append(sections, pythonGuidelines())
		case LangNode:
			sections = append(sections, nodeGuidelines())
		}
	}

	if len(langs) == 0 {
		sections = append(sections, "\n## General\n")
		sections = append(sections, "- Write clean, readable code\n")
		sections = append(sections, "- Handle errors appropriately\n")
		sections = append(sections, "- Write tests for new functionality\n")
	}

	return strings.Join(sections, "")
}

// WriteGuidelines detects languages and writes guidelines to file
func WriteGuidelines() error {
	langs := DetectLanguages()
	content := GenerateGuidelines(langs)
	return os.WriteFile(GuidelinesPath(), []byte(content), 0644)
}

// WriteGuidelinesForLanguages writes guidelines for specific languages
func WriteGuidelinesForLanguages(langs []Language) error {
	content := GenerateGuidelines(langs)
	return os.WriteFile(GuidelinesPath(), []byte(content), 0644)
}

// AllLanguages returns all supported languages for prompting
func AllLanguages() []Language {
	return []Language{LangGo, LangRust, LangPython, LangNode}
}

// ParseLanguage converts a string to a Language
func ParseLanguage(s string) (Language, bool) {
	switch strings.ToLower(s) {
	case "go", "golang":
		return LangGo, true
	case "rust":
		return LangRust, true
	case "python", "py":
		return LangPython, true
	case "node", "nodejs", "javascript", "js", "typescript", "ts":
		return LangNode, true
	default:
		return "", false
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func goGuidelines() string {
	return `
## Go

### Formatting & Linting
- ALWAYS run ` + "`gofmt -w .`" + ` before committing
- Critic MUST run ` + "`golangci-lint run`" + ` as part of code review

### Project Layout
- ` + "`cmd/`" + ` - Main applications (one subdirectory per binary)
- ` + "`internal/`" + ` - Private code that cannot be imported by other projects
- ` + "`pkg/`" + ` - Public library code that can be imported (use sparingly)
- Put main.go in the root only for simple single-binary projects
- Group related functionality into packages by domain, not by type
- Avoid ` + "`utils/`" + `, ` + "`helpers/`" + `, ` + "`common/`" + ` packages - be specific
- Keep package names short, lowercase, no underscores

### Panic Safety
- ALWAYS use ` + "`defer`" + ` for mutex unlock: ` + "`mu.Lock(); defer mu.Unlock()`" + `
- ALWAYS use ` + "`defer`" + ` for resource cleanup (files, connections, etc.)
- NEVER call ` + "`mu.Unlock()`" + ` without defer unless there's a specific reason documented in a comment
- If you need to hold a lock for only part of a function, use an IIFE:
  ` + "```go" + `
  result := func() T {
      mu.Lock()
      defer mu.Unlock()
      return protected
  }()
  ` + "```" + `

### Error Handling
- Always check and handle errors - never ignore them with ` + "`_`" + `
- Missing error checks are NOT minor issues, even in test code
- Wrap errors with context: ` + "`fmt.Errorf(\"doing X: %w\", err)`" + `
- Return errors to callers rather than panicking in library code
- **NEVER stifle or merely log errors** - ALWAYS prefer to fail fast. Returning an error is better than logging and continuing. Silently continuing after an error is a bug.

### Logging
- Always use ` + "`log/slog`" + ` for logging, not ` + "`log`" + `
- Never use ` + "`fmt.Print*`" + ` or ` + "`println`" + ` for logging

### Type Safety
- Prefer string-typed enums (` + "`type Status string`" + `) with constants to magic strings
- This gives type safety and makes refactoring easier
- Example: ` + "`type Status string; const StatusActive Status = \"active\"`" + `

### Concurrency
- Prefer channels over shared memory when possible
- Document which goroutine owns mutable state
- Use ` + "`sync.WaitGroup`" + ` to wait for goroutines to complete
- Stick to ` + "`sync.Mutex`" + ` in almost all situations - do not jump to ` + "`sync.RWMutex`" + ` for performance without benchmark data proving it helps
- RWMutex adds overhead and complexity; only use it when you have a read-heavy workload with measurable contention
`
}

func rustGuidelines() string {
	return `
## Rust

### Error Handling
- Use ` + "`Result<T, E>`" + ` for recoverable errors, not panics
- Use ` + "`?`" + ` operator for error propagation
- Provide context with ` + "`.context()`" + ` or ` + "`.with_context()`" + ` (anyhow)

### Safety
- Avoid ` + "`unsafe`" + ` unless absolutely necessary and well-documented
- Prefer owned types over references when ownership is clear
- Use ` + "`Arc<Mutex<T>>`" + ` for shared mutable state across threads
`
}

func pythonGuidelines() string {
	return `
## Python

### Error Handling
- Use specific exception types, not bare ` + "`except:`" + `
- Use context managers (` + "`with`" + `) for resource management
- Don't catch exceptions just to re-raise without additional context

### Type Safety
- Add type hints to function signatures
- Use ` + "`Optional[T]`" + ` for values that can be None
`
}

func nodeGuidelines() string {
	return `
## Node.js / TypeScript

### Async/Await
- Always ` + "`await`" + ` promises or handle with ` + "`.catch()`" + `
- Use ` + "`try/catch`" + ` around await calls for error handling
- Never leave promises unhandled

### Error Handling
- Always handle errors in callbacks
- Use typed errors when possible (TypeScript)
`
}

// goModulePath reads the module path from go.mod, or returns empty string
func goModulePath() string {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// golangciLintConfig returns a balanced .golangci.yml configuration
func golangciLintConfig(modulePath string) string {
	localPrefixes := ""
	if modulePath != "" {
		localPrefixes = fmt.Sprintf("    local-prefixes: %s", modulePath)
	} else {
		localPrefixes = `    # Update local-prefixes to match your module path from go.mod
    # Example: local-prefixes: github.com/yourname/yourproject
    local-prefixes: ""`
	}

	return fmt.Sprintf(`# golangci-lint configuration generated by autoclaude
# Run: golangci-lint run

linters:
  disable-all: true
  enable:
    # Default essential linters
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - typecheck
    - unused

    # Security
    - gosec

    # Error handling
    - errchkjson
    - errorlint
    - errname
    - nilerr
    - wrapcheck

    # Correctness & best practices
    - containedctx
    - contextcheck
    - copyloopvar
    - durationcheck
    - exhaustive
    - fatcontext
    - forcetypeassert
    - gochecksumtype
    - musttag
    - nilnil
    - reassign
    - recvcheck
    - rowserrcheck
    - sqlclosecheck
    - usestdlibvars
    - wastedassign

    # Code quality
    - goconst
    - gocritic
    - misspell
    - nolintlint
    - noctx
    - unconvert
    - unparam

    # Performance
    - bodyclose
    - prealloc

    # Style & formatting
    - goimports
    - revive

    # Testing
    - paralleltest
    - tparallel

linters-settings:
  govet:
    enable-all: true
    disable:
      - shadow # noisy, many false positives

  exhaustive:
    default-signifies-exhaustive: true

  gocritic:
    enabled-tags:
      - diagnostic
      - experimental
      - opinionated
      - performance
      - style

  goimports:
%s

  revive:
    confidence: 0.8
    rules:
      - name: blank-imports
      - name: context-as-argument
      - name: context-keys-type
      - name: dot-imports
      - name: error-return
      - name: error-strings
      - name: error-naming
      - name: exported
      - name: if-return
      - name: increment-decrement
      - name: var-naming
      - name: var-declaration
      - name: package-comments
      - name: range
      - name: receiver-naming
      - name: time-naming
      - name: unexported-return
      - name: indent-error-flow
      - name: errorf
      - name: empty-block
      - name: superfluous-else
      - name: unused-parameter
      - name: unreachable-code
      - name: redefines-builtin-id

  goconst:
    min-len: 3
    min-occurrences: 4

  misspell:
    locale: US

  prealloc:
    simple: true
    range-loops: true
    for-loops: false

issues:
  # Maximum issues count per one linter
  max-issues-per-linter: 0
  # Maximum count of issues with the same text
  max-same-issues: 0

  # Exclude some files
  exclude-rules:
    # Exclude known linter issues from generated files
    - path: _test\.go
      linters:
        - gocritic
        - revive

    # Exclude some linters from test files
    - path: _test\.go
      text: "do not define global errors"
      linters:
        - revive

run:
  timeout: 5m
  go: "1.25"
`, localPrefixes)
}

// WriteGolangciLintConfig writes .golangci.yml to the project root
func WriteGolangciLintConfig() error {
	modulePath := goModulePath()
	content := golangciLintConfig(modulePath)
	return os.WriteFile(".golangci.yml", []byte(content), 0644)
}
