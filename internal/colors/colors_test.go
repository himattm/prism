package colors

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"plain", "plain"},
		{Wrap(Cyan, "hi"), "hi"},
		{Cyan + "a" + Reset + Separator() + Magenta + "b" + Reset, "a · b"},
		{"keep ░▒█ glyphs", "keep ░▒█ glyphs"},
	}
	for _, tt := range tests {
		if got := StripANSI(tt.in); got != tt.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
