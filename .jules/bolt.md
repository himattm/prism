## 2024-04-24 - Cache Static Data Structures at Instance Level in Render Loops
**Learning:** In parallel plugin execution models (like Prism's statusline), allocating static data structures repeatedly (e.g., calling `colors.ColorMap()`) for each plugin execution causes unnecessary heap allocations and garbage collection overhead.
**Action:** Always cache static data maps at the instance level (e.g., in the `StatusLine` struct) during initialization to ensure thread-safety and minimize memory allocations across concurrent goroutines.
