## 2025-05-18 - Atomic file write via symlink vulnerability
**Vulnerability:** Files written with `os.WriteFile` can be susceptible to symlink attacks or write to unintended files when paths are predictable (e.g. `filepath.Join(os.TempDir(), ...)`). Wait... I see several plugins using predictable temp file names without secure atomic writes.
**Learning:** Predictable temp file paths can lead to symlink attacks, and non-atomic file writes can corrupt state or leak data.
**Prevention:** Use `os.CreateTemp` for unpredictable file names, close it, and `os.Rename` for atomic writes.
