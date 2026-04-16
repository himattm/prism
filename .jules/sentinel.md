## 2024-05-24 - [Insecure Temporary File Creation]
**Vulnerability:** Predictable temporary file names (e.g., `path + ".tmp"`) were used with `os.WriteFile` in shared directories like `os.TempDir()`. This allows symlink attacks where a malicious user could pre-create the predictable symlink, causing `os.WriteFile` to overwrite arbitrary files the application user has access to.
**Learning:** `os.WriteFile` follows symlinks. When performing atomic file writes via temporary files, predictable names should not be used in shared temporary directories.
**Prevention:** Always use `os.CreateTemp` to generate unpredictable, securely-created temporary files (it uses `O_EXCL` when opening) before writing data and subsequently calling `os.Rename`.
