// Package piinstall installs prism's Pi coding agent integration.
//
// Unlike Claude Code, Pi cannot invoke an external command for its status line —
// status can only be rendered from a TypeScript extension loaded into Pi's own
// runtime. prism carries that extension embedded in the binary and writes it into
// Pi's extension directory on demand, so users get Pi support without touching a
// separate npm package. The materialized extension shells back to this same prism
// binary using the JSON contract prism already speaks (see internal/statusline).
package piinstall

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed extension
var extensionFS embed.FS

const embeddedRoot = "extension"

// ExtensionDir returns the directory prism installs its Pi extension into:
// ~/.pi/agent/extensions/prism.
func ExtensionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".pi", "agent", "extensions", "prism"), nil
}

// Detected reports whether Pi appears to be installed for the current user
// (i.e. ~/.pi exists). Used by the installer to decide whether to offer Pi setup.
func Detected() bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(home, ".pi"))
	return err == nil && info.IsDir()
}

// Install writes the embedded Pi extension into the user's Pi extension directory
// and records the absolute path of the running prism binary so the extension knows
// which binary to invoke. Returns the directory the extension was written to.
func Install() (string, error) {
	dst, err := ExtensionDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", fmt.Errorf("creating extension directory: %w", err)
	}

	if err := writeEmbeddedFiles(dst); err != nil {
		return "", err
	}

	if err := writeBinPath(dst); err != nil {
		return "", err
	}

	return dst, nil
}

// Uninstall removes the installed Pi extension directory. It is not an error if
// the extension was never installed.
func Uninstall() (string, error) {
	dst, err := ExtensionDir()
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dst); err != nil {
		return "", fmt.Errorf("removing extension directory: %w", err)
	}
	return dst, nil
}

func writeEmbeddedFiles(dst string) error {
	return fs.WalkDir(extensionFS, embeddedRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(embeddedRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := extensionFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}
		return nil
	})
}

// writeBinPath records the absolute path of the running prism binary next to the
// extension so the shim can invoke the exact binary the user installed.
func writeBinPath(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		// Not fatal: the extension falls back to `prism` on PATH.
		return nil
	}
	if abs, err := filepath.Abs(exe); err == nil {
		exe = abs
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return os.WriteFile(filepath.Join(dst, "prism-bin.txt"), []byte(exe+"\n"), 0o644)
}
