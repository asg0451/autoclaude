package state

import (
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

### Logging
- Always use ` + "`log/slog`" + ` for logging, not ` + "`log`" + `
- Never use ` + "`fmt.Print*`" + ` or ` + "`println`" + ` for logging

### Type Safety
- Prefer string-typed enums (`type Status string`) with constants to magic strings
- This gives type safety and makes refactoring easier
- Example: `type Status string; const StatusActive Status = "active"`

### Concurrency
- Prefer channels over shared memory when possible
- Document which goroutine owns mutable state
- Use ` + "`sync.WaitGroup`" + ` to wait for goroutines to complete
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
