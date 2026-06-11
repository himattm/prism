## 2024-05-12 - Prevent Path Traversal in Plugin Manager
**Vulnerability:** Untrusted plugin metadata (like `# @name` in scripts) and URLs could be manipulated to contain path traversal sequences (e.g., `../`), allowing arbitrary files to be overwritten or removed.
**Learning:** Go's `filepath.Join` automatically calls `filepath.Clean` which evaluates `../` sequences, making it vulnerable when user-provided or untrusted inputs are directly appended to a base directory path.
**Prevention:** Always sanitize untrusted input used in file paths by applying `filepath.Base(filepath.Clean("/" + name))` to enforce that only the base filename is used, effectively dropping directory escape attempts.

## 2024-05-18 - Prevent Symlink Attacks in Shared Temporary Directories
**Vulnerability:** Predictable file names created in shared temporary directories (`os.TempDir()`) using `os.WriteFile` or `os.Create` are vulnerable to symlink attacks. A local attacker can create a symlink with the predictable name pointing to an arbitrary file, causing the application to overwrite it.
**Learning:** Atomic writes using temporary files and renames protect against symlink attacks because `os.Rename` replaces the file (including symlinks) instead of following it.
**Prevention:** Always use `os.CreateTemp` to create an unpredictable temporary file in the target directory, write to it, close it, and use `os.Rename` to atomically move it to the intended predictable path.

## 2024-05-19 - Prevent Server-Side Request Forgery (SSRF)
**Vulnerability:** Untrusted user-provided URLs in plugin installation were passed directly to `http.Get()`, allowing arbitrary scheme requests (e.g., `file://`) and potential SSRF vulnerabilities.
**Learning:** Passing untrusted URLs directly to HTTP clients without validation is a security risk.
**Prevention:** Always parse untrusted user-provided URLs using `net/url.Parse` and explicitly validate the scheme against an allowlist (e.g., checking for exactly `http` or `https`) before passing them to `http.Get()` or other `net/http` client methods.
