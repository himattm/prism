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

## 2024-05-21 - Avoid fmt.Sscanf and fmt.Sprintf for simple numeric parsing
**Learning:** `fmt.Sscanf` and `fmt.Sprintf` are significantly slower than `strconv.Atoi` / `strconv.ParseInt` and string concatenation because of reflection and format string parsing overhead.
**Action:** When parsing strictly formatted strings separated by a known single-byte delimiter (e.g., "%d,%d"), replace `fmt.Sscanf` with `strings.IndexByte` to locate the delimiter followed by `strconv.ParseInt`. Replace `fmt.Sprintf` with string concatenation and `strconv.FormatInt`.

## 2024-07-18 - Replace fmt.Sscanf with custom byte loop for sequential digit parsing
**Learning:** In Go, `fmt.Sscanf` is significantly slower than manual byte traversal due to reflection and format string parsing overhead. When parsing digits sequentially until a non-digit character is encountered (e.g., parsing `123beta` to `123`), replacing `fmt.Sscanf("%d")` with a custom byte loop is over 10x faster.
**Action:** If you need the lenient parsing behavior of `fmt.Sscanf` for performance optimization, implement a custom byte-traversal loop to extract leading digits rather than relying on `strconv.Atoi` or regex, as it maintains exact functional parity and avoids reflection overhead.
