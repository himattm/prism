## 2024-05-19 - Cache Plugin Configurations to Avoid Disk I/O Bottlenecks
**Learning:** Repeating disk I/O (`os.ReadFile`) and JSON parsing inside hot concurrent paths (like plugin execution loops) creates massive performance bottlenecks and thread contention.
**Action:** Always use thread-safe instance-level caching with `sync.RWMutex` using the double-checked locking pattern for dynamically loaded configurations to drastically improve throughput.

## 2025-05-09 - Prefer strings.HasPrefix for prefix checks
**Learning:** Manual string slicing and length checks (`len(key) >= len(prefix) && key[:len(prefix)] == prefix`) are less readable than `strings.HasPrefix` and easier to get wrong (off-by-one on the bound check). Performance is equivalent.
**Action:** Use `strings.HasPrefix` for prefix-based string checks; the bounds-check and slicing dance is not faster.

## 2024-06-06 - Regexp Compilation Optimization
**Learning:** Calling `regexp.MustCompile` inside frequently executed functions (like discovery loops or data parsing) causes significant CPU overhead and memory allocations because the regex is recompiled on every invocation.
**Action:** Always declare regular expressions as package-level global variables (e.g., `var myRe = regexp.MustCompile(...)`) to ensure they are compiled exactly once at application startup.
