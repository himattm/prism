## 2024-05-19 - Cache Plugin Configurations to Avoid Disk I/O Bottlenecks
**Learning:** Repeating disk I/O (`os.ReadFile`) and JSON parsing inside hot concurrent paths (like plugin execution loops) creates massive performance bottlenecks and thread contention.
**Action:** Always use thread-safe instance-level caching with `sync.RWMutex` using the double-checked locking pattern for dynamically loaded configurations to drastically improve throughput.

## 2025-05-09 - Prefer strings.HasPrefix for prefix checks
**Learning:** Manual string slicing and length checks (`len(key) >= len(prefix) && key[:len(prefix)] == prefix`) are less readable than `strings.HasPrefix` and easier to get wrong (off-by-one on the bound check). Performance is equivalent.
**Action:** Use `strings.HasPrefix` for prefix-based string checks; the bounds-check and slicing dance is not faster.

## 2024-05-20 - Avoid global locks during I/O in cache initialization
**Learning:** Holding a global `sync.Mutex` or write lock while performing slow I/O (like reading files) to populate a cache serializes access across all concurrent requests, even for different keys, causing severe bottlenecks.
**Action:** When implementing an in-memory cache, use `sync.RWMutex` with a double-checked locking pattern. Perform the expensive I/O outside the lock, and only acquire the write lock to update the cache map.

## 2024-06-09 - Avoid regexp.Compile in hot paths for glob matching
**Learning:** Compiling regular expressions dynamically using `regexp.Compile` inside functions/loops creates unnecessary CPU and memory overhead when doing simple glob matching.
**Action:** For simple wildcard/glob string matching, use `filepath.Match` or `path.Match` instead of converting the glob to a regex and compiling it. It is significantly faster.
## 2024-06-26 - Avoid fmt.Sscanf for simple parsing in hot paths
**Learning:** `fmt.Sscanf` uses reflection and parses format strings at runtime, making it significantly slower (e.g., >10x) than custom parsing or `strconv` for simple integer extraction.
**Action:** Replace `fmt.Sscanf` with `strconv` methods (like `strconv.ParseInt`) or manual byte iteration (`parseLeadingInt`) when extracting numbers from simple known formats or string prefixes in performance-critical code.
