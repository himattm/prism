## 2024-05-04 - Cache StatusLine ColorMap
**Learning:** In performance-sensitive areas with concurrent plugin executions, repeatedly calling `colors.ColorMap()` causes unnecessary map allocations per execution. Caching static data maps at the instance level during initialization ensures thread-safety and minimizes allocations.
**Action:** Use `sync.Once` to lazily cache reference types on the `StatusLine` struct, ensuring thread safety and backward compatibility for tests without relying on constructors.
