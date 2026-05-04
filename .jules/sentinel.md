## 2026-05-04 - Fix Predictable Temporary File Names for Atomic Writes
**Vulnerability:** Predictable temporary file names used during atomic file writes (`path + ".tmp"`) are susceptible to symlink attacks, allowing an attacker to overwrite arbitrary files if they have write access to the temporary directory.
**Learning:** Atomic file writes often rely on temporary files, but using predictable names allows malicious actors to pre-create symlinks pointing to sensitive files.
**Prevention:** Always use `os.CreateTemp` to generate unpredictable temporary file names, create them in the same directory as the target file to avoid cross-device rename issues, and securely set permissions before writing.
