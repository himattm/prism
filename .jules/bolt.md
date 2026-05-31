## 2024-05-19 - Cache Plugin Configurations to Avoid Disk I/O Bottlenecks
**Learning:** Repeating disk I/O (`os.ReadFile`) and JSON parsing inside hot concurrent paths (like plugin execution loops) creates massive performance bottlenecks and thread contention.
**Action:** Always use thread-safe instance-level caching with `sync.RWMutex` using the double-checked locking pattern for dynamically loaded configurations to drastically improve throughput.

## 2025-05-09 - Prefer strings.HasPrefix for prefix checks
**Learning:** Manual string slicing and length checks (`len(key) >= len(prefix) && key[:len(prefix)] == prefix`) are less readable than `strings.HasPrefix` and easier to get wrong (off-by-one on the bound check). Performance is equivalent.
**Action:** Use `strings.HasPrefix` for prefix-based string checks; the bounds-check and slicing dance is not faster.

## 2025-05-19 - Pre-compile Regexes at Package Level
**Learning:** Calling `regexp.MustCompile` inside frequently executed functions (like `ParseMetadata` inside discovery loops) adds unnecessary CPU overhead and memory allocations from repeated parsing of the same regex string.
**Action:** Always declare regular expressions as package-level global variables using `var myRe = regexp.MustCompile(...)` so they are compiled exactly once at application startup.
