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

## 2024-08-03 - Prevent SSRF with net.Dialer Control Hook
**Vulnerability:** Relying on `net/url.Parse` to validate URL schemes doesn't fully protect against SSRF (e.g. against cloud metadata services) and custom transports can break 'Happy Eyeballs'.
**Learning:** Using a `net.Dialer` with a `Control` hook allows inspecting the resolved IP *before* the socket connects, providing a robust way to block targeted IPs (like `169.254.169.254`) without breaking legitimate local use cases.
**Prevention:** Clone `http.DefaultTransport` and configure its `DialContext` to use a dialer with a Control hook that validates the resolved IP. Block specific cloud metadata endpoints instead of all private IPs.
