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

## 2025-05-15 - Prevent IP-Level SSRF via DNS Rebinding and IP Formats
**Vulnerability:** While untrusted URLs had scheme validation (`http`/`https`), the plugin manager still used the default `http.Client` to resolve and connect to the provided host. This left it vulnerable to advanced SSRF attacks via DNS rebinding, internal IP formats (e.g., `0x7f000001`), or explicit requests to internal network segments (loopback, private subnets, link-local metadata endpoints).
**Learning:** Basic URL scheme validation is insufficient for SSRF protection because attackers can supply valid URLs that resolve to internal IP addresses during connection. The Go standard library does not inherently block outbound connections to internal networks.
**Prevention:** Use a custom `http.Client` with a `net.Dialer` configured with a `Control` hook. This hook intercepts the exact IP address resolved by Go *before* the socket connects, enabling explicit rejection of loopback, private, unspecified, and link-local unicast IPs (like AWS IMDS), effectively mitigating IP-level SSRF and DNS rebinding attacks.
