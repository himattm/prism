## 2024-05-11 - Directory Traversal in Plugin Installation
**Vulnerability:** The script plugin installer allowed arbitrary directory traversal via a malicious `# @name ../` directive.
**Learning:** Plugin metadata provided via external sources (e.g., `# @name` headers) must be explicitly sanitized, because `filepath.Join` handles `../` internally to resolve paths, allowing directory escape.
**Prevention:** Always use `filepath.Base(filepath.Clean("/" + input))` when constructing file paths with untrusted user input that is expected to be a single file name.
