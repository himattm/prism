## 2024-05-12 - Cache Dynamically Loaded JSON Configurations
**Learning:** Repeatedly reading and parsing JSON configurations during concurrent plugin execution causes high disk I/O and parsing overhead, which can be a significant performance bottleneck.
**Action:** Cache dynamically loaded files (e.g., JSON configurations) in memory at the instance level using thread-safe mechanisms like `sync.RWMutex` and `sync.Once` to minimize allocations and overhead.
