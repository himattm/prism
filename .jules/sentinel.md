## 2025-04-25 - Prevent Symlink Attacks in File Writes
**Vulnerability:** The usage cache was written directly to `/tmp/prism-usage-cache` using `os.WriteFile`, making it vulnerable to symlink attacks where an attacker could pre-create a symlink and cause Prism to overwrite arbitrary files.
**Learning:** Writing to predictable locations in shared temporary directories without O_EXCL is dangerous. `os.WriteFile` cannot prevent following symlinks.
**Prevention:** Always use an atomic write pattern: create a temporary file with a random suffix using `os.CreateTemp` in the target directory, write to it, close it, and then atomically `os.Rename` it to the final target path.
