package sparkline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// BufferSize is the number of readings to keep in the ring buffer
	BufferSize = 8
)

// barChars are the Unicode block elements for sparkline visualization
var barChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Buffer is a fixed-size ring buffer of percentage values (0-100)
// that persists to disk across process invocations.
type Buffer struct {
	Values [BufferSize]int `json:"values"`
	Pos    int             `json:"pos"`
	Count  int             `json:"count"`
}

// Push appends a new percentage value to the ring buffer
func (b *Buffer) Push(pct int) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	b.Values[b.Pos] = pct
	b.Pos = (b.Pos + 1) % BufferSize
	if b.Count < BufferSize {
		b.Count++
	}
}

// Latest returns the most recently pushed value, or 0 if empty
func (b *Buffer) Latest() int {
	if b.Count == 0 {
		return 0
	}
	idx := (b.Pos - 1 + BufferSize) % BufferSize
	return b.Values[idx]
}

// Render returns the sparkline string (e.g. "▁▃▅▇█▅▃▂").
// Always renders BufferSize bars; empty slots show as the lowest bar.
func (b *Buffer) Render() string {
	runes := make([]rune, BufferSize)

	// Fill empty leading slots with lowest bar
	empty := BufferSize - b.Count
	for i := 0; i < empty; i++ {
		runes[i] = barChars[0]
	}

	// Read from oldest to newest
	start := 0
	if b.Count == BufferSize {
		start = b.Pos
	}

	for i := 0; i < b.Count; i++ {
		idx := (start + i) % BufferSize
		level := pctToLevel(b.Values[idx])
		runes[empty+i] = barChars[level]
	}

	return string(runes)
}

// pctToLevel maps a percentage (0-100) to a bar level (0-7)
func pctToLevel(pct int) int {
	if pct <= 0 {
		return 0
	}
	if pct >= 100 {
		return 7
	}
	// Map 0-100 to 0-7
	level := pct * 8 / 100
	if level > 7 {
		level = 7
	}
	return level
}

// Disk persistence

var (
	mu      sync.Mutex
	buffers = make(map[string]*Buffer) // in-memory cache of loaded buffers
)

// cacheFilePath returns the path for a given metric's sparkline buffer.
// Path separators are stripped from sessionID to prevent directory traversal.
func cacheFilePath(sessionID, metric string) string {
	safe := strings.ReplaceAll(strings.ReplaceAll(sessionID, "/", ""), "\\", "")
	return filepath.Join(os.TempDir(), fmt.Sprintf("prism-spark-%s-%s.json", metric, safe))
}

// Load reads a buffer from disk (or returns a cached in-memory copy).
// Returns a new empty buffer if the file doesn't exist.
func Load(sessionID, metric string) *Buffer {
	mu.Lock()
	defer mu.Unlock()

	key := metric + ":" + sessionID
	if b, ok := buffers[key]; ok {
		return b
	}

	b := &Buffer{}
	path := cacheFilePath(sessionID, metric)
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, b)
	}

	buffers[key] = b
	return b
}

// Save persists a buffer to disk
func Save(sessionID, metric string, b *Buffer) {
	mu.Lock()
	buffers[metric+":"+sessionID] = b
	mu.Unlock()

	path := cacheFilePath(sessionID, metric)
	data, err := json.Marshal(b)
	if err != nil {
		return
	}
	f, err := os.CreateTemp(filepath.Dir(path), "prism-spark-*.tmp")
	if err != nil {
		return
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return
	}
	f.Chmod(0644)
	f.Close()
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
	}
}

// PushAndSave is a convenience that loads, pushes, saves, and returns the buffer
func PushAndSave(sessionID, metric string, pct int) *Buffer {
	b := Load(sessionID, metric)
	b.Push(pct)
	Save(sessionID, metric, b)
	return b
}
