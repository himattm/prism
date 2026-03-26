package tokens

import "testing"

func TestCacheEfficiency(t *testing.T) {
	tests := []struct {
		name                  string
		input, creation, read int
		expectedRatio         int
		expectedOK            bool
	}{
		{"normal case", 5000, 2000, 3000, 30, true},
		{"high cache ratio", 1000, 1000, 8000, 80, true},
		{"all cache reads", 0, 0, 10000, 100, true},
		{"no cache reads", 10000, 5000, 0, 0, false},
		{"zero tokens", 0, 0, 0, 0, false},
		{"negative cache reads", 10000, 0, -1, 0, false},
		{"only input tokens", 50000, 0, 0, 0, false},
		{"78 percent cache hits", 1000, 1200, 7800, 78, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, ok := CacheEfficiency(tt.input, tt.creation, tt.read)
			if ok != tt.expectedOK {
				t.Errorf("CacheEfficiency(%d, %d, %d) ok = %v, want %v",
					tt.input, tt.creation, tt.read, ok, tt.expectedOK)
			}
			if ok && ratio != tt.expectedRatio {
				t.Errorf("CacheEfficiency(%d, %d, %d) = %d, want %d",
					tt.input, tt.creation, tt.read, ratio, tt.expectedRatio)
			}
		})
	}
}
