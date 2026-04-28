
## 2024-05-24 - Cache color map in StatusLine
**Learning:** In performance-sensitive areas (like color mapping for the status line), repeated map allocations per plugin execution can cause unnecessary overhead.
**Action:** Instead of calling a function that creates a new map repeatedly, cache the static data map at the instance level (e.g., in the `StatusLine` struct) during initialization.
