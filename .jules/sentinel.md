## 2025-04-19 - [Predictable Symlink Attack in Temporary Files]
**Vulnerability:** Predictable filenames were used to create temporary files in shared directories (like /tmp via `os.TempDir()`) utilizing `os.WriteFile`.
**Learning:** `os.WriteFile` will follow existing symlinks, which allows a malicious user to pre-create a symlink in a shared directory with a predictable name. When the application writes to it, the malicious user can overwrite arbitrary files as the application user.
**Prevention:** Instead of using predictable names with `os.WriteFile` in shared directories, use `os.CreateTemp` to generate unpredictable filenames, write the data, close the file, and then atomically rename it to the target path (e.g., using a wrapper like `secureWriteFile`).
