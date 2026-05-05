## 2024-05-05 - Cache instance-level static maps to prevent repeated allocations
**Learning:** Repeatedly creating static maps (like ANSI color maps) within concurrent operations (like plugin executions) causes unnecessary allocations and garbage collection overhead.
**Action:** Cache static data maps at the instance level (e.g., in the `StatusLine` struct) using `sync.Once` in a getter method to ensure thread-safety and maintain backward compatibility for tests.
