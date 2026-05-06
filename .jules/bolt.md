## 2024-05-06 - Cache static maps at instance level
**Learning:** Calling `colors.ColorMap()` allocates a new map every time, which is inefficient when called multiple times per status line render (once for each plugin). Caching it at the instance level reduces execution time from ~4300 ns to ~3 ns per call.
**Action:** Always cache static reference types (like maps) at the struct instance level using `sync.Once` in performance-sensitive paths instead of recreating them on every invocation.
