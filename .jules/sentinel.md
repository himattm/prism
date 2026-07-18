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

## 2024-05-19 - Prevent Advanced SSRF to Local/Private IPs
**Vulnerability:** Validating URL schemes (e.g., `http`/`https`) does not prevent Server-Side Request Forgery (SSRF) attacks against internal network resources (like `http://127.0.0.1` or `http://169.254.169.254`).
**Learning:** Go's standard `http.Get` or simple custom clients will blindly follow DNS resolution to local or private IPs.
**Prevention:** Use a custom `http.Client` with a `net.Dialer` and a `Control` hook to validate the exact resolved IP address before the socket connects, rejecting loopback, private, unspecified, and link-local unicast IPs.

## 2024-05-19 - SSRF Mitigations in Local CLI Tools
**Vulnerability:** Attempted to unconditionally block loopback and private IPs to mitigate Server-Side Request Forgery (SSRF) in a local CLI tool using a `net.Dialer` control hook.
**Learning:** For local CLI applications (unlike remote web servers), fetching from local/private URLs or relying on local proxy routing is often legitimate and expected functionality. Unconditional private IP blocking breaks this functional requirement and cannot be used as a safe, drop-in SSRF fix.
**Prevention:** When mitigating SSRF in local CLI tools, network policies (like blocking private IPs) must be implemented as opt-in configurations with proper redirect and proxy behavior handling, rather than unconditional blocks.
