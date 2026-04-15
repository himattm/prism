## 2024-05-14 - Go `fmt.Sscanf` vs `strconv.Atoi`
**Learning:** `fmt.Sscanf` is significantly slower (up to 10x) than `strings.Split`/`strings.Fields` combined with `strconv.Atoi` in Go. In a hot path like parsing `git diff --numstat` output for every render loop, `fmt.Sscanf` becomes a CPU bottleneck due to its heavy use of reflection and format string parsing.
**Action:** Avoid `fmt.Sscanf` in loops or hot paths. Use `strings.Fields` or `strings.Split` and parse integers directly with `strconv.Atoi` for much better performance.
