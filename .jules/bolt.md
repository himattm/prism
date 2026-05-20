## 2024-05-20 - Avoid I/O in Write Locks
**Learning:** When using `sync.RWMutex` to cache data loaded via slow, blocking operations (like disk I/O) across multiple distinct keys, holding a single write lock during the I/O operation serializes execution and causes bottlenecks for concurrent access to other distinct keys.
**Action:** Always perform the slow I/O operation outside the write lock before acquiring it to update the cache.
