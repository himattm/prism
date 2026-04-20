## 2024-04-20 - Predictable Temporary Filenames (Symlink Attack Risk)
**Vulnerability:** Found files being saved using predictable temporary names (e.g., `path + ".tmp"`) in shared directories like `os.TempDir()` before being renamed in atomic writes.
**Learning:** Using predictable filenames in world-writable directories allows a malicious local user to pre-create symlinks pointing to sensitive files. When the application writes to the predictable `.tmp` file, it follows the symlink and overwrites the target file with application privileges.
**Prevention:** Always use `os.CreateTemp` to generate an unpredictable temporary file in shared directories. Explicitly `Chmod` the file if necessary to match the desired permissions of `os.WriteFile`, and close it before renaming.
