## 2024-05-10 - Replace HasPrefix with HasPrefix
**Learning:** Checking for `strings.HasPrefix` is perfectly fine. Let's look for map access or sync.Once usage on hot paths as noted in memory.
**Action:** Optimize `getColorMap()` in `internal/statusline/statusline.go` to use direct map reference after the sync.Once initialization.
## 2024-05-10 - Cache plugin config loads
**Learning:** Loading configuration files involves disk I/O (`os.ReadFile`) and JSON parsing, which is expensive when called repeatedly for each plugin execution.
**Action:** Cache the loaded configurations in memory (using a `sync.RWMutex` protected map) to avoid repeated disk reads and allocations on hot paths.
