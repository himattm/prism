## 2024-05-18 - Avoid map literals in frequently called functions
**Learning:** Returning a map literal inside a function causes a new allocation on every function call. In performance-sensitive areas like rendering the statusline, this creates significant garbage collection pressure and increases execution time.
**Action:** When a static map is needed repeatedly, define it as a package-level variable and return a reference to it. This caches the map and avoids repeated allocations.
