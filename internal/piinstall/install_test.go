package piinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAndUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := Install()
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	want := filepath.Join(home, ".pi", "agent", "extensions", "prism")
	if dir != want {
		t.Errorf("Install() dir = %q, want %q", dir, want)
	}

	for _, name := range []string{"index.ts", "package.json", "prism-bin.txt"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	// prism-bin.txt should contain a non-empty path.
	data, err := os.ReadFile(filepath.Join(dir, "prism-bin.txt"))
	if err != nil {
		t.Fatalf("reading prism-bin.txt: %v", err)
	}
	if len(data) == 0 {
		t.Error("prism-bin.txt is empty")
	}

	if _, err := Uninstall(); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("extension dir still exists after Uninstall: %v", err)
	}

	// Uninstall is idempotent.
	if _, err := Uninstall(); err != nil {
		t.Errorf("second Uninstall() error: %v", err)
	}
}

func TestDetected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if Detected() {
		t.Error("Detected() = true before ~/.pi exists")
	}

	if err := os.MkdirAll(filepath.Join(home, ".pi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !Detected() {
		t.Error("Detected() = false after ~/.pi created")
	}
}
