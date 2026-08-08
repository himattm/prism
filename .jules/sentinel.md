## 2024-05-12 - Prevent Path Traversal in Plugin Manager
**Vulnerability:** Untrusted plugin metadata (like `# @name` in scripts) and URLs could be manipulated to contain path traversal sequences (e.g., `../`), allowing arbitrary files to be overwritten or removed.
**Learning:** Go's `filepath.Join` automatically calls `filepath.Clean` which evaluates `../` sequences, making it vulnerable when user-provided or untrusted inputs are directly appended to a base directory path.
**Prevention:** Always sanitize untrusted input used in file paths by applying `filepath.Base(filepath.Clean("/" + name))` to enforce that only the base filename is used, effectively dropping directory escape attempts.

## 2024-05-18 - Prevent Symlink Attacks in Shared Temporary Directories
**Vulnerability:** Predictable file names created in shared temporary directories (`os.TempDir()`) using `os.WriteFile` or `os.Create` are vulnerable to symlink attacks. A local attacker can create a symlink with the predictable name pointing to an arbitrary file, causing the application to overwrite it.
**Learning:** Atomic writes using temporary files and renames protect against symlink attacks because `os.Rename` replaces the file (including symlinks) instead of following it.
**Prevention:** Always use `os.CreateTemp` to create an unpredictable temporary file in the target directory, write to it, close it, and use `os.Rename` to atomically move it to the intended predictable path.

## 2024-05-18 - Prevent SSRF via Untrusted URLs
**Vulnerability:** The `addFromDirectURL` function passed untrusted user-provided URLs directly to `http.Get()`. This allows Server-Side Request Forgery (SSRF), where an attacker could coerce the application to make requests to unexpected or private schemes and endpoints.
**Learning:** Go's `net/http` client will execute requests for any valid scheme if not explicitly restricted. User-supplied URLs must always be validated before being used in outbound requests.
**Prevention:** Always parse untrusted URLs using `net/url.Parse` and enforce an explicit allowlist of acceptable schemes (e.g., only `http` and `https`) before making HTTP requests.
## 2024-05-24 - Unnecessary Temporary Files for In-Memory Data
**Vulnerability:** Writing fully buffered in-memory data to temporary disk files solely for parsing.
**Learning:** This increases attack surface, risks disk exhaustion, and violates the principle of least privilege.
**Prevention:** Refactor parsing functions to accept byte slices or `io.Reader` directly to process data in memory.
## 2024-05-24 - SSRF vulnerability in plugin downloads
**Vulnerability:** The application used `http.Get` directly for downloading plugins from potentially user-controlled URLs without IP validation, exposing it to Server-Side Request Forgery (SSRF) attacks, specifically against cloud metadata endpoints like `169.254.169.254`.
**Learning:** `http.Get` does not protect against SSRF. Merely parsing URLs is insufficient since DNS rebinding can still target internal IPs.
**Prevention:** Use a custom `http.Client` with a `net.Dialer` and a `Control` hook to intercept socket connections *after* DNS resolution but *before* connecting, blocking prohibited IPs (e.g., `169.254.169.254`). Also, ensure proper timeouts are configured.
