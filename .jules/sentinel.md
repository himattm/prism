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

## 2024-05-18 - Prevent SSRF in Default HTTP Client Setup
**Vulnerability:** Even when URL schemes are restricted, relying on `http.DefaultTransport` or a basic `http.Client` allows Server-Side Request Forgery (SSRF) where requests can target sensitive internal IPs like loopback, private blocks, or AWS metadata (`169.254.169.254`).
**Learning:** Checking IPs solely via `net/url.Parse` is insufficient due to DNS rebinding. A secure custom `http.Client` should use a `net.Dialer` with a `Control` hook to validate the explicitly resolved IP address *immediately before* the connection is made.
**Prevention:** Instantiate a global safe `http.Client` configured with a custom `DialContext` that strips IPv6 zones and blocks loopback, private, unspecified, and link-local unicast IPs before executing external requests.
