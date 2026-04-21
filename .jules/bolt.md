## 2026-04-21 - Cache Colors Map Avoid Reallocation
**Learning:** In performance-sensitive areas like a status line that renders frequently, allocating a map for colors on every execution adds up to overhead. Specifically, calling `colors.ColorMap()` inside `runPlugin` caused unnecessary map allocations on each render.
**Action:** Caching static configuration maps at the instance level (like in the `StatusLine` struct) ensures thread-safety without repeating expensive map allocations per execution.
