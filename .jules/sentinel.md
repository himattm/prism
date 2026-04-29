## 2025-04-29 - Predictable Temp Files in Atomic Writes
**Vulnerability:** Predictable temporary file paths (e.g., `filepath + ".tmp"`) were used with `os.WriteFile` before `os.Rename` for atomic saves.
**Learning:** This exposes the application to symlink attacks or TOCTOU (Time-of-Check to Time-of-Use) vulnerabilities, as an attacker could preemptively create the `.tmp` file as a symlink to write to unauthorized locations.
**Prevention:** Always use `os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*")` to generate an unpredictable temporary file in the same directory. Write, `Chmod` to set correct permissions, explicitly `Close` the file, and then `os.Rename`.
