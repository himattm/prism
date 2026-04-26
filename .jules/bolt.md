## 2024-05-18 - [Cache ColorMap]
**Learning:** Performance-sensitive areas (like color mapping for the status line) should avoid repeated map allocations per plugin execution.
**Action:** Cache static data maps at the instance level (e.g., in the `StatusLine` struct) during initialization to ensure thread-safety and minimize allocations.
