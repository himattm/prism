## 2024-05-07 - Insecure Temporary File Creation
**Vulnerability:** Predictable temporary filenames used before renaming in shared directories (like `/tmp`) expose the application to symlink attacks, allowing local file overwrite.
**Learning:** Using `os.WriteFile` with predictable filenames like `path + ".tmp"` in `os.TempDir()` is unsafe because `os.WriteFile` follows symlinks.
**Prevention:** Always use `os.CreateTemp(filepath.Dir(path), "prefix-*")` to generate an unpredictable temporary file, write to it, `Close()` it, and then `os.Rename()` it to the final destination.
