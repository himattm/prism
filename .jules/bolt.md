## 2024-04-25 - Caching StatusLine colors

**Learning:** `colors.ColorMap()` allocates a new map for every plugin execution. While this might seem fast, in a system where many plugins run in parallel per status line render, this repeated allocation adds up and wastes CPU cycles.

**Action:** Cache static data maps at the instance level (e.g. `StatusLine` struct initialization) instead of calling a package-level function that returns a new map every time. This ensures thread-safety without locks since it's read-only map state per instance.
