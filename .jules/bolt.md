## 2024-05-16 - Cache Plugin Configs
**Learning:** Repeatedly parsing JSON and reading files for plugin configurations in concurrent goroutines introduces unnecessary disk I/O and CPU overhead.
**Action:** Cache dynamically loaded configuration files at the instance level using `sync.RWMutex` with double-checked locking, and return shallow copies to maintain thread-safety.
