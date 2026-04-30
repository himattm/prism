## 2024-04-30 - Map Allocation Overhead in Statusline Plugins
**Learning:** In performance-sensitive areas like concurrent plugin execution in the status line, calling functions like `colors.ColorMap()` repeatedly causes expensive map allocations per plugin execution, increasing memory usage and GC pressure.
**Action:** Cache static data maps at the instance level (e.g., in the `StatusLine` struct) during initialization to ensure thread-safety, minimize allocations, and improve plugin dispatch performance.
