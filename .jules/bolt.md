## 2024-05-07 - Avoid Per-Execution Map Allocations
**Learning:** Repeatedly creating map structures (like `colors.ColorMap()`) during per-plugin execution within a parallel loop causes excessive memory allocation and performance overhead.
**Action:** Cache static map data at the instance level (e.g., `StatusLine` struct) during initialization using `sync.Once` to ensure thread-safety, minimize allocations, and maintain backward compatibility.
