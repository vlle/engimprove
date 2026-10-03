package resume

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoader(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "resume.txt"), []byte("Artemii Kulikov\nGo engineer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "hunt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hunt", "stories.md"), []byte("# STAR\n- S: ..."), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader(dir)
	if !loader.Enabled() {
		t.Fatal("expected enabled")
	}
	ctx := loader.Context()
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if !strings.Contains(ctx, "Artemii Kulikov") || !strings.Contains(ctx, "STAR") {
		t.Fatalf("context missing expected content: %q", ctx)
	}

	files := loader.Files()
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
}

func TestLoaderDisabled(t *testing.T) {
	loader := NewLoader("")
	if loader.Enabled() {
		t.Fatal("expected disabled")
	}
	if loader.Context() != "" {
		t.Fatal("expected empty context")
	}
}
