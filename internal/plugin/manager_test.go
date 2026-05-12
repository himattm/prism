package plugin

import "testing"

func TestSanitizeFilenameDropsPathTraversal(t *testing.T) {
	tests := map[string]string{
		"safe-plugin":              "safe-plugin",
		"../evil":                  "evil",
		"../../evil":               "evil",
		"nested/evil":              "evil",
		"nested/../evil":           "evil",
		"/absolute/path/evil":      "evil",
		"..\\windows\\style\\evil": "..\\windows\\style\\evil",
	}

	for input, want := range tests {
		if got := sanitizeFilename(input); got != want {
			t.Fatalf("sanitizeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}
