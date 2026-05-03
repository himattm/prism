## 2026-05-03 - Symlink Vulnerability in Atomic File Writes
**Vulnerability:** Predictable temporary filenames used before `os.Rename` are vulnerable to symlink attacks.
**Learning:** Atomic file writes often use a predictable temporary file (e.g., appending `.tmp` or `.new`), which an attacker can pre-create as a symlink to an arbitrary target.
**Prevention:** Use `os.CreateTemp` to generate unpredictable temporary filenames in the same directory as the target, write data, close the file, and then rename.
