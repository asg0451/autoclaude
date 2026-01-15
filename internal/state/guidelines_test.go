package state

import (
	"os"
	"strings"
	"testing"
)

func TestDetectLanguages(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// No language files
	langs := DetectLanguages()
	if len(langs) != 0 {
		t.Errorf("expected no languages, got %v", langs)
	}

	// Create go.mod
	os.WriteFile("go.mod", []byte("module test"), 0644)
	langs = DetectLanguages()
	if len(langs) != 1 || langs[0] != LangGo {
		t.Errorf("expected [go], got %v", langs)
	}

	// Add package.json
	os.WriteFile("package.json", []byte("{}"), 0644)
	langs = DetectLanguages()
	if len(langs) != 2 {
		t.Errorf("expected 2 languages, got %v", langs)
	}

	// Check both are present
	hasGo, hasNode := false, false
	for _, l := range langs {
		if l == LangGo {
			hasGo = true
		}
		if l == LangNode {
			hasNode = true
		}
	}
	if !hasGo || !hasNode {
		t.Errorf("expected go and node, got %v", langs)
	}
}

func TestDetectLanguagesRust(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.WriteFile("Cargo.toml", []byte("[package]"), 0644)
	langs := DetectLanguages()
	if len(langs) != 1 || langs[0] != LangRust {
		t.Errorf("expected [rust], got %v", langs)
	}
}

func TestDetectLanguagesPython(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.WriteFile("requirements.txt", []byte("flask"), 0644)
	langs := DetectLanguages()
	if len(langs) != 1 || langs[0] != LangPython {
		t.Errorf("expected [python], got %v", langs)
	}

	os.Remove("requirements.txt")
	os.WriteFile("pyproject.toml", []byte("[project]"), 0644)
	langs = DetectLanguages()
	if len(langs) != 1 || langs[0] != LangPython {
		t.Errorf("expected [python], got %v", langs)
	}
}

func TestGenerateGuidelines(t *testing.T) {
	// Test Go guidelines
	content := GenerateGuidelines([]Language{LangGo})
	if !strings.Contains(content, "## Go") {
		t.Error("Go guidelines should contain '## Go' header")
	}
	if !strings.Contains(content, "gofmt") {
		t.Error("Go guidelines should mention gofmt")
	}
	if !strings.Contains(content, "golangci-lint") {
		t.Error("Go guidelines should mention golangci-lint")
	}
	if !strings.Contains(content, "defer") {
		t.Error("Go guidelines should mention defer for mutex")
	}

	// Test Rust guidelines
	content = GenerateGuidelines([]Language{LangRust})
	if !strings.Contains(content, "## Rust") {
		t.Error("Rust guidelines should contain '## Rust' header")
	}
	if !strings.Contains(content, "Result") {
		t.Error("Rust guidelines should mention Result type")
	}

	// Test Python guidelines
	content = GenerateGuidelines([]Language{LangPython})
	if !strings.Contains(content, "## Python") {
		t.Error("Python guidelines should contain '## Python' header")
	}
	if !strings.Contains(content, "type hints") {
		t.Error("Python guidelines should mention type hints")
	}

	// Test Node guidelines
	content = GenerateGuidelines([]Language{LangNode})
	if !strings.Contains(content, "## Node") {
		t.Error("Node guidelines should contain '## Node' header")
	}
	if !strings.Contains(content, "await") {
		t.Error("Node guidelines should mention await")
	}

	// Test empty languages
	content = GenerateGuidelines([]Language{})
	if !strings.Contains(content, "## General") {
		t.Error("Empty languages should produce general guidelines")
	}

	// Test multiple languages
	content = GenerateGuidelines([]Language{LangGo, LangRust})
	if !strings.Contains(content, "## Go") || !strings.Contains(content, "## Rust") {
		t.Error("Multiple languages should include all sections")
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected Language
		ok       bool
	}{
		{"go", LangGo, true},
		{"golang", LangGo, true},
		{"Go", LangGo, true},
		{"rust", LangRust, true},
		{"Rust", LangRust, true},
		{"python", LangPython, true},
		{"py", LangPython, true},
		{"node", LangNode, true},
		{"nodejs", LangNode, true},
		{"javascript", LangNode, true},
		{"js", LangNode, true},
		{"typescript", LangNode, true},
		{"ts", LangNode, true},
		{"unknown", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		lang, ok := ParseLanguage(tt.input)
		if ok != tt.ok {
			t.Errorf("ParseLanguage(%q): expected ok=%v, got ok=%v", tt.input, tt.ok, ok)
		}
		if lang != tt.expected {
			t.Errorf("ParseLanguage(%q): expected %q, got %q", tt.input, tt.expected, lang)
		}
	}
}

func TestWriteGuidelines(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)
	os.WriteFile("go.mod", []byte("module test"), 0644)

	err := WriteGuidelines()
	if err != nil {
		t.Fatalf("WriteGuidelines failed: %v", err)
	}

	data, err := os.ReadFile(GuidelinesPath())
	if err != nil {
		t.Fatalf("failed to read guidelines: %v", err)
	}

	if !strings.Contains(string(data), "## Go") {
		t.Error("guidelines file should contain Go section")
	}
}

func TestWriteGuidelinesForLanguages(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	os.MkdirAll(AutoclaudeDir, 0755)

	err := WriteGuidelinesForLanguages([]Language{LangRust, LangPython})
	if err != nil {
		t.Fatalf("WriteGuidelinesForLanguages failed: %v", err)
	}

	data, err := os.ReadFile(GuidelinesPath())
	if err != nil {
		t.Fatalf("failed to read guidelines: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "## Rust") {
		t.Error("guidelines file should contain Rust section")
	}
	if !strings.Contains(content, "## Python") {
		t.Error("guidelines file should contain Python section")
	}
}

func TestAllLanguages(t *testing.T) {
	langs := AllLanguages()
	if len(langs) != 4 {
		t.Errorf("expected 4 languages, got %d", len(langs))
	}

	expected := map[Language]bool{
		LangGo:     true,
		LangRust:   true,
		LangPython: true,
		LangNode:   true,
	}

	for _, l := range langs {
		if !expected[l] {
			t.Errorf("unexpected language: %s", l)
		}
	}
}
