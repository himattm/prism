## 2025-02-27 - Predictable Temp File Names Lead to Symlink Attacks
**Vulnerability:** Found uses of predictable temp file names (e.g. `path + ".tmp"`) written with `os.WriteFile` in shared directories (`os.TempDir()`).
**Learning:** `os.WriteFile` follows symlinks. In shared environments, an attacker can pre-create a symlink at the predictable path, causing the application to unknowingly overwrite an arbitrary file the user running the application has access to.
**Prevention:** Use `os.CreateTemp` to generate cryptographically random, unpredictable filenames. Additionally, ensure `f.Close()` is called prior to `os.Rename()` to prevent "file in use" errors on Windows.
