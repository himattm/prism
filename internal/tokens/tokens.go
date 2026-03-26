package tokens

// CacheEfficiency returns the cache read hit percentage (0-100).
// Returns (ratio, true) if there are cache reads, or (0, false) if not applicable.
func CacheEfficiency(inputTokens, cacheCreationTokens, cacheReadTokens int) (int, bool) {
	if cacheReadTokens <= 0 {
		return 0, false
	}
	denominator := inputTokens + cacheCreationTokens + cacheReadTokens
	if denominator <= 0 {
		return 0, false
	}
	return cacheReadTokens * 100 / denominator, true
}
