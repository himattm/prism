package piinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAndUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir() uses USERPROFILE on Windows

	dir, err := Install()
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	want := filepath.Join(home, ".pi", "agent", "extensions", "prism")
	if dir != want {
		t.Errorf("Install() dir = %q, want %q", dir, want)
	}

	for _, name := range []string{"index.ts", "package.json", "prism-bin.txt", "version.txt"} {
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

func TestRefreshIfStale(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Not installed → no-op, no error.
	if refreshed, err := RefreshIfStale(); err != nil || refreshed {
		t.Fatalf("RefreshIfStale() before install = (%v, %v), want (false, nil)", refreshed, err)
	}

	dir, err := Install()
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Freshly installed → already current, no refresh.
	if refreshed, err := RefreshIfStale(); err != nil || refreshed {
		t.Fatalf("RefreshIfStale() after install = (%v, %v), want (false, nil)", refreshed, err)
	}

	// Simulate a shim left behind by an older binary: stale version + mangled file
	// + a preserved binary path.
	binBefore, err := os.ReadFile(filepath.Join(dir, "prism-bin.txt"))
	if err != nil {
		t.Fatalf("reading prism-bin.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("0.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.ts"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshIfStale()
	if err != nil || !refreshed {
		t.Fatalf("RefreshIfStale() when stale = (%v, %v), want (true, nil)", refreshed, err)
	}

	// index.ts was rewritten from the embedded copy...
	got, err := os.ReadFile(filepath.Join(dir, "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "stale" {
		t.Error("index.ts was not refreshed")
	}
	// ...and the recorded binary path was preserved.
	binAfter, err := os.ReadFile(filepath.Join(dir, "prism-bin.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(binAfter) != string(binBefore) {
		t.Errorf("prism-bin.txt changed during refresh: %q -> %q", binBefore, binAfter)
	}
}

func TestDetected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir() uses USERPROFILE on Windows

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
