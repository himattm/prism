## 2025-05-09 - Path Traversal Vulnerability in Plugin Manager
**Vulnerability:** Untrusted plugin names from parsed URLs or downloaded script metadata were concatenated directly into file paths using `filepath.Join`, allowing attackers to write arbitrary files (e.g. `../etc/passwd`) outside the plugin directory.
**Learning:** In Go, `filepath.Join` calls `filepath.Clean`, which evaluates `../` sequences internally, making naive concatenation insecure against path traversal.
**Prevention:** When constructing file paths using untrusted user input, always sanitize the input explicitly using `filepath.Base(filepath.Clean("/" + name))` to prevent directory escapes.
