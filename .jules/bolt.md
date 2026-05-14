## 2024-05-14 - Cache Plugin JSON Configurations
**Learning:** Concurrent execution of multiple plugins reading the same config files on disk causes massive overhead via redundant `os.ReadFile` and `json.Unmarshal` operations. Reference types (like maps) loaded from disk must be defensively shallow-copied when cached to prevent data races across goroutines.
**Action:** Always cache configuration data using thread-safe instance-level caches (`sync.RWMutex`) and return shallow copies to ensure fast, thread-safe access in highly concurrent environments.
