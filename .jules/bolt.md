## 2026-04-23 - Cache static data maps at instance level
**Learning:** In performance-sensitive areas like `runPlugin` that executes repeatedly, calling a function that allocates and returns a large map (like `colors.ColorMap()`) causes repeated memory allocations.
**Action:** Instead of calling a map-generating function on every execution, cache the static data map at the instance level (e.g., in the `StatusLine` struct during initialization) to avoid allocations per plugin execution while maintaining thread-safety.
