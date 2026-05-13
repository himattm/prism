## 2024-05-13 - Instance-level Config Caching
**Learning:** In hot paths with concurrent execution (like goroutines rendering plugins), repeated disk I/O and JSON parsing for configuration files causes significant overhead.
**Action:** Cache dynamically loaded files at the instance level using `sync.RWMutex` for thread safety, and use `sync.Once` in accessor methods to safely initialize maps and prevent nil panics.
