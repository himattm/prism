package sparkline

import (
	"os"
	"testing"
)

func TestBuffer_Push(t *testing.T) {
	b := &Buffer{}

	b.Push(50)
	if b.Count != 1 {
		t.Errorf("expected count 1, got %d", b.Count)
	}
	if b.Latest() != 50 {
		t.Errorf("expected latest 50, got %d", b.Latest())
	}

	// Fill buffer
	for i := 0; i < BufferSize; i++ {
		b.Push(i * 10)
	}
	if b.Count != BufferSize {
		t.Errorf("expected count %d, got %d", BufferSize, b.Count)
	}
}

func TestBuffer_PushClamps(t *testing.T) {
	b := &Buffer{}
	b.Push(-10)
	if b.Latest() != 0 {
		t.Errorf("expected clamped to 0, got %d", b.Latest())
	}
	b.Push(200)
	if b.Latest() != 100 {
		t.Errorf("expected clamped to 100, got %d", b.Latest())
	}
}

func TestBuffer_Latest(t *testing.T) {
	b := &Buffer{}
	if b.Latest() != 0 {
		t.Errorf("expected 0 for empty buffer, got %d", b.Latest())
	}

	b.Push(42)
	b.Push(77)
	if b.Latest() != 77 {
		t.Errorf("expected 77, got %d", b.Latest())
	}
}

func TestBuffer_Render(t *testing.T) {
	b := &Buffer{}
	if b.Render() != "" {
		t.Errorf("expected empty string for empty buffer, got %q", b.Render())
	}

	b.Push(0)
	r := b.Render()
	if r != "▁" {
		t.Errorf("expected ▁ for 0%%, got %q", r)
	}

	b.Push(100)
	r = b.Render()
	if r != "▁█" {
		t.Errorf("expected ▁█, got %q", r)
	}
}

func TestBuffer_RenderFull(t *testing.T) {
	b := &Buffer{}
	// Push values 0, 14, 28, 42, 57, 71, 85, 100
	for i := 0; i < BufferSize; i++ {
		b.Push(i * 100 / (BufferSize - 1))
	}

	r := b.Render()
	if len([]rune(r)) != BufferSize {
		t.Errorf("expected %d runes, got %d: %q", BufferSize, len([]rune(r)), r)
	}

	// First should be lowest bar, last should be highest
	runes := []rune(r)
	if runes[0] != '▁' {
		t.Errorf("expected first rune ▁, got %c", runes[0])
	}
	if runes[BufferSize-1] != '█' {
		t.Errorf("expected last rune █, got %c", runes[BufferSize-1])
	}
}

func TestBuffer_RenderWraparound(t *testing.T) {
	b := &Buffer{}
	// Push more than BufferSize values
	for i := 0; i < BufferSize+4; i++ {
		b.Push(i * 10)
	}

	r := b.Render()
	runes := []rune(r)
	if len(runes) != BufferSize {
		t.Errorf("expected %d runes after wraparound, got %d", BufferSize, len(runes))
	}

	// Oldest visible value should be 40 (pushed 4th-from-start of overflow)
	// Newest should be 110 → clamped to 100
	if runes[BufferSize-1] != '█' {
		t.Errorf("expected last rune █ (100%%), got %c", runes[BufferSize-1])
	}
}

func TestPctToLevel(t *testing.T) {
	tests := []struct {
		pct   int
		level int
	}{
		{0, 0},
		{1, 0},
		{12, 0},
		{13, 1},
		{25, 2},
		{50, 4},
		{75, 6},
		{99, 7},
		{100, 7},
	}

	for _, tt := range tests {
		got := pctToLevel(tt.pct)
		if got != tt.level {
			t.Errorf("pctToLevel(%d) = %d, want %d", tt.pct, got, tt.level)
		}
	}
}

func TestDiskPersistence(t *testing.T) {
	sessionID := "test-session-sparkline"
	metric := "test-metric"

	// Clean up any leftover file
	os.Remove(cacheFilePath(sessionID, metric))

	// Clear in-memory cache
	mu.Lock()
	delete(buffers, metric+":"+sessionID)
	mu.Unlock()

	// Push some values
	buf := PushAndSave(sessionID, metric, 42)
	if buf.Latest() != 42 {
		t.Errorf("expected 42, got %d", buf.Latest())
	}

	// Clear in-memory cache to force disk read
	mu.Lock()
	delete(buffers, metric+":"+sessionID)
	mu.Unlock()

	// Load from disk
	buf2 := Load(sessionID, metric)
	if buf2.Latest() != 42 {
		t.Errorf("expected 42 from disk, got %d", buf2.Latest())
	}
	if buf2.Count != 1 {
		t.Errorf("expected count 1 from disk, got %d", buf2.Count)
	}

	// Clean up
	os.Remove(cacheFilePath(sessionID, metric))
}
