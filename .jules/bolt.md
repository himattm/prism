## 2024-04-16 - Pre-allocated static lookup maps
**Learning:** Found that `ColorMap()` was re-allocating a large map of ANSI color codes (~90 keys) upon every invocation. This was called on every plugin execution per prompt refresh via `colors.ColorMap()`.
**Action:** Migrated static mapping data to package-level variables and return a reference to it to avoid garbage collection overhead and repeated allocations in performance-sensitive plugin execution paths.
