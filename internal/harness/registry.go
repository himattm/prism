package harness

import (
	"os"
	"sort"
)

var registry = map[string]Harness{}

// Register adds a harness to the global registry.
// Typically called from init() in each harness implementation file.
func Register(h Harness) {
	registry[h.ID()] = h
}

// Get returns the harness with the given ID, or nil if not found.
func Get(id string) Harness {
	return registry[id]
}

// All returns all registered harness IDs in sorted order.
func All() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Detect determines which harness is invoking Prism.
// It checks the PRISM_HARNESS environment variable, falling back to "claude".
func Detect() Harness {
	if id := os.Getenv("PRISM_HARNESS"); id != "" {
		if h := Get(id); h != nil {
			return h
		}
	}
	return Get("claude")
}
