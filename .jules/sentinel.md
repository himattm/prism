## 2024-05-23 - Prevent Path Traversal in Plugin Paths
**Vulnerability:** Plugin names parsed from untrusted metadata (`# @name`) or extracted from direct URLs were passed unsanitized into `filepath.Join`, allowing path traversal (e.g., writing or deleting arbitrary files outside the plugin directory).
**Learning:** `filepath.Join` cleans paths but evaluates `../` sequences internally. Thus, combining a base directory with untrusted input containing `../` still allows escaping the base directory.
**Prevention:** Sanitize untrusted input using `filepath.Base(filepath.Clean("/" + name))` *before* joining it with base directories to explicitly restrict it to a single filename.
