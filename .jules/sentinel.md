## 2025-02-14 - Predictable Temporary Filename Symlink Vulnerability
**Vulnerability:** Predictable temporary files were created using hardcoded extensions (e.g. `path + ".tmp"`) in shared directories like `/tmp`. This allows an attacker to pre-create a symlink at the predicted location, tricking the application into overwriting an arbitrary file.
**Learning:** Atomic file writes often involve creating a temporary file and renaming it. If the temporary filename is predictable and located in a shared directory, it is vulnerable to symlink attacks.
**Prevention:** Always use `os.CreateTemp` to generate unpredictable temporary filenames. Explicitly set permissions using `Chmod` if matching the original file permissions is necessary, and ensure `Close()` is called before `Rename()`.
