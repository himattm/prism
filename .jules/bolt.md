## 2024-06-25 - Cache Static Maps for Performance
**Learning:** In performance-sensitive areas with highly concurrent paths (e.g. statusline rendering with plugins executed in parallel), repeatedly allocating new large maps (like a color lookup map) causes measurable allocation overhead and garbage collection pressure.
**Action:** Cache static, read-only data maps at the instance level (e.g., in the `StatusLine` struct during initialization). Always use fallback initialization logic in getter methods to support test suites that construct structs via literals without constructors.
