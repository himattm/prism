## 2024-03-24 - Instance-level static map caching
**Learning:** In performance-sensitive areas (like color mapping for the status line where it gets called repeatedly), we should avoid repeated map allocations per plugin execution. However, using global package-level variables risks shared mutable state or requires expensive deep copying.
**Action:** Cache static data maps at the instance level (e.g., in the `StatusLine` struct) during initialization to ensure thread-safety and minimize allocations.
